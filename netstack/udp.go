package netstack

import (
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

// udpConn 实现了UDP连接接口
type udpConn struct {
	*gonet.UDPConn
	id stack.TransportEndpointID
}

// ID 返回连接的传输端点ID
func (c *udpConn) ID() *stack.TransportEndpointID {
	return &c.id
}
