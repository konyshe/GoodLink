package netstack

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"goodlink/proxy"
	"log"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func ForwardUdpConn(originConn *udpConn, stun_quic_conn quic.Connection) {
	new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("打开quic流失败", err)
		originConn.Close()
		return
	}

	new_quic_stream.Write([]byte{0x01})

	ipv4Bytes := originConn.ID().LocalAddress.As4()
	new_quic_stream.Write(ipv4Bytes[:]) // 添加[:]转换为切片

	portBytes := go2pool.Malloc(2)
	defer go2pool.Free(portBytes)

	binary.BigEndian.PutUint16(portBytes, originConn.ID().LocalPort)
	new_quic_stream.Write(portBytes)

	go proxy.ForwardQ2T(new_quic_stream, originConn, stun_quic_conn)
	go proxy.ForwardT2Q(originConn, new_quic_stream, stun_quic_conn)
}

func NewUdpForwarder(s *stack.Stack, stun_quic_conn quic.Connection) *udp.Forwarder {
	return udp.NewForwarder(s, func(r *udp.ForwarderRequest) {
		var (
			wq waiter.Queue
			id = r.ID()
		)

		if stun_quic_conn == nil {
			return
		}

		// 创建UDP端点
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			// 记录UDP转发请求错误
			log.Printf("forward udp request: %s:%d->%s:%d: %s", id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort, err)
			return
		}

		ForwardUdpConn(&udpConn{
			UDPConn: gonet.NewUDPConn(&wq, ep),
			id:      id,
		}, stun_quic_conn)
	})
}
