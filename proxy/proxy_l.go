package proxy

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyClient(listener net.Listener, stun_quic_conn quic.Connection) {
	log.Println("您已可以访问remote端的主机, 请勿关闭本程序")

	for {
		new_tcp_conn, err := listener.Accept()
		if err == nil {
			new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
			if err == nil {
				go ForwardT2Q(new_tcp_conn, new_quic_stream, stun_quic_conn)
				go ForwardQ2T(new_quic_stream, new_tcp_conn, stun_quic_conn)
				continue
			}
			break
		}
		break
	}
}
