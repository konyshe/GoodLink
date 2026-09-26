package proxy

import (
	"encoding/binary"
	"fmt"
	go2pool "go2/pool"
	"goodlink/tls2"
	"io"
	"net"
	"sync"
	"time"
)

const (
	// 64KB 是 go2/pool 能回收的上限；再大每次 Copy 都会 make，反而更慢。
	copyBufSize = 64 * 1024

	// maxUDPFramePayload 是单个 UDP 数据报的最大负载。
	// 隧道是字节流, 数据报边界由 2 字节长度前缀承载。
	maxUDPFramePayload = 65535

	// UDP 会话空闲超时（任一方向有数据都会刷新）。
	// 无超时则 DNS 短请求会留下端点+流+两条 goroutine。
	UDPIdleTimeout = 60 * time.Second

	// TCP 转发此前完全没有回收: 一侧静默失联(掉电、NAT 老化且不发 FIN)时,
	// 两个 goroutine + 本地 socket + 一条隧道流会一直悬挂。
	// 取值远大于正常业务间隔, 只在真正静默时回收。
	TCPIdleTimeout = 10 * time.Minute
)

type closeWriter interface {
	CloseWrite() error
}

func closeWrite(c any) {
	if cw, ok := c.(closeWriter); ok {
		_ = cw.CloseWrite()
		return
	}
	// quic.Stream.Close 只关写侧，对端 Copy 能读到 EOF；smux 的 Close 会关整条流。
	if s, ok := c.(io.Closer); ok {
		_ = s.Close()
	}
}

func copyBuf(dst io.Writer, src io.Reader, bump func()) {
	buf := go2pool.Malloc(copyBufSize)
	defer go2pool.Free(buf)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if bump != nil {
				bump()
			}
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

// Relay 双向拷贝，一侧 EOF 后半关闭写，等两边都结束后再全关。
// 旧实现两侧 defer 同时 Close/CancelRead，FIN 会被对端当成 RST，HTTP 等协议会异常。
func Relay(conn net.Conn, stream tls2.Stream) {
	relay(conn, stream, TCPIdleTimeout)
}

func RelayIdle(conn net.Conn, stream tls2.Stream, idle time.Duration) {
	relay(conn, stream, idle)
}

// newIdleTimer 返回一个刷新函数与停止函数, 空闲到期时把两端的读写都置为已超时。
func newIdleTimer(idle time.Duration, conn net.Conn, stream tls2.Stream) (bump func(), stop func()) {
	bump = func() {}
	stop = func() {}
	if idle <= 0 {
		return
	}

	timer := time.AfterFunc(idle, func() {
		_ = conn.SetDeadline(time.Now())
		_ = stream.SetReadDeadline(time.Now())
		_ = stream.SetWriteDeadline(time.Now())
	})
	stop = func() { timer.Stop() }

	var mu sync.Mutex
	bump = func() {
		mu.Lock()
		timer.Reset(idle)
		mu.Unlock()
	}
	return
}

func relay(conn net.Conn, stream tls2.Stream, idle time.Duration) {
	bump, stopTimer := newIdleTimer(idle, conn, stream)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		copyBuf(stream, conn, bump)
		closeWrite(stream)
	}()
	go func() {
		defer wg.Done()
		copyBuf(conn, stream, bump)
		closeWrite(conn)
	}()
	wg.Wait()
	stopTimer()
	stream.CancelRead(0)
	_ = stream.Close()
	_ = conn.Close()
}

// writeFrame 把一个数据报写入字节流: 前 2 字节是负载长度(大端)。
// 隧道是字节流, 不显式保留边界时, 多个数据报会被合并成一个 UDP 包发给目标
// (DNS 等协议会视为畸形报文直接丢弃), 大数据报也会被拆成多个。
func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxUDPFramePayload {
		return fmt.Errorf("datagram too large: %d", len(payload))
	}

	buf := go2pool.Malloc(2 + len(payload))
	defer go2pool.Free(buf)
	binary.BigEndian.PutUint16(buf[:2], uint16(len(payload)))
	copy(buf[2:], payload)

	_, err := w.Write(buf[:2+len(payload)])
	return err
}

// readFrame 从字节流中读出一个完整的数据报到 buf, 返回负载长度。
// 长度非法时返回错误: 此时流的帧边界已经无法对齐, 只能关闭这条流。
func readFrame(r io.Reader, buf []byte) (int, error) {
	var hdr [2]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, err
	}

	n := int(binary.BigEndian.Uint16(hdr[:]))
	// 零长度数据报是合法的, 只拒绝超过缓冲区(也就超过协议上限)的长度,
	// 否则一个空数据报就会让整条流被判为错位而关闭
	if n > len(buf) {
		return 0, fmt.Errorf("invalid datagram frame length: %d", n)
	}
	if _, err := io.ReadFull(r, buf[:n]); err != nil {
		return 0, err
	}
	return n, nil
}

// RelayUDPIdle 在 UDP 端点与隧道流之间搬运数据报, 并保留数据报边界。
// conn 是 UDP 语义的连接(每次 Read 返回一个数据报), 因此两端都要走分帧。
func RelayUDPIdle(conn net.Conn, stream tls2.Stream, idle time.Duration) {
	bump, stop := newIdleTimer(idle, conn, stream)
	defer stop()

	var wg sync.WaitGroup
	wg.Add(2)

	// 隧道 -> UDP: 先按长度前缀还原出数据报边界, 再整体写出
	go func() {
		defer wg.Done()
		buf := go2pool.Malloc(copyBufSize)
		defer go2pool.Free(buf)
		for {
			n, err := readFrame(stream, buf)
			if n > 0 {
				bump()
				if _, werr := conn.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	// UDP -> 隧道: 每读到一个数据报就带长度前缀写入
	go func() {
		defer wg.Done()
		buf := go2pool.Malloc(copyBufSize)
		defer go2pool.Free(buf)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				bump()
				if werr := writeFrame(stream, buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	wg.Wait()
	stream.CancelRead(0)
	_ = stream.Close()
	_ = conn.Close()
}
