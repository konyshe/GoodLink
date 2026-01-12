package netstack

import (
	"context"
	"encoding/binary"
	"goodlink/proxy"
	"log"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func ForwardTCPConn(originConn *TcpConn, stun_quic_conn *quic.Conn) {
	new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("打开quic流失败", err)
		originConn.Close()
		return
	}

	// 使用缓冲池获取头部缓冲区
	headerBuf := getHeaderBuffer()
	defer putHeaderBuffer(headerBuf)

	// 批量构建头部数据：协议类型(1字节) + IP地址(4字节) + 端口(2字节)
	buf := (*headerBuf)[:7]
	buf[0] = 0x00 // TCP协议标识

	// 写入IPv4地址
	ipv4Bytes := originConn.ID().LocalAddress.As4()
	copy(buf[1:5], ipv4Bytes[:])

	// 写入端口（大端序）
	binary.BigEndian.PutUint16(buf[5:7], originConn.ID().LocalPort)

	// 一次性写入所有头部数据
	if _, err := new_quic_stream.Write(buf); err != nil {
		log.Println("写入头部失败", err)
		originConn.Close()
		new_quic_stream.Close()
		return
	}

	go proxy.ForwardQ2T(new_quic_stream, originConn)
	go proxy.ForwardT2Q(originConn, new_quic_stream)
}

func NewTcpForwarder(s *stack.Stack, stun_quic_conn *quic.Conn) *tcp.Forwarder {
	return tcp.NewForwarder(s, 0, 2048, func(r *tcp.ForwarderRequest) {
		var (
			wq  waiter.Queue
			ep  tcpip.Endpoint
			err tcpip.Error
			id  = r.ID()
		)

		if stun_quic_conn == nil {
			r.Complete(true) // 拒绝连接
			return
		}

		if id.LocalPort == 31337 {
			// 31337端口仅用于UDP
			return
		}

		// 开始三次握手
		ep, err = r.CreateEndpoint(&wq)
		if err != nil {
			log.Printf("forward tcp request: %s:%d->%s:%d: %s", id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort, err)

			// 发送RST：防止潜在的半开TCP连接泄漏
			r.Complete(true)
			return
		}
		defer r.Complete(false)

		setSocketOptions(s, ep)

		ForwardTCPConn(&TcpConn{
			TCPConn: gonet.NewTCPConn(&wq, ep),
			id:      id,
		}, stun_quic_conn)
	})
}
