package netstack

import (
	"fmt"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

func setNetStack(s *stack.Stack, nicID tcpip.NICID) error {
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         nicID,
		},
	})

	if err := netstack_stack.SetPromiscuousMode(nicID, true); err != nil {
		return fmt.Errorf("promisc: %s", err)
	}

	if err := netstack_stack.SetSpoofing(nicID, true); err != nil {
		return fmt.Errorf("spoofing: %s", err)
	}

	return nil
}

var (
	init_stack_suss = false
	netstack_stack  *stack.Stack
)

func Start() error {
	if init_stack_suss {
		return nil
	}

	netstack_stack = stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
		},
	})

	wintunEP, err := Open(GetName(), 1490) //因为要加自定义头，防止超出1500
	if err != nil {
		return fmt.Errorf("请管理员权限运行")
	}

	SetTunIP(&wintunEP, GetRemoteIP(), 32)

	// 将TUN设备注册到协议栈中，使用NIC ID 1
	nicID := tcpip.NICID(1)
	if err := netstack_stack.CreateNIC(nicID, wintunEP); err != nil {
		return fmt.Errorf("设备注册: %v", err)
	}

	setNetStack(netstack_stack, nicID)

	init_stack_suss = true

	return nil
}

func SetForWarder(stun_quic_conn quic.Connection) {
	netstack_stack.SetTransportProtocolHandler(tcp.ProtocolNumber, NewTcpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
	netstack_stack.SetTransportProtocolHandler(udp.ProtocolNumber, NewUdpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
}

func GetRemoteIP() string {
	return "192.17.19.1"
}

func GetName() string {
	return "GoodLink"
}
