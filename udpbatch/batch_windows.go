//go:build windows

package udpbatch

import (
	"errors"
	"io"
	"net"
	"net/netip"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	queueCap   = 256
	maxBatch   = 64
	senderIdle = 100 * time.Millisecond
	poolBufCap = 2048
)

// quicUDP 是 quic-go 在 Windows 上仍会用到的套接字能力：调大缓冲区，以及设置 DF。
type quicUDP interface {
	net.PacketConn
	SyscallConn() (syscall.RawConn, error)
	SetReadBuffer(int) error
	SetWriteBuffer(int) error
}

type windowsWriter struct {
	conn    *net.UDPConn
	uso     bool
	scratch []byte
	ctrl    ctrlBuf
}

// ctrlBuf 按指针宽度对齐，保证 WSACMSGHDR 落在合法地址上。
type ctrlBuf struct {
	_   uintptr
	raw [32]byte
}

// NewWriter 在支持 USO 的套接字上合并同长度报文，否则逐包发送。
func NewWriter(conn *net.UDPConn) Writer {
	return &windowsWriter{conn: conn, uso: usoSupported(conn)}
}

// Wrap 仅在 Windows、底层是 *net.UDPConn 且 USO 可用时包一层异步合并写。
// 其它情况原样返回，避免无谓的拷贝。
func Wrap(pc net.PacketConn) net.PacketConn {
	uc, ok := pc.(*net.UDPConn)
	if !ok || !usoSupported(uc) {
		return pc
	}
	return newCoalesceConn(uc)
}

func usoSupported(conn *net.UDPConn) bool {
	raw, err := conn.SyscallConn()
	if err != nil {
		return false
	}
	var serr error
	if err = raw.Control(func(fd uintptr) {
		_, serr = windows.GetsockoptInt(windows.Handle(fd), windows.IPPROTO_UDP, windows.UDP_SEND_MSG_SIZE)
	}); err != nil {
		return false
	}
	return serr == nil
}

func (w *windowsWriter) WriteBatch(msgs []Message) (int, error) {
	sent := 0
	for sent < len(msgs) {
		count, total := groupRun(msgs[sent:], maxCoalescedPayload)
		var (
			n   int
			err error
		)
		if count < 2 || !w.uso {
			n, err = w.writeOne(msgs[sent])
		} else {
			n, err = w.writeUSO(msgs[sent:sent+count], total)
		}
		sent += n
		if err != nil {
			return sent, err
		}
		if n == 0 {
			return sent, io.ErrShortWrite
		}
	}
	return sent, nil
}

func (w *windowsWriter) writeOne(m Message) (int, error) {
	if err := writeMsg(w.conn, m); err != nil {
		return 0, err
	}
	return 1, nil
}

func (w *windowsWriter) writeUSO(msgs []Message, total int) (int, error) {
	if cap(w.scratch) < total {
		w.scratch = make([]byte, total)
	}
	buf := w.scratch[:total]
	off := 0
	seg := len(msgs[0].Buf)

	// USO 的每个分段必须等长, 长度不一致时按单一 seg 划分会把报文边界切错
	if seg <= 0 {
		w.uso = false
		return w.writeEach(msgs)
	}
	for _, m := range msgs {
		if len(m.Buf) != seg {
			w.uso = false
			return w.writeEach(msgs)
		}
		off += copy(buf[off:], m.Buf)
	}

	n, _, err := w.conn.WriteMsgUDPAddrPort(buf, w.control(uint32(seg)), msgs[0].Addr)
	if err != nil || n == 0 {
		// 这次系统调用没有把数据交出去，改逐包重发，并停用 USO。
		w.uso = false
		return w.writeEach(msgs)
	}
	if n != total {
		// 只发出了部分字节: 必须按段长折算真正完整发出的报文数。
		// 原来固定返回 len(msgs), 上层会把没发出去的报文也算成已发送, 造成静默丢包。
		w.uso = false
		complete := n / seg
		if complete <= 0 {
			return 0, io.ErrShortWrite
		}
		return complete, nil
	}
	return len(msgs), nil
}

func (w *windowsWriter) writeEach(msgs []Message) (int, error) {
	for i, m := range msgs {
		if err := writeMsg(w.conn, m); err != nil {
			return i, err
		}
	}
	return len(msgs), nil
}

