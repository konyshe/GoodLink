package netstack

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

const (
	healthCheckInterval = 30 * time.Second
	nicID               = tcpip.NICID(1)
	NetStackName        = "Goodlink"
	NetStackIP          = "192.17.19.1"
)

func tunInterfacePrefixFrom(destIP string) (netip.Prefix, error) {
	dest, err := netip.ParseAddr(destIP)
	if err != nil {
		return netip.Prefix{}, err
	}
	if !dest.Is4() {
		return netip.Prefix{}, fmt.Errorf("tun interface IP requires IPv4, got %s", destIP)
	}
	b := dest.As4()
	iface := netip.AddrFrom4([4]byte{b[0], b[1], 0, 1})
	if iface == dest {
		return netip.Prefix{}, fmt.Errorf("tun interface IP equals dest IP %s", destIP)
	}
	return netip.PrefixFrom(iface, 32), nil
}

func setNetStack(s *stack.Stack, id tcpip.NICID) error {
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: header.IPv4EmptySubnet,
			NIC:         id,
		},
	})

	if err := s.SetPromiscuousMode(id, true); err != nil {
		return fmt.Errorf("promisc: %s", err)
	}

	if err := s.SetSpoofing(id, true); err != nil {
		return fmt.Errorf("spoofing: %s", err)
	}

	return nil
}

var (
	mu             sync.Mutex
	initStackDone  bool
	netstack_stack *stack.Stack
	currentDevice  Device
)

func initStack() error {
	CleanupOldAdapter(NetStackName)

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

	dev, err := Open(NetStackName, 0)
	if err != nil {
		return fmt.Errorf("请管理员权限运行")
	}
	currentDevice = dev

	SetTunIP(&dev, NetStackIP, 32)

	if err := netstack_stack.CreateNIC(nicID, dev); err != nil {
		return fmt.Errorf("设备注册: %v", err)
	}

	remoteIP := net.ParseIP(NetStackIP).To4()
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

	if err := setNetStack(netstack_stack, nicID); err != nil {
		return err
	}

	return nil
}

func Start() error {
	mu.Lock()
	defer mu.Unlock()

	if initStackDone {
		return nil
	}

	if err := initStack(); err != nil {
		return err
	}

	initStackDone = true
	go healthCheckLoop()

	return nil
}

// healthCheckLoop 定期检查协议栈 NIC 状态，如果发现异常则尝试重建
func healthCheckLoop() {
	for {
		time.Sleep(healthCheckInterval)

		mu.Lock()
		if !initStackDone || netstack_stack == nil {
			mu.Unlock()
			continue
		}

		nicInfo, ok := netstack_stack.NICInfo()[nicID]
		if !ok || nicInfo.Flags.Running == false {
			log.Printf("[netstack] 健康检查: NIC %d 异常 (exists=%v), 尝试重建协议栈...", nicID, ok)
			rebuildStack()
		}
		mu.Unlock()
	}
}

// rebuildStack 重建整个协议栈（调用方需持有 mu 锁）
func rebuildStack() {
	if currentDevice != nil {
		currentDevice.Close()
		currentDevice = nil
	}

	if netstack_stack != nil {
		netstack_stack.Close()
		netstack_stack = nil
	}

	if err := initStack(); err != nil {
		log.Printf("[netstack] 重建协议栈失败: %v", err)
		initStackDone = false
		return
	}

	log.Printf("[netstack] 协议栈重建成功")
}

func SetForWarder(stun_quic_conn *quic.Conn) {
	mu.Lock()
	defer mu.Unlock()

	if netstack_stack == nil {
		return
	}
	netstack_stack.SetTransportProtocolHandler(tcp.ProtocolNumber, NewTcpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
	netstack_stack.SetTransportProtocolHandler(udp.ProtocolNumber, NewUdpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
}
