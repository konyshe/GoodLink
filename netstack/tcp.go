package netstack

import (
	"time"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	// defaultWndSize 如果设置为零，则使用默认的接收窗口缓冲区大小
	defaultWndSize = 0

	// maxConnAttempts 指定最大并发TCP连接尝试数
	maxConnAttempts = 2 << 10

	// tcpKeepaliveCount 在放弃并关闭连接之前发送的最大TCP保活探测次数
	// 如果在另一端没有收到响应
	tcpKeepaliveCount = 9

	// tcpKeepaliveIdle 指定在发送第一个TCP保活数据包之前连接必须保持空闲的时间
	// 一旦达到这个时间，就使用tcpKeepaliveInterval选项
	tcpKeepaliveIdle = 60 * time.Second

	// tcpKeepaliveInterval 指定发送TCP保活数据包之间的间隔时间
	tcpKeepaliveInterval = 30 * time.Second
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
		// 设置发送缓冲区大小
		var ss tcpip.TCPSendBufferSizeRangeOption
		if err := s.TransportProtocolOption(header.TCPProtocolNumber, &ss); err == nil {
			ep.SocketOptions().SetSendBufferSize(int64(ss.Default), false)
		}

		// 设置接收缓冲区大小
		var rs tcpip.TCPReceiveBufferSizeRangeOption
		if err := s.TransportProtocolOption(header.TCPProtocolNumber, &rs); err == nil {
			ep.SocketOptions().SetReceiveBufferSize(int64(rs.Default), false)
		}
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
