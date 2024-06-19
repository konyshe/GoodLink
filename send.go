package main

import (
	"fmt"
	"gogo"
	"log"
	"net"
)

func process_send(conn *net.UDPConn, ip string, port int, m_send_data []byte, process *bool) {
	if conn == nil {
		log.Println("process_send err conn: nil")
		return
	}

	if ip == "" {
		log.Printf("process_send err ip: %s\n", ip)
		return
	}

	if port <= 0 || port >= 65535 {
		log.Printf("process_send err port: %d\n", port)
		return
	}

	remoteAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ip, port))
	assertErrorToNilf("process_send net.ResolveUDPAddr: %v", err)

	//log.Printf("process_send send: %v => %v\n", conn.LocalAddr(), remoteAddr)

	for !*process {
		_, err = conn.WriteToUDP(m_send_data, remoteAddr)
		assertErrorToNilf("process_send conn.WriteToUDP: %v", err)
		gogo.Utils().TimeSleepMilliSecond(300)
	}
}
