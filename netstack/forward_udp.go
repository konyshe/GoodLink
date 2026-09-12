package netstack

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"goodlink/proxy"
	"goodlink/tls2"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
	"log"
)

func ForwardUdpConn(originConn *udpConn, stun_quic_conn tls2.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), quicOpenStreamTimeout)
	defer cancel()

	new_quic_stream, err := stun_quic_conn.OpenStreamSync(ctx)
	if err != nil {
		log.Printf("[netstack] UDP转发打开quic流失败 %s:%d: %v", originConn.ID().LocalAddress, originConn.ID().LocalPort, err)
		originConn.Close()
		// 开流失败只关这条会话。os.Exit 会干掉整个进程。
		return
	}

	ioBuf := go2pool.Malloc(proxy.HEAD_LEN)
	defer go2pool.Free(ioBuf)

	ioBuf[0] = 0x01
	copy(ioBuf[1:5], LoopBackAddr[:])
	binary.BigEndian.PutUint16(ioBuf[5:proxy.HEAD_LEN], originConn.ID().LocalPort)

	if _, err := new_quic_stream.Write(ioBuf[:proxy.HEAD_LEN]); err != nil {
		log.Println("写入头部失败", err)
		originConn.Close()
		new_quic_stream.CancelRead(0)
		new_quic_stream.Close()
		return
	}

	// DNS 等短会话需要空闲超时，否则 gonet 端点 + QUIC 流会一直占着。
	proxy.RelayIdle(originConn, new_quic_stream, proxy.UDPIdleTimeout)
}

func NewUdpForwarder(s *stack.Stack) *udp.Forwarder {
	return udp.NewForwarder(s, func(r *udp.ForwarderRequest) bool {
		var (
			wq waiter.Queue
			id = r.ID()
		)

		stun_quic_conn := currentTunnel()
		if stun_quic_conn == nil {
			return false
		}

		// CreateEndpoint 必须在回调内完成，后续包才能 demux 到该端点。
		ep, err := r.CreateEndpoint(&wq)
		if err != nil {
			log.Printf("forward udp request: %s:%d->%s:%d: %s", id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort, err)
			return false
		}
		setUdpSocketOptions(ep)

		conn := &udpConn{
			UDPConn: gonet.NewUDPConn(&wq, ep),
			id:      id,
		}
		// UDP Forwarder 回调跑在 DeliverTransportPacket 上（不像 TCP 会 go handler）。
		// OpenStreamSync 最长 10s，同步调用会卡住整栈收包。
		go ForwardUdpConn(conn, stun_quic_conn)
		return true
	})
}
