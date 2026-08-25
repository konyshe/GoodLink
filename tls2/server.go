package tls2

import (
	"crypto/tls"
	"net"
	"strings"
)

func GetServerTLSConfig() *tls.Config {
	initOnce.Do(initTLSConfigs)
	return serverConfig
}

// Listen 在已打洞的 PacketConn 上等待对端。transport 为 kcp 走明文 KCP+smux，否则走 QUIC。
func Listen(conn net.PacketConn, transport string) (Listener, error) {
	if strings.EqualFold(strings.TrimSpace(transport), "kcp") {
		return listenKCP(conn)
	}
	return listenQUIC(conn)
}
