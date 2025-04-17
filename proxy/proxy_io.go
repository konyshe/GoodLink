package proxy

import (
	"io"
	"net"

	"github.com/quic-go/quic-go"
)

func ForwardT2Q(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	defer func() {
		qc.Close()
		tc.Close()
	}()

	io.Copy(tc, qc)
}

func ForwardQ2T(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	defer func() {
		qc.Close()
		tc.Close()
	}()

	io.Copy(qc, tc)
}
