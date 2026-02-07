package netstack

import (
	"fmt"
	"net"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
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

	// 先清理之前可能残留的虚拟网卡
	CleanupOldAdapter(GetName())

	netstack_stack = stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			tcp.NewProtocol,
			udp.NewProtocol,
			icmp.NewProtocol4,
		},
	})

	wintunEP, err := Open(GetName(), 0)
	if err != nil {
		return fmt.Errorf("请管理员权限运行")
	}

	SetTunIP(&wintunEP, GetRemoteIP(), 32)

	// 将TUN设备注册到协议栈中，使用NIC ID 1
	nicID := tcpip.NICID(1)
	if err := netstack_stack.CreateNIC(nicID, wintunEP); err != nil {
		return fmt.Errorf("设备注册: %v", err)
	}

	// 将虚拟IP地址绑定到NIC，使协议栈能够响应ICMP Echo Request（ping）
	remoteIP := net.ParseIP(GetRemoteIP()).To4()
	protoAddr := tcpip.ProtocolAddress{
		Protocol: ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.AddressWithPrefix{
			Address:   tcpip.AddrFromSlice(remoteIP),
			PrefixLen: 32,
		},
	}
	if err := netstack_stack.AddProtocolAddress(nicID, protoAddr, stack.AddressProperties{}); err != nil {
		return fmt.Errorf("绑定IP地址: %v", err)
	}

	setNetStack(netstack_stack, nicID)

	init_stack_suss = true

	return nil
}

func SetForWarder(stun_quic_conn *quic.Conn) {
	netstack_stack.SetTransportProtocolHandler(tcp.ProtocolNumber, NewTcpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
	netstack_stack.SetTransportProtocolHandler(udp.ProtocolNumber, NewUdpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
}

func GetRemoteIP() string {
	return "192.17.19.1"
}

func GetName() string {
	return "GoodLink"
}
