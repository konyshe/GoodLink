package proxy

import (
	"context"
	"encoding/binary"
	"go2/log"
	go2pool "go2/pool"
	"io"
	"net"
	proxy_handle "proxy/handle"

	"github.com/quic-go/quic-go"
)

func ProcessProxyServer(stun_quic_conn *quic.Conn) {
	head_len := 7 // 1字节传输协议类型 + 4字节IPv4地址 + 2字节端口号

	proxy_handle.Init()
	log.Info("开启代理模式")

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err != nil {
			return
		}

		buf := go2pool.Malloc(head_len)
		_, err = io.ReadFull(new_quic_stream, buf[:head_len])
		if err != nil {
			log.Error("read quic head: ", err)
			new_quic_stream.Close()
			go2pool.Free(buf)
			continue
		}
		remotePort := binary.BigEndian.Uint16(buf[head_len-2 : head_len])
		go2pool.Free(buf)

		switch buf[0] {
		case 0x00: // TCP
			switch remotePort {
			case 1080:
				go func() {
					defer new_quic_stream.Close()
					remoteAddr := stun_quic_conn.RemoteAddr().(*net.UDPAddr)
					proxy_handle.Serve(new_quic_stream, remoteAddr.String())
				}()
			default:
				// 用户反馈无法连接3389端口，修改端口后可以连接
				// 这里是为了方便用户，直接访问13389端口就可以连接到3389端口
				if remotePort == 13389 {
					remotePort = 3389
				}
				new_conn, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: int(remotePort),
				})
				if err == nil {
					go ForwardT2Q(new_conn, new_quic_stream)
					go ForwardQ2T(new_quic_stream, new_conn)
				} else {
					log.Error("dial tcp error: ", err)
					new_quic_stream.Close()
				}
			}
		case 0x01: // UDP
			switch remotePort {
			//case 1080:

			default:
				new_conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: int(remotePort),
				})
				if err == nil {
					go ForwardT2Q(new_conn, new_quic_stream)
					go ForwardQ2T(new_quic_stream, new_conn)
				} else {
					log.Error("dial udp error: ", err)
					new_quic_stream.Close()
				}
			}
		}
	}
}
