package stun2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math/rand"
	"net"
	"time"
)

func getStunIpPort2(conn *net.UDPConn, addr string) (string, int, error) {
	log.Printf("stun_svr: %s\n", addr)

	rand.Seed(time.Now().UnixNano())

	udpAddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return "", 0, err
	}

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

	// STUN message header
	buf := new(bytes.Buffer)
	// Start with fixed 0x00, message type: 0x01, message length: 0x0000
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00})
	magicCookie := []byte{0x21, 0x12, 0xA4, 0x42}
	buf.Write(magicCookie)
	transactionID := make([]byte, 12)
	rand.Read(transactionID)
	buf.Write(transactionID)

	_, err = conn.WriteToUDP(buf.Bytes(), udpAddr)
	if err != nil {
		return "", 0, err
	}

	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		return "", 0, err
	}
	if n < 32 {
		return "", 0, fmt.Errorf("invalid response")
	}

	// Parse STUN message
	if !bytes.Equal(response[4:8], buf.Bytes()[4:8]) {
		return "", 0, fmt.Errorf("invalid magic cookie in response")
	}
	if !bytes.Equal(response[8:20], buf.Bytes()[8:20]) {
		return "", 0, fmt.Errorf("transaction ID mismatch in response")
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

	// Parse STUN attributes
	attributes := response[20:]

	attrType := binary.BigEndian.Uint16(attributes[:2])
	// Mapped Address && Xor-Mapped Address
	if attrType != 0x0001 && attrType != 0x0020 {
		return "", 0, fmt.Errorf("invalid address attribute type")
	}
	attrLength := binary.BigEndian.Uint16(attributes[2:4])
	if attrLength < 8 {
		return "", 0, fmt.Errorf("invalid address attribute length")
	}

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

var m_stun_list []string

func init() {
	m_stun_list = []string{
		"stun.easyvoip.com:3478",
		"stun.easyvoip.com:3478",
		"s1.taraba.net:3478",
		"s2.taraba.net:3478",
		"s1.voipstation.jp:3478",
		"s2.voipstation.jp:3478",
		"stun.xten.com:3478",
		"stun.voipbuster.com:3478",
		"stun.sipgate.net:3478",
		"stun.ekiga.net:3478",
		"stun.ideasip.com:3478",
		"stun.schlund.de:3478",
		"stun.voiparound.com:3478",
		"stun.voipbuster.com:3478",
		"stun.voipstunt.com:3478",
		"stun.counterpath.com:3478",
		"stun.1und1.de:3478",
		"stun.gmx.net:3478",
		"stun.callwithus.com:3478",
		"stun.counterpath.net:3478",
		"stun.internetcalls.com:3478",
		"numb.viagenie.ca:3478",
		"124.223.50.150:13478",
	}
}

func GetWanIpPort(conn *net.UDPConn) (wan_ip string, wan_port int) {
	for {
		stun_svr := m_stun_list[rand.Intn(len(m_stun_list))]
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		if wan_ip, wan_port, _ = getStunIpPort2(conn, stun_svr); wan_ip != "" && wan_port > 0 {
			conn.SetReadDeadline(time.Time{})
			break
		}
	}
	return
}

func TestStun() {
	conn, err := net.ListenUDP("udp4", nil)
	if err == nil {
		for _, stun_svr := range m_stun_list {
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			if wan_ip, wan_port, _ := getStunIpPort2(conn, stun_svr); wan_ip != "" && wan_port > 0 {
				conn.SetReadDeadline(time.Time{})
				//fmt.Printf("stun_svr: %s, wan_ip: %s, wan_port: %d\n", stun_svr, wan_ip, wan_port)
			} else {
				fmt.Printf("stun_svr: %s, failed\n", stun_svr)
			}
		}
	}
}
