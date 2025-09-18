package proxy

import (
	pool2 "go2/pool"
	"io"
	"net"

	"github.com/quic-go/quic-go"
)

func ForwardT2Q(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	defer func() {
		qc.Close()
		tc.Close()
	}()

	buf := pool2.Malloc(32 * 1024) // 32KB缓冲区提升吞吐量
	defer pool2.Free(buf)
	io.CopyBuffer(tc, qc, buf)
}

func ForwardQ2T(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	defer func() {
		qc.Close()
		tc.Close()
	}()

	buf := pool2.Malloc(32 * 1024) // 32KB缓冲区提升吞吐量
	defer pool2.Free(buf)
	io.CopyBuffer(qc, tc, buf)
}
