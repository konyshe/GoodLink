package proxy

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyServer(addr string, stun_quic_conn quic.Connection) {
	if stun_quic_conn == nil {
		log.Println("ProcessProxyServer stun_quic_conn is nil")
		return
	}

	log.Println("ProcessProxyServer start...")

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err == nil {
			log.Printf("ProcessProxyServer new_quic_conn.AcceptStream: %v==>%v\n", stun_quic_conn.RemoteAddr(), stun_quic_conn.LocalAddr())
			new_tcp_conn, err := net.Dial("tcp", addr)
			if err == nil {
				go stunT2QProcess1(new_tcp_conn, new_quic_stream, stun_quic_conn)
				go stunQ2TProcess1(new_quic_stream, new_tcp_conn, stun_quic_conn)
				continue
			}
			break
		}
		break
	}
}
