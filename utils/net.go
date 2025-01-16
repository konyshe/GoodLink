package utils

import (
	"fmt"
	"net"
)

func GetListenUDP() *net.UDPConn {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		Log().ErrorF("绑定端口失败: %v", err)
	}
	return conn
}

func GetListenUDP2(port int) *net.UDPConn {
	if addr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf(":%d", port)); addr != nil && err == nil {
		if conn, err := net.ListenUDP("udp4", addr); conn != nil && err == nil {
			return conn
		}
	}
	return nil
}
