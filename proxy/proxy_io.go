package proxy

import (
	"io"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func stunT2QProcess1(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	for {
		if n, err := io.Copy(tc, qc); n == 0 || err != nil {
			tc.Close()
			qc.Close()
			if err != nil {
				log.Printf("udp.conn: %v, quic.conn: %v, quic.stream: %v, err: %v\n", tc.RemoteAddr(), stun_quic_conn.RemoteAddr(), qc.StreamID(), err)
			}
			break
		}
	}
}

func stunQ2TProcess1(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	for {
		if n, err := io.Copy(qc, tc); n == 0 || err != nil {
			tc.Close()
			qc.Close()
			if err != nil {
				log.Printf("udp.conn: %v, quic.conn: %v, quic.stream: %v, err: %v\n", tc.RemoteAddr(), stun_quic_conn.RemoteAddr(), qc.StreamID(), err)
			}
			break
		}
	}
}
