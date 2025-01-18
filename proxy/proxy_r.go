package proxy

import (
	"context"
	"goodlink/socks5"
	"goodlink/utils"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyServer_Quic(remote_addr string, stun_quic_conn quic.Connection) {

	utils.Log().DebugF("转发地址: %s", remote_addr)

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err == nil {
			new_tcp_conn, err := net.Dial("tcp", remote_addr)
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

func ProcessProxyServer(remote_addr string, stun_quic_conn quic.Connection) {
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
