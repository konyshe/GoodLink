package proxy

import (
	"io"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func stunT2QProcess1(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	for {
		if _, err := io.Copy(tc, qc); err != nil {
			log.Printf("udp.conn: %v, quic.conn: %v, quic.stream: %v, err: %v\n", tc.RemoteAddr(), stun_quic_conn.RemoteAddr(), qc.StreamID(), err)
			//log.Printf("   stunT2QProcess1 tcp close: %v ==> %v\n", tc.RemoteAddr(), tc.LocalAddr())
			tc.Close()
			//log.Printf("   stunT2QProcess1 quic stream close: %v, %v ==> %v\n", qc.StreamID(), stun_quic_conn.RemoteAddr(), stun_quic_conn.LocalAddr())
			qc.Close()
			break
		}
	}
}

func stunQ2TProcess1(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	for {
		if _, err := io.Copy(qc, tc); err != nil {
			log.Printf("udp.conn: %v, quic.conn: %v, quic.stream: %v, err: %v\n", tc.RemoteAddr(), stun_quic_conn.RemoteAddr(), qc.StreamID(), err)
			//log.Printf("   stunQ2TProcess1 tcp close: %v ==> %v\n", tc.RemoteAddr(), tc.LocalAddr())
			tc.Close()
			//log.Printf("   stunQ2TProcess1 quic stream close: %v, %v ==> %v\n", qc.StreamID(), stun_quic_conn.RemoteAddr(), stun_quic_conn.LocalAddr())
			qc.Close()
			break
		}
	}
}
