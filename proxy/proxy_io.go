package proxy

import (
	"goodlink/pool"
	"io"
	"net"

	"github.com/quic-go/quic-go"
)

func ForwardT2Q(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	buf := pool.Malloc(1500)

	defer func() {
		pool.Free(buf)
		qc.Close()
		tc.Close()
	}()

	io.CopyBuffer(tc, qc, buf)
}

func ForwardQ2T(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	buf := pool.Malloc(1500)

	defer func() {
		pool.Free(buf)
		qc.Close()
		tc.Close()
	}()

	io.CopyBuffer(qc, tc, buf)
}
