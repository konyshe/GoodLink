package tls2

import (
	"context"
	"crypto/tls"
	"goodlink/udpbatch"
	"net"
	"strings"
)

func GetClientTLSConfig() *tls.Config {
	initOnce.Do(initTLSConfigs)
	return clientConfig
}

// Dial 在已打洞的 PacketConn 上建连。transport 为 kcp 走明文 KCP+smux，否则走 QUIC。
func Dial(ctx context.Context, conn net.PacketConn, remoteAddr net.Addr, transport string) (Conn, error) {
	// Windows 上合并同目的地 UDP 为一次 USO 发送。Linux 原样返回，保留 kcp-go 的 sendmmsg 和 quic-go 的 GSO。
	conn = udpbatch.Wrap(conn)
	if strings.EqualFold(strings.TrimSpace(transport), "kcp") {
		return dialKCP(conn, remoteAddr)
	}
	return dialQUIC(ctx, conn, remoteAddr)
}
