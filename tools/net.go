package tools

import (
	"log"
	"net"
	"time"
)

func GetListenUDP() *net.UDPConn {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		log.Panic("   net.ListenUDP: ", err)
	}
	conn.SetDeadline(time.Time{})
	return conn
}
