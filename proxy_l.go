package main

import (
	"context"
	"fmt"
	"gogo"
	"io"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

func stunT2QProcess1(tc net.Conn, qc quic.Stream) {
	for {
		tc.SetDeadline(time.Now().Add(15 * time.Minute))
		qc.SetDeadline(time.Now().Add(15 * time.Minute))
		if n, err := io.Copy(tc, qc); n == 0 || err != nil {
			log.Printf("stunT2QProcess1 tcp close: %v==>%v\n", tc.RemoteAddr(), tc.LocalAddr())
			tc.Close()
			log.Printf("stunT2QProcess1 quic stream close: %v, %v==>%v\n", qc.StreamID(), m_stun_quic_conn.RemoteAddr(), m_stun_quic_conn.LocalAddr())
			qc.Close()
			break
		}
	}
}

func stunQ2TProcess1(qc quic.Stream, tc net.Conn) {
	for {
		tc.SetDeadline(time.Now().Add(15 * time.Minute))
		qc.SetDeadline(time.Now().Add(15 * time.Minute))
		if n, err := io.Copy(qc, tc); n == 0 || err != nil {
			log.Printf("stunQ2TProcess1 tcp close: %v==>%v\n", tc.RemoteAddr(), tc.LocalAddr())
			tc.Close()
			log.Printf("stunQ2TProcess1 quic stream close: %v, %v==>%v\n", qc.StreamID(), m_stun_quic_conn.RemoteAddr(), m_stun_quic_conn.LocalAddr())
			qc.Close()
			break
		}
	}
}

func process_proxy_local(addr string) {
	fmt.Println("process_proxy_local start...")

	var err error
	var listener net.Listener

	// 创建 listener
	for {
		if listener, err = net.Listen("tcp", addr); err == nil {
			break
		}
		log.Printf("process_proxy_local listening: %v\n", err)
		gogo.Utils().TimeSleepSecond(1)
	}

	// 监听并接受来自客户端的连接
	for {
		new_tcp_conn, err := listener.Accept()
		if err == nil && new_tcp_conn != nil {
			log.Printf("process_proxy_local listener.Accept: %v==>%v\n", new_tcp_conn.RemoteAddr(), new_tcp_conn.LocalAddr())
			new_quic_stream, err := m_stun_quic_conn.OpenStreamSync(context.Background())
			if err == nil && new_quic_stream != nil {
				log.Printf("process_proxy_local m_stun_quic_conn.OpenStreamSync: %v==>%v\n", m_stun_quic_conn.RemoteAddr(), m_stun_quic_conn.LocalAddr())
				go stunT2QProcess1(new_tcp_conn, new_quic_stream)
				go stunQ2TProcess1(new_quic_stream, new_tcp_conn)
			}
		}
	}
}