// control 构造一条 UDP_SEND_MSG_SIZE 附属数据。
// 布局对应 ws2def.h 的 WSA_CMSG_SPACE / WSA_CMSG_LEN / WSA_CMSG_DATA：
// 数据区按 MAX_NATURAL_ALIGNMENT（指针宽度）对齐，头按 WSACMSGHDR 对齐。
func (w *windowsWriter) control(seg uint32) []byte {
	space := cmsgSpace(4)
	for i := range w.ctrl.raw[:space] {
		w.ctrl.raw[i] = 0
	}
	base := unsafe.Pointer(&w.ctrl.raw[0])
	hdr := (*windows.WSACMSGHDR)(base)
	hdr.Len = uintptr(cmsgLen(4))
	hdr.Level = windows.IPPROTO_UDP
	hdr.Type = windows.UDP_SEND_MSG_SIZE
	*(*uint32)(unsafe.Add(base, cmsgDataOff())) = seg
	return w.ctrl.raw[:space]
}

func alignUp(n, a int) int {
	return (n + a - 1) & ^(a - 1)
}

func cmsgHdrAlign(n int) int {
	return alignUp(n, int(unsafe.Alignof(windows.WSACMSGHDR{})))
}

func cmsgDataAlign(n int) int {
	return alignUp(n, int(unsafe.Sizeof(uintptr(0))))
}

func cmsgDataOff() int {
	return cmsgDataAlign(int(unsafe.Sizeof(windows.WSACMSGHDR{})))
}

func cmsgLen(length int) int {
	return cmsgDataOff() + length
}

func cmsgSpace(length int) int {
	return cmsgDataAlign(int(unsafe.Sizeof(windows.WSACMSGHDR{})) + cmsgHdrAlign(length))
}

var bufPool = sync.Pool{
	New: func() any {
		return make([]byte, poolBufCap)
	},
}

func getBuf(n int) []byte {
	if n > poolBufCap {
		return make([]byte, n)
	}
	b := bufPool.Get().([]byte)
	if cap(b) < n {
		return make([]byte, n)
	}
	return b[:n]
}

func putBuf(b []byte) {
	if cap(b) == poolBufCap {
		bufPool.Put(b[:poolBufCap])
	}
}

type coalesceConn struct {
	pc *net.UDPConn
	w  Writer

	mu     sync.Mutex
	cond   *sync.Cond
	kick   chan struct{}
	queue  []Message
	closed bool
	fatal  error
	sender bool
}

func newCoalesceConn(pc *net.UDPConn) *coalesceConn {
	c := &coalesceConn{
		pc:    pc,
		w:     NewWriter(pc),
		kick:  make(chan struct{}, 1),
		queue: make([]Message, 0, queueCap),
	}
	c.cond = sync.NewCond(&c.mu)
	return c
}

var (
	_ net.PacketConn = (*coalesceConn)(nil)
	_ quicUDP        = (*coalesceConn)(nil)
)

func (c *coalesceConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	ap, ok := addrPortFrom(addr)
	if !ok {
		return c.pc.WriteTo(b, addr)
	}
	buf := getBuf(len(b))
	copy(buf, b)

	c.mu.Lock()
	for len(c.queue) >= queueCap && !c.closed && c.fatal == nil {
		c.cond.Wait()
	}
	if c.fatal != nil {
		err := c.fatal
		c.mu.Unlock()
		putBuf(buf)
		return 0, err
	}
	if c.closed {
		c.mu.Unlock()
		putBuf(buf)
		return 0, net.ErrClosed
	}
	c.queue = append(c.queue, Message{Buf: buf, Addr: ap})
	if !c.sender {
		c.sender = true
		go c.sendLoop()
	}
	c.wake()
	c.mu.Unlock()
	return len(b), nil
}

func (c *coalesceConn) Close() error {
	c.mu.Lock()
	if !c.closed {
		c.closed = true
		c.wake()
		c.cond.Broadcast()
	}
	for c.sender {
		c.cond.Wait()
	}
	c.mu.Unlock()
	return c.pc.Close()
}

func (c *coalesceConn) wake() {
	select {
	case c.kick <- struct{}{}:
	default:
	}
}

