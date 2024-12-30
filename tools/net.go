package tools

import (
	"net"

	"gogo"
)

func GetListenUDP() *net.UDPConn {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		gogo.Log().ErrorF("   绑定端口失败: %v", err)
	}
	return conn
}
