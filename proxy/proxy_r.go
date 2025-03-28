package proxy

import (
	"context"
	"encoding/binary"
	"fmt"
	pool2 "goodlink/pool"
	"goodlink/socks5"
	"goodlink/utils"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyServer_Quic(remote_addr string, stun_quic_conn quic.Connection) {

	log.Printf("转发地址: %s", remote_addr)

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err == nil {
			new_tcp_conn, err := net.Dial("tcp", remote_addr)
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

func ProcessProxyServer_Socks5(remote_addr string, stun_quic_conn quic.Connection) {
	// Create a SOCKS5 server
	socks5_svr, err := socks5.New(&socks5.Config{})
	if err != nil {
		utils.Log().DebugF("代理模式: %v\n", err)
		return
	}
	log.Println("开启代理模式")

	defer log.Println("退出代理模式")

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err == nil {
			go socks5_svr.ServeConnQuic(new_quic_stream, stun_quic_conn.RemoteAddr().(*net.UDPAddr).IP, stun_quic_conn.RemoteAddr().(*net.UDPAddr).Port)
			continue
		}
		break
	}
}

func ProcessProxyServer(stun_quic_conn quic.Connection) {
	buf := pool2.Malloc(1500)
	defer pool2.Free(buf)

	for {
	fewfgwegwe:
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err != nil {
			continue
		}

		head_len := 2
		buf_len := 0
		for buf_len < head_len {
			log.Printf("读取头部: %d/%d", buf_len, head_len)
			buf_len2, err := new_quic_stream.Read(buf[buf_len:])
			if err != nil {
				log.Println("读取头部失败", err)
				new_quic_stream.Close()
				goto fewfgwegwe
			}
			buf_len += buf_len2
		}
		remotePort := binary.BigEndian.Uint16(buf[:head_len])
		remoteAddr := fmt.Sprintf("127.0.0.1:%d", remotePort)
		log.Printf("转发地址: %s", remoteAddr)
		new_tcp_conn, err := net.Dial("tcp", remoteAddr)
		if err == nil {
			if buf_len > head_len {
				new_tcp_conn.Write(buf[head_len:buf_len])
			}
			go ForwardT2Q(new_tcp_conn, new_quic_stream, stun_quic_conn)
			go ForwardQ2T(new_quic_stream, new_tcp_conn, stun_quic_conn)
			continue
		}
	}
}

func ProcessProxyServer2(remote_addr string, stun_quic_conn quic.Connection) {
	if stun_quic_conn == nil {
		log.Println("   隧道建立失败！")
		return
	}

	switch len(remote_addr) {
	case 0:
		ProcessProxyServer_Socks5(remote_addr, stun_quic_conn)
	default:
		ProcessProxyServer_Quic(remote_addr, stun_quic_conn)
	}
}
