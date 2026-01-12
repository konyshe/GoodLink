package proxy

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyClient(listener net.Listener, stun_quic_conn *quic.Conn) {
	log.Println("您已可以访问remote端的主机, 请勿关闭本程序")

	for {
		new_tcp_conn, err := listener.Accept()
		if err != nil {
			log.Println("accept error:", err)
			break
		}

		new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
		if err != nil {
			log.Println("open stream error:", err)
			new_tcp_conn.Close()
			break
		}

		go ForwardT2Q(new_tcp_conn, new_quic_stream)
		go ForwardQ2T(new_quic_stream, new_tcp_conn)
	}
}
