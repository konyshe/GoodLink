package main

import (
	"context"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func process_proxy_remote(addr string, stun_quic_conn quic.Connection) {
	log.Println("process_proxy_remote start...")

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err == nil && new_quic_stream != nil {
			log.Printf("process_proxy_remote new_quic_conn.AcceptStream: %v==>%v\n", stun_quic_conn.RemoteAddr(), stun_quic_conn.LocalAddr())
			new_tcp_conn, err := net.Dial("tcp", addr)
			if err == nil && new_tcp_conn != nil {
				go stunT2QProcess1(new_tcp_conn, new_quic_stream)
				go stunQ2TProcess1(new_quic_stream, new_tcp_conn)
			}
		}
	}
}
