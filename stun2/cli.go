package stun2

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"goodlink/config"
	"log"
	"net"
	"time"

	go2pool "go2/pool"
)

func getStunIpPort5(attrType uint16, attributes []byte, attrLength uint16, magicCookie []byte, transactionID []byte) (string, int, error) {
	// https://www.rfc-editor.org/rfc/rfc5389.html#section-15.1
	// MAPPED-ADDRESS
	//  0                   1                   2                   3
	//   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |0 0 0 0 0 0 0 0|    Family     |           Port                |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                                                               |
	// |                 Address (32 bits or 128 bits)                 |
	// |                                                               |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// https://www.rfc-editor.org/rfc/rfc5389.html#section-15.2
	// XOR-MAPPED-ADDRESS
	//  0                   1                   2                   3
	//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |x x x x x x x x|    Family     |         X-Port                |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                X-Address (Variable)
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	attributeValue := attributes[4 : 4+attrLength]
	family := attributeValue[1]
	var ip, port []byte
	switch family {
	case 1:
		ip = attributeValue[4:8]
		port = attributeValue[2:4]
	case 2:
		ip = attributeValue[4:20]
	default:
		return "", 0, fmt.Errorf("unknown address family")
	}
	if attrType == 0x0020 { // XOR-Mapped Address
		for i := 0; i < 4; i++ {
			ip[i] ^= magicCookie[i]
		}
		if family == 2 {
			for i := 4; i < len(ip); i++ {
				ip[i] ^= transactionID[i-4]
			}
		}
	}
	port2 := binary.BigEndian.Uint16(port)
	return net.IP(ip).String(), int(port2), nil
}

func getStunIpPort4(response []byte, response_len int, needType uint16, magicCookie []byte, transactionID []byte) (string, int, error) {
	start := 0

	for {
		if start >= response_len-4 {
			break
		}

		// Parse STUN attributes
		attributes := response[start:]
		attrType := binary.BigEndian.Uint16(attributes[:2])
		attrLength := binary.BigEndian.Uint16(attributes[2:4])
		if attrLength < 8 {
			break
		}

		if attrType == needType {
			return getStunIpPort5(attrType, attributes, attrLength, magicCookie, transactionID)
		}

		start = start + int(attrLength) + 4
	}

	return "", 0, fmt.Errorf("attrType not found")
}

func getStunResponse(conn *net.UDPConn, addr string, buf *bytes.Buffer) ([]byte, int, error) {
	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, 0, err
	}

	_, err = conn.WriteToUDP(buf.Bytes(), udpAddr)
	if err != nil {
		return nil, 0, err
	}

	response := go2pool.Malloc(1024)
	defer go2pool.Free(response)

	conn.SetReadDeadline(time.Now().Add(3000 * time.Millisecond))
	n, err := conn.Read(response)
	defer conn.SetReadDeadline(time.Time{})

	if err != nil {
		return nil, 0, err
	}
	if n < 32 {
		return nil, 0, fmt.Errorf("invalid response")
	}

	// Parse STUN message
	if !bytes.Equal(response[4:8], buf.Bytes()[4:8]) {
		return nil, 0, fmt.Errorf("invalid magic cookie in response")
	}
	if !bytes.Equal(response[8:20], buf.Bytes()[8:20]) {
		return nil, 0, fmt.Errorf("transaction ID mismatch in response")
	}

	return response, n, nil
}

func getStunIpPort2(conn *net.UDPConn, addr string, buf *bytes.Buffer, magicCookie []byte, transactionID []byte) (string, int, string, int) {

	// https://www.rfc-editor.org/rfc/rfc5389.html#section-6
	// STUN Message Structure
	//   0                   1                   2                   3
	//   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |0 0|     STUN Message Type     |         Message Length        |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                         Magic Cookie                          |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                                                               |
	// |                     Transaction ID (96 bits)                  |
	// |                                                               |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	response, n, err := getStunResponse(conn, addr, buf)
	if err != nil {
		log.Printf("getStunResponse error: %v", err)
		return "", 0, "", 0
	}

	// https://www.rfc-editor.org/rfc/rfc5389.html#section-15
	// STUN Attributes
	//   0                   1                   2                   3
	//   0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |         Type                  |            Length             |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                         Value (variable)                ....
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+

	wan_ip1, wan_port1, err := getStunIpPort4(response[20:], n, 0x0001, magicCookie, transactionID)
	if err != nil {
		wan_ip1, wan_port1, err = getStunIpPort4(response[20:], n, 0x0020, magicCookie, transactionID)
	}
	if err != nil {
		log.Printf("getStunIpPort4 error: %v", err)
		return "", 0, "", 0
	}

	change_ip, change_port, err := getStunIpPort4(response[20:], n, 0x0005, magicCookie, transactionID)
	if err != nil {
		log.Printf("getStunIpPort4 error: %v", err)
	}

	return wan_ip1, wan_port1, change_ip, change_port
}

func GetStunIpPort2(stun_svr string, conn *net.UDPConn) (wan_ip string, wan_port1, wan_port2, wan_port3 int, err error) {
	var change_ip string
	var change_port int
	var ips []net.IP

	ips, err = net.LookupIP(stun_svr)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("lookup stun ip: %s, %v", stun_svr, err)
	}
	if len(ips) == 0 {
		return "", 0, 0, 0, fmt.Errorf("stun ip not found: %s", stun_svr)
	}

	// STUN message header
	var buf bytes.Buffer
	// Start with fixed 0x00, message type: 0x01, message length: 0x0000
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00})
	magicCookie := []byte{0x21, 0x12, 0xA4, 0x42}
	buf.Write(magicCookie)
	transactionID := go2pool.Malloc(12)
	defer go2pool.Free(transactionID)
	rand.Read(transactionID)
	buf.Write(transactionID)

	for _, ip := range ips {
		log.Printf("stun_svr: %s => %s", stun_svr, ip.String())

		wan_ip, wan_port1, change_ip, change_port = getStunIpPort2(conn, ip.String()+":3478", &buf, magicCookie, transactionID)
		if wan_ip == "" || wan_port1 == 0 || change_ip == "" || change_port == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		_, wan_port2, _, _ = getStunIpPort2(conn, ip.String()+":3479", &buf, magicCookie, transactionID)
		if wan_port2 == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		_, wan_port3, _, _ = getStunIpPort2(conn, fmt.Sprintf("%s:%d", change_ip, change_port), &buf, magicCookie, transactionID)
		if wan_port3 == 0 {
			time.Sleep(1 * time.Second)
			continue
		}

		return wan_ip, wan_port1, wan_port2, wan_port3, nil
	}

	return "", 0, 0, 0, fmt.Errorf("stun ip found failed")
}

func GetStunIpPort(conn *net.UDPConn) (wan_ip string, wan_port1, wan_port2, wan_port3 int) {
	var err error

	for {
		stun_svr_list := config.GetStunList()
		for _, stun_svr := range stun_svr_list {
			wan_ip, wan_port1, wan_port2, wan_port3, err = GetStunIpPort2(stun_svr, conn)
			if err != nil {
				log.Printf("%v", err)
				continue
			}
			return wan_ip, wan_port1, wan_port2, wan_port3
		}
		time.Sleep(5 * time.Second)
	}
}

func TestStun() {
	conn4, _ := net.ListenUDP("udp4", nil)
	defer conn4.Close()

	wan_ip, wan_port1, wan_port2, wan_port3 := GetStunIpPort(conn4)
	log.Printf("wan_ip: %s, wan_port1: %d, wan_port2: %d, wan_port3: %d", wan_ip, wan_port1, wan_port2, wan_port3)
}
