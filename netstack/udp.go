package netstack

import (
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	// gVisor UDP 默认仅 32KB，突发 DNS/音视频容易丢。256KB 仍远小于 TCP 上限。
	udpSendBufferSize    = 256 * 1024
	udpReceiveBufferSize = 256 * 1024
)

func setUdpSocketOptions(ep tcpip.Endpoint) {
	ep.SocketOptions().SetSendBufferSize(int64(udpSendBufferSize), true)
	ep.SocketOptions().SetReceiveBufferSize(int64(udpReceiveBufferSize), true)
}

type udpConn struct {
	*gonet.UDPConn
	id stack.TransportEndpointID
}

func (c *udpConn) ID() *stack.TransportEndpointID {
	return &c.id
}
