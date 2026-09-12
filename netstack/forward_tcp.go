package netstack

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"goodlink/proxy"
	"goodlink/tls2"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
	"log"
	"time"
)

const quicOpenStreamTimeout = 10 * time.Second

var LoopBackAddr = [4]byte{127, 0, 0, 1}

func ForwardTCPConn(originConn *TcpConn, stun_quic_conn tls2.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), quicOpenStreamTimeout)
	defer cancel()

	new_quic_stream, err := stun_quic_conn.OpenStreamSync(ctx)
	if err != nil {
		log.Printf("[netstack] TCP转发打开quic流失败 %s:%d: %v", originConn.ID().LocalAddress, originConn.ID().LocalPort, err)
		originConn.Close()
		// 开流失败只关这条连接。os.Exit 会干掉整个进程，引发重连风暴。
		return
	}

	ioBuf := go2pool.Malloc(proxy.HEAD_LEN)
	defer go2pool.Free(ioBuf)

	ioBuf[0] = 0x00
	copy(ioBuf[1:5], LoopBackAddr[:])
	binary.BigEndian.PutUint16(ioBuf[5:proxy.HEAD_LEN], originConn.ID().LocalPort)

	if _, err := new_quic_stream.Write(ioBuf[:proxy.HEAD_LEN]); err != nil {
		log.Println("写入头部失败", err)
		originConn.Close()
		new_quic_stream.CancelRead(0)
		new_quic_stream.Close()
		return
	}

	proxy.Relay(originConn, new_quic_stream)
}

func NewTcpForwarder(s *stack.Stack) *tcp.Forwarder {
	// rcvWnd 与套接字缓冲一致，避免 SYN-ACK 仍通告默认 1MB。
	return tcp.NewForwarder(s, defaultReceiveBufferSize, 2048, func(r *tcp.ForwarderRequest) {
		var (
			wq  waiter.Queue
			ep  tcpip.Endpoint
			err tcpip.Error
			id  = r.ID()
		)

		stun_quic_conn := currentTunnel()
		if stun_quic_conn == nil {
			r.Complete(true)
			return
		}

		ep, err = r.CreateEndpoint(&wq)
		if err != nil {
			log.Printf("forward tcp request: %s:%d->%s:%d: %s", id.RemoteAddress, id.RemotePort, id.LocalAddress, id.LocalPort, err)
			r.Complete(true)
			return
		}
		// 握手完成后立刻 Complete，释放 inFlight 名额；OpenStream 可能阻塞数秒。
		r.Complete(false)

		setSocketOptions(s, ep)

		conn := &TcpConn{
			TCPConn: gonet.NewTCPConn(&wq, ep),
			id:      id,
		}
		go ForwardTCPConn(conn, stun_quic_conn)
	})
}
