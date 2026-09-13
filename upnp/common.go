package upnp

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// GetLocalInternetIp 返回本机用于对外通信的 IPv4 地址。
// UDP Dial 不真正发包，不依赖探测目标在线。
func GetLocalInternetIp() (string, error) {
	conn, err := net.DialTimeout("udp4", "8.8.8.8:80", time.Second)
	if err != nil {
		return "", fmt.Errorf("upnp: 无法获取本机IP: %w", err)
	}
	defer conn.Close()

	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || udpAddr.IP == nil || udpAddr.IP.IsUnspecified() {
		return "", errors.New("upnp: 无法获取本机IP")
	}
	return udpAddr.IP.String(), nil
}