func (c *coalesceConn) sendLoop() {
	timer := time.NewTimer(senderIdle)
	defer timer.Stop()
	var batch []Message
	for {
		select {
		case <-timer.C:
			if c.tryIdleExit() {
				return
			}
		case <-c.kick:
		}
		var (
			fatal error
			exit  bool
		)
		batch, fatal, exit = c.pop(batch)
		if exit {
			return
		}
		if len(batch) == 0 {
			timer.Reset(senderIdle)
			continue
		}
		if fatal != nil {
			releaseBatch(batch)
		} else {
			c.writeAll(batch)
		}
		timer.Reset(senderIdle)
	}
}

func (c *coalesceConn) tryIdleExit() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || len(c.queue) > 0 {
		return false
	}
	c.sender = false
	c.cond.Broadcast()
	return true
}

func (c *coalesceConn) pop(dst []Message) (batch []Message, fatal error, exit bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queue) == 0 {
		if c.closed {
			c.sender = false
			c.cond.Broadcast()
			return nil, nil, true
		}
		return dst[:0], nil, false
	}
	n := min(len(c.queue), maxBatch)
	batch = append(dst[:0], c.queue[:n]...)
	copy(c.queue, c.queue[n:])
	newLen := len(c.queue) - n
	for i := newLen; i < len(c.queue); i++ {
		c.queue[i] = Message{}
	}
	c.queue = c.queue[:newLen]
	c.cond.Broadcast()
	return batch, c.fatal, false
}

func (c *coalesceConn) writeAll(batch []Message) {
	_, err := c.w.WriteBatch(batch)
	releaseBatch(batch)
	if err == nil {
		return
	}
	if isFatalWriteErr(err) {
		c.mu.Lock()
		if c.fatal == nil {
			c.fatal = err
		}
		c.mu.Unlock()
		return
	}
	// 瞬时错误丢掉这一批（调用方的 WriteTo 已经返回成功）。稍作退让，避免打满 CPU。
	time.Sleep(time.Millisecond)
}

func releaseBatch(batch []Message) {
	for i := range batch {
		putBuf(batch[i].Buf)
		batch[i] = Message{}
	}
}

// isFatalWriteErr 判断写错误是否不可恢复。
// 只认 net.ErrClosed 时, EPERM / ENETDOWN / EAFNOSUPPORT 之类的永久错误
// 会被当成瞬时错误反复重试: 数据既发不出去也不报错, 发送循环还会以毫秒级间隔空转。
func isFatalWriteErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}

	var errno syscall.Errno
	if errors.As(err, &errno) {
		switch errno {
		case windows.WSAEACCES,
			windows.WSAENETDOWN,
			windows.WSAENETUNREACH,
			windows.WSAEAFNOSUPPORT,
			windows.WSAEINVAL,
			windows.WSAEMSGSIZE:
			return true
		}
	}
	return false
}

func addrPortFrom(addr net.Addr) (netip.AddrPort, bool) {
	ua, ok := addr.(*net.UDPAddr)
	if !ok || ua == nil {
		return netip.AddrPort{}, false
	}
	ip, ok := netip.AddrFromSlice(ua.IP)
	if !ok {
		return netip.AddrPort{}, false
	}
	if ua.Zone != "" {
		ip = ip.WithZone(ua.Zone)
	}
	return netip.AddrPortFrom(ip, uint16(ua.Port)), true
}

func (c *coalesceConn) ReadFrom(p []byte) (int, net.Addr, error) {
	return c.pc.ReadFrom(p)
}

func (c *coalesceConn) LocalAddr() net.Addr { return c.pc.LocalAddr() }

func (c *coalesceConn) SetDeadline(t time.Time) error      { return c.pc.SetDeadline(t) }
func (c *coalesceConn) SetReadDeadline(t time.Time) error  { return c.pc.SetReadDeadline(t) }
func (c *coalesceConn) SetWriteDeadline(t time.Time) error { return c.pc.SetWriteDeadline(t) }
func (c *coalesceConn) SetReadBuffer(n int) error          { return c.pc.SetReadBuffer(n) }
func (c *coalesceConn) SetWriteBuffer(n int) error         { return c.pc.SetWriteBuffer(n) }

func (c *coalesceConn) SyscallConn() (syscall.RawConn, error) { return c.pc.SyscallConn() }
