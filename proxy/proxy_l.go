package proxy

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyClient(addr string, stun_quic_conn quic.Connection) {
	log.Println("ProcessProxyClient start...")

	if stun_quic_conn == nil {
		log.Println("ProcessProxyClient stun_quic_conn is nil")
		return
	}

	// 创建 listener
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("net.Listen(tcp, addr): %v", err)
		return
	}
	defer listener.Close()

	// 监听并接受来自客户端的连接
	for {
		new_tcp_conn, err := listener.Accept()
		if err == nil {
			log.Printf("ProcessProxyClient listener.Accept: %v==>%v\n", new_tcp_conn.RemoteAddr(), new_tcp_conn.LocalAddr())
			new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
			if err == nil {
				log.Printf("ProcessProxyClient stun_quic_conn.OpenStreamSync: %v==>%v\n", stun_quic_conn.RemoteAddr(), stun_quic_conn.LocalAddr())
				go stunT2QProcess1(new_tcp_conn, new_quic_stream, stun_quic_conn)
				go stunQ2TProcess1(new_quic_stream, new_tcp_conn, stun_quic_conn)
				continue
			}
			break
		}
		break
	}
}
