package netstack

import (
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	// gVisor TCP MaxBufferSize 为 4MB；再大不会生效，还会加重 bufferbloat。
	// 第二参数必须 notify=true，否则只改了选项数字、窗口/发送队列仍是握手时的 1MB。
	defaultSendBufferSize    = 4 * 1024 * 1024
	defaultReceiveBufferSize = 4 * 1024 * 1024

	tcpKeepaliveCount    = 6
	tcpKeepaliveIdle     = 30 * time.Second
	tcpKeepaliveInterval = 10 * time.Second
)

func setSocketOptions(s *stack.Stack, ep tcpip.Endpoint) tcpip.Error {
	_ = s
	{
		ep.SocketOptions().SetKeepAlive(true)

		idle := tcpip.KeepaliveIdleOption(tcpKeepaliveIdle)
		if err := ep.SetSockOpt(&idle); err != nil {
			return err
		}

		interval := tcpip.KeepaliveIntervalOption(tcpKeepaliveInterval)
		if err := ep.SetSockOpt(&interval); err != nil {
			return err
		}

		if err := ep.SetSockOptInt(tcpip.KeepaliveCountOption, tcpKeepaliveCount); err != nil {
			return err
		}
	}
	{
		ep.SocketOptions().SetSendBufferSize(int64(defaultSendBufferSize), true)
		ep.SocketOptions().SetReceiveBufferSize(int64(defaultReceiveBufferSize), true)
	}
	{
		// SetDelayOption 是 TCP_NODELAY 的反义：true=开启 Nagle。
		// 转发/RDP/SSH 都是小包，开 Nagle 会把包攒到 MSS，延迟明显变差。
		ep.SocketOptions().SetDelayOption(false)
		// 尽快 ACK，避免与对端 Nagle 叠加成「延迟 ACK + Nagle」卡顿。
		ep.SocketOptions().SetQuickAck(true)

		ep.SocketOptions().SetReuseAddress(true)
		ep.SocketOptions().SetReusePort(true)
	}
	return nil
}

type TcpConn struct {
	*gonet.TCPConn
	id stack.TransportEndpointID
}

func (c *TcpConn) ID() *stack.TransportEndpointID {
	return &c.id
}
