package netstack

import (
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	// defaultWndSize 如果设置为零，则使用默认的接收窗口缓冲区大小
	defaultWndSize = 64 * 1024

	// maxConnAttempts 指定最大并发TCP连接尝试数
	maxConnAttempts = 2 << 12

	// 优化的缓冲区大小配置
	minSendBufferSize = 64 * 1024
	maxSendBufferSize = 4 * 1024 * 1024
	defaultSendBufferSize = 256 * 1024

	minReceiveBufferSize = 64 * 1024
	maxReceiveBufferSize = 8 * 1024 * 1024
	defaultReceiveBufferSize = 512 * 1024

	// tcpKeepaliveCount 在放弃并关闭连接之前发送的最大TCP保活探测次数
	tcpKeepaliveCount = 6

	// tcpKeepaliveIdle 减少保活空闲时间以更快检测断开连接
	tcpKeepaliveIdle = 30 * time.Second

	// tcpKeepaliveInterval 减少探测间隔
	tcpKeepaliveInterval = 10 * time.Second
)

// setSocketOptions 设置TCP套接字选项
func setSocketOptions(s *stack.Stack, ep tcpip.Endpoint) tcpip.Error {
	{ /* TCP保活选项 */
		ep.SocketOptions().SetKeepAlive(true)

		// 设置保活空闲时间
		idle := tcpip.KeepaliveIdleOption(tcpKeepaliveIdle)
		if err := ep.SetSockOpt(&idle); err != nil {
			return err
		}

		// 设置保活探测间隔
		interval := tcpip.KeepaliveIntervalOption(tcpKeepaliveInterval)
		if err := ep.SetSockOpt(&interval); err != nil {
			return err
		}

		// 设置保活探测次数
		if err := ep.SetSockOptInt(tcpip.KeepaliveCountOption, tcpKeepaliveCount); err != nil {
			return err
		}
	}
	{ /* TCP接收/发送缓冲区大小 */
		// 设置优化的发送缓冲区大小
		ep.SocketOptions().SetSendBufferSize(int64(defaultSendBufferSize), false)

		// 设置优化的接收缓冲区大小
		ep.SocketOptions().SetReceiveBufferSize(int64(defaultReceiveBufferSize), false)
	}
	{ /* TCP性能优化选项 */
		// 启用延迟ACK以减少网络流量
		ep.SocketOptions().SetDelayOption(true)

		// 启用重用地址和端口
		ep.SocketOptions().SetReuseAddress(true)
		ep.SocketOptions().SetReusePort(true)
	}
	return nil
}

// tcpConn 实现了TCP连接接口
type TcpConn struct {
	*gonet.TCPConn
	id stack.TransportEndpointID
}

// ID 返回连接的传输端点ID
func (c *TcpConn) ID() *stack.TransportEndpointID {
	return &c.id
}
