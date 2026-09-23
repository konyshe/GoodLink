package tls2

import (
	"crypto/tls"
	"goodlink/udpbatch"
	"net"
	"strings"
)

func GetServerTLSConfig() *tls.Config {
	initOnce.Do(initTLSConfigs)
	return serverConfig
}

// Listen 在已打洞的 PacketConn 上等待对端。transport 为 kcp 走明文 KCP+smux，否则走 QUIC。
func Listen(conn net.PacketConn, transport string) (Listener, error) {
	// Windows 上合并同目的地 UDP 为一次 USO 发送。Linux 原样返回，保留 kcp-go 的 sendmmsg 和 quic-go 的 GSO。
	conn = udpbatch.Wrap(conn)
	if strings.EqualFold(strings.TrimSpace(transport), "kcp") {
		return listenKCP(conn)
	}
	return listenQUIC(conn)
}
