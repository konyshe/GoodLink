package tools

import (
	"log"
	"net"
)

func GetListenUDP() *net.UDPConn {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		log.Panic("   net.ListenUDP: ", err)
	}
	return conn
}
