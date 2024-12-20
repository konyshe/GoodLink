package process

import (
	"goodlink/proxy"
	"goodlink/tools"
	"goodlink/tunnel"
	_ "goodlink/tunnel"
	"log"
	"net"
	"os"
	"time"

	"github.com/quic-go/quic-go"
)

func RunRemote(remote_addr string, tun_key string, time_out time.Duration) error {
	if remote_addr == "" {
		remote_addr = tools.GetFreeLocalAddr()
		if remote_addr == "" {
			log.Panic("   获取本地端口失败")
			os.Exit(0)
		}
		go proxy.ListenSocks5(remote_addr)
	}

	for {
		pro := tunnel.TunnelServer{
			RedisTimeOut:    time_out * 3,
			SocketTimeOut:   time_out,
			TunQuicConn:     nil,
			TunHealthStream: nil,
			Conn:            nil,
			ConnList:        make([]*net.UDPConn, 0),
		}

		conn := pro.GetQuicConn()
		if conn == nil {
			pro.Release()
			continue
		}

		go func(remote string, svr *tunnel.TunnelServer, conn quic.Connection) {
			defer svr.Release()
			go proxy.ProcessProxyServer(remote, conn)
			tunnel.ProcessHealth(svr.TunHealthStream)
			log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		}(remote_addr, &pro, conn)
	}
}
