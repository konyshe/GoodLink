package proxy

import (
	go2pool "go2/pool"
	"goodlink/tls2"
	"io"
	"net"
)

func ForwardT2Q(tc net.Conn, qc tls2.Stream) {
	defer func() {
		qc.CancelRead(0)
		qc.Close()
		tc.Close()
	}()

	buf := go2pool.Malloc(32 * 1024) // 32KB缓冲区提升吞吐量
	defer go2pool.Free(buf)
	io.CopyBuffer(qc, tc, buf) // 从TCP复制到隧道流
}

func ForwardQ2T(qc tls2.Stream, tc net.Conn) {
	defer func() {
		qc.CancelRead(0)
		qc.Close()
		tc.Close()
	}()

	buf := go2pool.Malloc(32 * 1024) // 32KB缓冲区提升吞吐量
	defer go2pool.Free(buf)
	io.CopyBuffer(tc, qc, buf) // 从隧道流复制到TCP
}
