package netstack

import (
	"context"
	"encoding/binary"
	"goodlink/proxy"
	"log"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func ForwardUdpConn(originConn *udpConn, stun_quic_conn *quic.Conn) {
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
	buf[0] = 0x01 // UDP协议标识

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

func NewUdpForwarder(s *stack.Stack, stun_quic_conn *quic.Conn) *udp.Forwarder {
	return udp.NewForwarder(s, func(r *udp.ForwarderRequest) bool {
		var (
			wq waiter.Queue
			id = r.ID()
		)

		if stun_quic_conn == nil {
			return false
		}

		// 创建UDP端点
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			// 记录UDP转发请求错误
			log.Printf("forward udp request: %s:%d->%s:%d: %s", id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort, err)
			return false
		}

		ForwardUdpConn(&udpConn{
			UDPConn: gonet.NewUDPConn(&wq, ep),
			id:      id,
		}, stun_quic_conn)
		return true
	})
}
