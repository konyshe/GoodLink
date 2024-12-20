package process

import (
	"fmt"
	"goodlink/proxy"
	"goodlink/tunnel"
	_ "goodlink/tunnel"
	"log"
	"net"
)

func RunLocal(tun_local_addr string, tun_key string, retry bool) error {
	listener, err := net.Listen("tcp", tun_local_addr)
	if listener == nil || err != nil {
		return fmt.Errorf("地址监听失败: %v", tun_local_addr)
	}
	defer listener.Close()

	count := 0

	for {
		tunnelClient := tunnel.TunPassive{
			TunQuicConn:     nil,
			TunHealthStream: nil,
			TunState:        1,
			ConnList:        make([]*net.UDPConn, 0),
		}

		count++

		conn := tunnelClient.GetQuicConn(count)
		if conn == nil {
			tunnelClient.Release()
			continue
		}

		chain := make(chan int, 1)
		go func() {
			proxy.ProcessProxyClient(listener, conn)
			chain <- 1
		}()

		tunnel.ProcessHealth(tunnelClient.TunHealthStream)
		log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		tunnelClient.Release()

		if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
			conn.Write([]byte("hello"))
			conn.Close() // 关闭连接
		}

		<-chain
		count = 0

		if !retry {
			return fmt.Errorf("   连接已断开")
		}
	}
}
