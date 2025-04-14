package proxy

import (
	"context"
	"encoding/binary"
	pool2 "goodlink/pool"
	"goodlink/socks5"
	"goodlink/utils"
	"io"
	"log"
	"net"

	"github.com/quic-go/quic-go"
)

func ProcessProxyServer(stun_quic_conn quic.Connection) {
	head_len := 7 // 1字节传输协议类型 + 4字节IPv4地址 + 2字节端口号
	buf := pool2.Malloc(head_len)
	defer pool2.Free(buf)

	socks5_svr, err := socks5.New(&socks5.Config{})
	if err != nil {
		utils.Log().DebugF("代理模式: %v\n", err)
		return
	}
	log.Println("开启代理模式")

	for {
	fewfgwegwe:
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err != nil {
			return
		}

		_, err = io.ReadFull(new_quic_stream, buf[:head_len])
		if err != nil {
			log.Println("read quic head: ", err)
			new_quic_stream.Close()
			goto fewfgwegwe
		}
		remotePort := binary.BigEndian.Uint16(buf[head_len-2 : head_len])

		switch buf[0] {
		case 0x00:
			switch remotePort {
			case 1080:
				go func() {
					remoteAddr := stun_quic_conn.RemoteAddr().(*net.UDPAddr)
					socks5_svr.ServeConnQuic(new_quic_stream, remoteAddr.IP, remoteAddr.Port)
				}()
			default:
				new_conn, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: int(remotePort),
				})
				if err == nil {
					go ForwardT2Q(new_conn, new_quic_stream, stun_quic_conn)
					go ForwardQ2T(new_quic_stream, new_conn, stun_quic_conn)
				}
			}
		case 0x01:
			switch remotePort {
			//case 1080:

			default:
				new_conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
					IP:   net.IPv4(127, 0, 0, 1),
					Port: int(remotePort),
				})
				if err == nil {
					go ForwardT2Q(new_conn, new_quic_stream, stun_quic_conn)
					go ForwardQ2T(new_quic_stream, new_conn, stun_quic_conn)
				}
			}
		}
	}
}
