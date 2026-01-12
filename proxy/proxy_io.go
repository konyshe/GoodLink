package proxy

import (
	go2pool "go2/pool"
	"io"
	"net"

	"github.com/quic-go/quic-go"
)

func ForwardT2Q(tc net.Conn, qc *quic.Stream) {
	defer func() {
		qc.Close()
		tc.Close()
	}()

	buf := go2pool.Malloc(32 * 1024) // 32KB缓冲区提升吞吐量
	defer go2pool.Free(buf)
	io.CopyBuffer(qc, tc, buf) // 从TCP复制到QUIC
}

func ForwardQ2T(qc *quic.Stream, tc net.Conn) {
	defer func() {
		qc.Close()
		tc.Close()
	}()

	buf := go2pool.Malloc(32 * 1024) // 32KB缓冲区提升吞吐量
	defer go2pool.Free(buf)
	io.CopyBuffer(tc, qc, buf) // 从QUIC复制到TCP
}
