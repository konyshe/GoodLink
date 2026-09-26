package stun2

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	go2pool "go2/pool"
	"goodlink/config"
	goodlink_config "goodlink3/config"
)

// extraLogSink 可选：由 UI 设置后，STUN 相关日志会同时输出到该回调（如运行日志列表），cmd 不设置则为 nil
var extraLogSink func(string)

// SetExtraLogSink 设置额外日志回调，仅 GUI 在调用 GetStunIpPort 前设置；cmd 不调用则保持默认仅标准 log
func SetExtraLogSink(f func(string)) { extraLogSink = f }

func stunLogf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if extraLogSink != nil {
		extraLogSink(msg)
		return
	}
	log.Printf("%s", msg)
}

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
	// attrLength 取自网络报文, 必须先校验边界再切片, 否则畸形报文会越界 panic
	if len(attributes) < 4 || int(attrLength)+4 > len(attributes) {
		return "", 0, fmt.Errorf("attr length overflow: %d", attrLength)
	}
	attributeValue := attributes[4 : 4+int(attrLength)]
	if len(attributeValue) < 2 {
		return "", 0, fmt.Errorf("short address attribute")
	}
	family := attributeValue[1]
	var ip, port []byte
	switch family {
	case 1:
		if len(attributeValue) < 8 {
			return "", 0, fmt.Errorf("short ipv4 address attribute: %d", len(attributeValue))
		}
		ip = attributeValue[4:8]
		port = attributeValue[2:4]
	case 2:
		if len(attributeValue) < 20 {
			return "", 0, fmt.Errorf("short ipv6 address attribute: %d", len(attributeValue))
		}
		ip = attributeValue[4:20]
		port = attributeValue[2:4]
	default:
		return "", 0, fmt.Errorf("unknown address family: %d", family)
	}
	if attrType == 0x0020 { // XOR-Mapped Address
		if len(magicCookie) < 4 {
			return "", 0, fmt.Errorf("short magic cookie")
		}
		if len(transactionID) < 12 {
			return "", 0, fmt.Errorf("short transaction id")
		}
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

	// 解析范围取 response_len 与实际长度的较小者, 避免读到缓冲区未写入区域
	limit := response_len
	if limit > len(response) {
		limit = len(response)
	}

	for {
		if start+4 > limit {
			break
		}

		// Parse STUN attributes
		attributes := response[start:limit]
		attrType := binary.BigEndian.Uint16(attributes[:2])
		attrLength := binary.BigEndian.Uint16(attributes[2:4])
		if attrLength < 8 || 4+int(attrLength) > len(attributes) {
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

	poolBuf := go2pool.Malloc(1024)
	defer go2pool.Free(poolBuf)

	conn.SetReadDeadline(time.Now().Add(3000 * time.Millisecond))
	n, err := conn.Read(poolBuf)
	defer conn.SetReadDeadline(time.Time{})

	if err != nil {
		return nil, 0, err
	}
	if n < 32 {
		return nil, 0, fmt.Errorf("invalid response")
	}

	// Parse STUN message
	if !bytes.Equal(poolBuf[4:8], buf.Bytes()[4:8]) {
		return nil, 0, fmt.Errorf("invalid magic cookie in response")
	}
	if !bytes.Equal(poolBuf[8:20], buf.Bytes()[8:20]) {
		return nil, 0, fmt.Errorf("transaction ID mismatch in response")
	}

	// poolBuf 随 defer Free 归还池, 不能再把它的底层数组交给上层持有, 必须拷贝一份
	response := make([]byte, n)
	copy(response, poolBuf[:n])

	return response, n, nil
}

func getStunIpPort2(conn *net.UDPConn, stun_svr, addr string, buf *bytes.Buffer, magicCookie []byte, transactionID []byte) (string, int, string, int, error) {

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
		return "", 0, "", 0, fmt.Errorf("%s(%s): %s", stun_svr, addr, err.Error())
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

	// 属性区必须是本次实际读到的字节, 不能用整个缓冲区长度, 否则会解析到未写入区域
	attrs := response[20:n]

	wan_ip1, wan_port1, err := getStunIpPort4(attrs, len(attrs), 0x0001, magicCookie, transactionID)
	if err != nil {
		wan_ip1, wan_port1, err = getStunIpPort4(attrs, len(attrs), 0x0020, magicCookie, transactionID)
	}
	if err != nil {
		return "", 0, "", 0, fmt.Errorf("mapped %s(%s): %s", stun_svr, addr, err.Error())
	}

	change_ip, change_port, err := getStunIpPort4(attrs, len(attrs), 0x0005, magicCookie, transactionID)
	if err != nil {
		return "", 0, "", 0, fmt.Errorf("changed %s(%s): %s", stun_svr, addr, err.Error())
	}

	return wan_ip1, wan_port1, change_ip, change_port, nil
}

func GetStunIpPort2(stun_svr string, conn *net.UDPConn) (wan_ip string, wan_port1, wan_port2, wan_port3 int, err error) {
	var change_ip string
	var change_port int
	var ips []net.IP

	ips, err = net.LookupIP(stun_svr)
	if err != nil {
		return "", 0, 0, 0, fmt.Errorf("lookup %s: %s", stun_svr, err.Error())
	}

	// STUN message header
	var buf bytes.Buffer
	// Start with fixed 0x00, message type: 0x01, message length: 0x0000
	buf.Write([]byte{0x00, 0x01, 0x00, 0x00})
	magicCookie := []byte{0x21, 0x12, 0xA4, 0x42}
	buf.Write(magicCookie)
	transactionID := go2pool.Malloc(12)
	defer go2pool.Free(transactionID)
	if _, err := rand.Read(transactionID); err != nil {
		return "", 0, 0, 0, fmt.Errorf("generate transaction id: %s", err.Error())
	}
	buf.Write(transactionID)

	for _, ip := range ips {

		wan_ip, wan_port1, change_ip, change_port, err = getStunIpPort2(conn, stun_svr, ip.String()+":3478", &buf, magicCookie, transactionID)
		if wan_ip == "" || wan_port1 == 0 || change_ip == "" || change_port == 0 || err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		_, wan_port2, _, _, err = getStunIpPort2(conn, stun_svr, ip.String()+":3479", &buf, magicCookie, transactionID)
		if wan_port2 == 0 || err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		_, wan_port3, _, _, err = getStunIpPort2(conn, stun_svr, fmt.Sprintf("%s:%d", change_ip, change_port), &buf, magicCookie, transactionID)
		if wan_port3 == 0 || err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		return wan_ip, wan_port1, wan_port2, wan_port3, nil
	}

	return "", 0, 0, 0, err
}

func stunServerList() []string {
	if rc, ok := config.LoadRuntimeConfFromEnv(); ok && len(rc.StunList) > 0 {
		return rc.StunList
	}
	return goodlink_config.GetStunList()
}

// maxStunRounds 是整个 STUN 服务器列表的重试轮数上限。
// 任一轮全部成功则立即返回, 全部失败则退出而不是无限等待。
const maxStunRounds = 3

func GetStunIpPort(conn *net.UDPConn) (wan_ip string, wan_port1, wan_port2, wan_port3 int) {
	for round := 0; round < maxStunRounds; round++ {
		stun_svr_list := stunServerList()
		if len(stun_svr_list) == 0 {
			stunLogf("STUN 服务器列表为空, 无法获取公网地址")
			return "", 0, 0, 0
		}

		for _, stun_svr := range stun_svr_list {
			var err error

			wan_ip, wan_port1, wan_port2, wan_port3, err = GetStunIpPort2(stun_svr, conn)
			if err != nil {
				stunLogf("%s", err.Error())
				continue
			}
			// 结果不完整时不能用默认值充当有效数据: 上层会按 WanPort1==WanPort2 判定 NAT 类型,
			// 端口为 0 会被误判成 NAT1-3 从而走注定失败的主动连接
			if wan_ip == "" || wan_port1 == 0 || wan_port2 == 0 {
				stunLogf("%s: STUN 探测结果不完整(ip=%s port1=%d port2=%d)", stun_svr, wan_ip, wan_port1, wan_port2)
				continue
			}
			return wan_ip, wan_port1, wan_port2, wan_port3
		}

		time.Sleep(5 * time.Second)
	}

	stunLogf("STUN 探测失败, 已重试%d轮", maxStunRounds)
	return "", 0, 0, 0
}
