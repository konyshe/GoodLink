package proxy

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyClient(listener net.Listener, stun_quic_conn quic.Connection) {
	log.Println("   您已可以访问remote端的主机 ...")

	for {
		new_tcp_conn, err := listener.Accept()
		if err == nil {
			new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
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
