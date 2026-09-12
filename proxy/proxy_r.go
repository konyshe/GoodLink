package proxy

import (
	"context"
	"encoding/binary"
	"go2/log"
	go2pool "go2/pool"
	"goodlink/tls2"
	"io"
	"net"
	proxy_handle "proxy/handle"
	"strconv"
	"time"
)

const (
	HEAD_LEN   = 7 // 1字节传输协议类型 + 4字节IPv4地址 + 2字节端口号
	PROXY_PORT = 1080

	tcpDialTimeout = 10 * time.Second
)

func process_stream(new_quic_stream tls2.Stream, remoteAddrStr string) {

	headerBuf := go2pool.Malloc(HEAD_LEN)
	defer go2pool.Free(headerBuf)

	_, err := io.ReadFull(new_quic_stream, headerBuf[:HEAD_LEN])
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
		} else {
			log.Error("read quic head error: ", err)
		}
		new_quic_stream.CancelRead(0)
		new_quic_stream.Close()
		return
	}

	protocolType := headerBuf[0]
	remotePort := binary.BigEndian.Uint16(headerBuf[5:HEAD_LEN])
	// 拷出 IP：headerBuf 来自缓冲池，不能把子切片交给 Dial。
	ip := make(net.IP, 4)
	copy(ip, headerBuf[1:5])

	switch protocolType {
	case 0x00: // TCP
		switch remotePort {
		case PROXY_PORT:
			proxy_handle.Serve(new_quic_stream, remoteAddrStr)
			new_quic_stream.CancelRead(0)
			new_quic_stream.Close()
		default:
			// 用户反馈无法连接3389端口，修改端口后可以连接
			// 这里是为了方便用户，直接访问13389端口就可以连接到3389端口
			if remotePort == 13389 {
				remotePort = 3389
			}
			d := &net.Dialer{
				Timeout:   tcpDialTimeout,
				KeepAlive: 30 * time.Second,
			}
			c, err := d.Dial("tcp4", net.JoinHostPort(ip.String(), strconv.Itoa(int(remotePort))))
			if err == nil {
				if tcpConn, ok := c.(*net.TCPConn); ok {
					// 对端内核 TCP 默认开 Nagle，与 gVisor 侧叠加会拖慢 RDP/SSH。
					_ = tcpConn.SetNoDelay(true)
				}
				Relay(c, new_quic_stream)
			} else {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
				} else {
					log.Error("dial tcp error: ", err)
				}
				new_quic_stream.CancelRead(0)
				new_quic_stream.Close()
			}
		}
	case 0x01: // UDP
		new_conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
			IP:   ip,
			Port: int(remotePort),
		})
		if err == nil {
			_ = new_conn.SetReadBuffer(256 * 1024)
			_ = new_conn.SetWriteBuffer(256 * 1024)
			RelayIdle(new_conn, new_quic_stream, UDPIdleTimeout)
		} else {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
			} else {
				log.Error("dial udp error: ", err)
			}
			new_quic_stream.CancelRead(0)
			new_quic_stream.Close()
		}
	}
}

func ProcessProxyServer(stun_quic_conn tls2.Conn) {
	proxy_handle.Init()
	log.Info("开启代理模式")

	remoteAddrStr := stun_quic_conn.RemoteAddr().String()

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err != nil {
			log.Error("ProcessProxyServer AcceptStream error: ", err)
			return
		}

		go process_stream(new_quic_stream, remoteAddrStr)
	}
}
