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
	// writeAttempts 是非致命写失败时，对还没发出去的报文再试的次数。
	writeAttempts = 3
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
	// 除最后一条外必须等长。最后一条可以更短，Windows 会把它当成不足一段的尾包。
	// 形状不对只影响这一批，不能关掉 USO，否则后面的等长报文也会退回逐包。
	if !usoGroupOK(msgs) {
		return w.writeEach(msgs)
	}
	seg := len(msgs[0].Buf)
	if cap(w.scratch) < total {
		w.scratch = make([]byte, total)
	}
	buf := w.scratch[:total]
	off := 0
	for _, m := range msgs {
		off += copy(buf[off:], m.Buf)
	}

	n, _, err := w.conn.WriteMsgUDPAddrPort(buf, w.control(uint32(seg)), msgs[0].Addr)
	if err != nil && usoUnsupported(err) {
		w.uso = false
	}
	if n <= 0 {
		// 这次系统调用没有把数据交出去，整批改逐包。短暂失败不关闭 USO。
		return w.writeEach(msgs)
	}
	if n >= total {
		return len(msgs), nil
	}
	// 已完整发出的段，以及不足一段的残包，都不再重发。
	// 残包已经在路上，再发完整报文会让对端先收到截断包再收到一份副本。
	done := coveredCount(n, seg, len(msgs))
	if done >= len(msgs) {
		return len(msgs), nil
	}
	tail, werr := w.writeEach(msgs[done:])
	return done + tail, werr
}

// usoUnsupported 判断失败是不是因为套接字不接受 UDP_SEND_MSG_SIZE。
func usoUnsupported(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case windows.WSAENOPROTOOPT, windows.WSAEOPNOTSUPP:
		return true
	default:
		return false
	}
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
	c.mu.Unlock()
	// 先关掉底层套接字，打断堵在发送缓冲上的 Write。
	// 若先等发送协程退出，而它正堵在系统调用里，两边会互相等待。
	err := c.pc.Close()
	c.mu.Lock()
	for c.sender {
		c.cond.Wait()
	}
	c.mu.Unlock()
	return err
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
	rest := batch
	for attempt := 0; attempt < writeAttempts && len(rest) > 0; attempt++ {
		n, err := c.w.WriteBatch(rest)
		if n > len(rest) {
			n = len(rest)
		}
		if n > 0 {
			releaseBatch(rest[:n])
			rest = rest[n:]
		}
		if err == nil || len(rest) == 0 {
			releaseBatch(rest)
			return
		}
		if isFatalWriteErr(err) {
			c.noteFatal(err)
			releaseBatch(rest)
			return
		}
		// 调用方的 WriteTo 已经返回成功，这里只能自己重试还没发出去的报文。
		if attempt+1 < writeAttempts {
			time.Sleep(time.Millisecond)
		}
	}
	releaseBatch(rest)
}

func (c *coalesceConn) noteFatal(err error) {
	c.mu.Lock()
	if c.fatal == nil {
		c.fatal = err
	}
	c.cond.Broadcast()
	c.mu.Unlock()
}

func releaseBatch(batch []Message) {
	for i := range batch {
		putBuf(batch[i].Buf)
		batch[i] = Message{}
	}
}

// isFatalWriteErr 判断套接字是否已经不可再用。
// 报文过大、地址非法、暂时不可达只丢掉这一批，不能把整条连接标成失败。
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
		case windows.WSAENETDOWN,
			windows.WSAENOTSOCK,
			windows.WSAESHUTDOWN,
			windows.WSAEBADF:
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
