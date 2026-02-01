package proxy

import (
	"context"
	"encoding/binary"
	"go2/log"
	"io"
	"net"
	proxy_handle "proxy/handle"

	"github.com/quic-go/quic-go"
)

const (
	head_len = 7 // 1字节传输协议类型 + 4字节IPv4地址 + 2字节端口号
)

func ProcessProxyServer(stun_quic_conn *quic.Conn) {
	proxy_handle.Init()
	log.Info("开启代理模式")

	// 提前获取并缓存远程地址，避免在 goroutine 中重复调用
	remoteAddr := stun_quic_conn.RemoteAddr().(*net.UDPAddr)
	remoteAddrStr := remoteAddr.String()

	// 复用头部缓冲区，减少内存分配开销
	var headerBuf [head_len]byte

	for {
		new_quic_stream, err := stun_quic_conn.AcceptStream(context.Background())
		if err != nil {
			// 连接关闭是正常情况，不需要记录错误日志
			return
		}

		// 读取头部数据
		_, err = io.ReadFull(new_quic_stream, headerBuf[:head_len])
		if err != nil {
			// 区分连接关闭和其他错误
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				// 连接关闭是正常情况，不需要记录错误日志
			} else {
				log.Error("read quic head error: ", err)
			}
			new_quic_stream.Close()
			continue
		}

		// 在释放缓冲区前提取所有需要的数据
		protocolType := headerBuf[0]
		remotePort := binary.BigEndian.Uint16(headerBuf[head_len-2 : head_len])

		switch protocolType {
		case 0x00: // TCP
			switch remotePort {
			case 1080:
				go func() {
					defer new_quic_stream.Close()
					proxy_handle.Serve(new_quic_stream, remoteAddrStr)
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
					// 区分连接关闭和其他错误
					if err == io.EOF || err == io.ErrUnexpectedEOF {
						// 连接关闭是正常情况，不需要记录错误日志
					} else {
						log.Error("dial tcp error: ", err)
					}
					new_quic_stream.Close()
				}
			}
		case 0x01: // UDP
			new_conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: int(remotePort),
			})
			if err == nil {
				go ForwardT2Q(new_conn, new_quic_stream)
				go ForwardQ2T(new_quic_stream, new_conn)
			} else {
				// 区分连接关闭和其他错误
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					// 连接关闭是正常情况，不需要记录错误日志
				} else {
					log.Error("dial udp error: ", err)
				}
				new_quic_stream.Close()
			}
		}
	}
}
