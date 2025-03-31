//go:build windows

package netstack

import (
	"context"
	"encoding/binary"
	pool2 "goodlink/pool"
	"goodlink/proxy"
	"log"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func ForwardTCPConn(originConn *TcpConn, stun_quic_conn quic.Connection) {
	new_quic_stream, err := stun_quic_conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Println("打开quic流失败", err)
		originConn.Close()
		return
	}

	new_quic_stream.Write([]byte{0x00})

	ipv4Bytes := originConn.ID().LocalAddress.As4()
	new_quic_stream.Write(ipv4Bytes[:]) // 添加[:]转换为切片

	portBytes := pool2.Malloc(2)
	defer pool2.Free(portBytes)

	binary.BigEndian.PutUint16(portBytes, originConn.ID().LocalPort)
	new_quic_stream.Write(portBytes)

	go proxy.ForwardQ2T(new_quic_stream, originConn, stun_quic_conn)
	go proxy.ForwardT2Q(originConn, new_quic_stream, stun_quic_conn)
}

func NewTcpForwarder(s *stack.Stack, stun_quic_conn quic.Connection) *tcp.Forwarder {
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
		/*
			log.Printf("forward tcp request: %s:%d->%s:%d",
				id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort)

			// 延迟处理错误日志
			defer func() {
				if err != nil {
					log.Printf("forward tcp request: %s:%d->%s:%d: %s",
						id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort, err)
				}
			}()
		*/
		// 执行TCP三次握手
		ep, err = r.CreateEndpoint(&wq)
		if err != nil {
			// 发送RST：防止潜在的半开TCP连接泄漏
			r.Complete(true)
			return
		}
		defer r.Complete(false)

		setSocketOptions(s, ep)

		conn := &TcpConn{
			TCPConn: gonet.NewTCPConn(&wq, ep),
			id:      id,
		}
		ForwardTCPConn(conn, stun_quic_conn)
	})
}
