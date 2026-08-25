package tls2

import (
	"context"
	"crypto/tls"
	"net"
	"strings"
)

func GetClientTLSConfig() *tls.Config {
	initOnce.Do(initTLSConfigs)
	return clientConfig
}

// Dial 在已打洞的 PacketConn 上建连。transport 为 kcp 走明文 KCP+smux，否则走 QUIC。
func Dial(ctx context.Context, conn net.PacketConn, remoteAddr net.Addr, transport string) (Conn, error) {
	if strings.EqualFold(strings.TrimSpace(transport), "kcp") {
		return dialKCP(conn, remoteAddr)
	}
	return dialQUIC(ctx, conn, remoteAddr)
}
