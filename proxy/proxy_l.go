package proxy

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyClient(listener net.Listener, stun_quic_conn quic.Connection) {
	log.Println("   ProcessProxyClient start...")

	for {
		new_tcp_conn, err := listener.Accept()
		if err == nil {
			log.Printf("   ProcessProxyClient listener.Accept: %v ==> %v\n", new_tcp_conn.RemoteAddr(), new_tcp_conn.LocalAddr())
			new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
			if err == nil {
				log.Printf("   ProcessProxyClient stun_quic_conn.OpenStreamSync: %v ==> %v\n", stun_quic_conn.RemoteAddr(), stun_quic_conn.LocalAddr())
				go stunT2QProcess1(new_tcp_conn, new_quic_stream, stun_quic_conn)
				go stunQ2TProcess1(new_quic_stream, new_tcp_conn, stun_quic_conn)
				continue
			}
			break
		}
		break
	}
}
