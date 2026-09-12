package proxy

import (
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

	// UDP 会话空闲超时（任一方向有数据都会刷新）。
	// 无超时则 DNS 短请求会留下端点+流+两条 goroutine。
	UDPIdleTimeout = 60 * time.Second
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
	relay(conn, stream, 0)
}

func RelayIdle(conn net.Conn, stream tls2.Stream, idle time.Duration) {
	relay(conn, stream, idle)
}

func relay(conn net.Conn, stream tls2.Stream, idle time.Duration) {
	bump := func() {}
	var stopTimer func()
	if idle > 0 {
		timer := time.AfterFunc(idle, func() {
			_ = conn.SetDeadline(time.Now())
			_ = stream.SetReadDeadline(time.Now())
			_ = stream.SetWriteDeadline(time.Now())
		})
		stopTimer = func() { timer.Stop() }
		var mu sync.Mutex
		bump = func() {
			mu.Lock()
			timer.Reset(idle)
			mu.Unlock()
		}
	}

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
	if stopTimer != nil {
		stopTimer()
	}
	stream.CancelRead(0)
	_ = stream.Close()
	_ = conn.Close()
}
