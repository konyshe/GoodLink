package proxy

import (
	"io"
	"net"

	"github.com/quic-go/quic-go"
)

func stunT2QProcess1(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	for {
		if _, err := io.Copy(tc, qc); err != nil {
			tc.Close()
			break
		}
	}
}

func stunQ2TProcess1(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	for {
		if _, err := io.Copy(qc, tc); err != nil {
			qc.Close()
			break
		}
	}
}
