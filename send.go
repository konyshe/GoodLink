package main

import (
	"fmt"
	"gogo"
	"net"
)

func process_send(conn *net.UDPConn, ip string, port int, m_send_data []byte) {
	if conn == nil {
		fmt.Println("process_send err conn: nil")
		return
	}

	if ip == "" {
		fmt.Printf("process_send err ip: %s\n", ip)
		return
	}

	if port <= 0 || port >= 65535 {
		fmt.Printf("process_send err port: %d\n", port)
		return
	}

	m_process_lock.Lock()
	defer m_process_lock.Unlock()

	if m_process_stop {
		return
	}

	remoteAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ip, port))
	assertErrorToNilf("process_send net.ResolveUDPAddr: %v", err)

	//fmt.Printf("process_send send: %v => %v\n", conn.LocalAddr(), remoteAddr)

	go func() {
		for !m_process_stop {
			_, err = conn.WriteToUDP(m_send_data, remoteAddr)
			assertErrorToNilf("process_send conn.WriteToUDP: %v", err)
			gogo.Utils().TimeSleepMilliSecond(300)
		}
	}()
}
