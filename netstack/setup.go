package netstack

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"goodlink/tls2"
	"goodlink/utils"
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

	// healthCheckRetryInterval 是重建失败后的重试间隔。
	// 失败时必须继续尝试: 原实现失败后每轮都只是 sleep, 网卡异常后永久无法自愈
	healthCheckRetryInterval = 5 * time.Second

	nicID = tcpip.NICID(1)

	// 内层 TUN 默认 1500 时 gVisor 通告 MSS≈1460，外层还要加 QUIC/UDP/IP，
	// 一个内层 TCP 段会被拆成多个 QUIC 包。1280 让内层段通常能进一个 QUIC 包。
	TunMTU = 1280
)

// NetStackName / NetStackIP 由 Start 写入, 一个进程一张网卡。
var (
	NetStackName = "Goodlink"
	NetStackIP   = "192.17.19.1"
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
	iface := netip.AddrFrom4([4]byte{b[0], b[1], b[2], 2})
	if iface == dest {
		return netip.Prefix{}, fmt.Errorf("tun interface IP equals dest IP %s", destIP)
	}
	return netip.PrefixFrom(iface, 24), nil
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

	// 隧道连接与协议栈 handler 解耦：gVisor 规定 SetTransportProtocolHandler
	// 只能在初始化时调用，运行中替换不安全。重连只改这个指针。
	tunnelMu sync.RWMutex
	tunnel   tls2.Conn
)

var (
	// healthStopCh 让健康检查循环能够退出, 避免重建失败后 goroutine 永久悬挂
	healthStopCh   = make(chan struct{})
	healthStopOnce sync.Once
)

// StopHealthCheck 停止健康检查循环
func StopHealthCheck() {
	healthStopOnce.Do(func() {
		close(healthStopCh)
	})
}

// cleanupStackResources 回收 initStack 过程中已创建的资源。
// Open / CreateNIC 成功但后续步骤失败时, 不清理会残留 TUN 网卡与栈对象,
// 重连重试若干次后会耗尽 fd 与网卡资源。可重复调用。
func cleanupStackResources() {
	if currentDevice != nil {
		currentDevice.Close()
		currentDevice = nil
	}
	if netstack_stack != nil {
		netstack_stack.RemoveNIC(nicID)
		netstack_stack.Close()
		netstack_stack = nil
	}
}

func setTunnel(c tls2.Conn) {
	tunnelMu.Lock()
	tunnel = c
	tunnelMu.Unlock()
}

func currentTunnel() tls2.Conn {
	tunnelMu.RLock()
	defer tunnelMu.RUnlock()
	return tunnel
}

func initStack() error {
	CleanupOldAdapter(NetStackName)

	netstack_stack = stack.New(stack.Options{
		NetworkProtocols: []stack.NetworkProtocolFactory{
			ipv4.NewProtocol,
		},
		TransportProtocols: []stack.TransportProtocolFactory{
			// Cubic 在 WAN/高 BDP 上比默认 Reno 爬升更快；内层 TCP 最终要跑在 QUIC 上。
			tcp.NewProtocolCUBIC,
			udp.NewProtocol,
			icmp.NewProtocol4,
		},
	})

	// 协议栈级默认也可能开 Nagle；这里关掉，避免新 endpoint 继承。
	delayOff := tcpip.TCPDelayEnabled(false)
	if err := netstack_stack.SetTransportProtocolOption(tcp.ProtocolNumber, &delayOff); err != nil {
		log.Printf("[netstack] 关闭协议栈级 Nagle 失败: %s", err)
	}

	dev, err := Open(NetStackName, TunMTU)
	if err != nil {
		if strings.Contains(err.Error(), "wintun.dll") {
			return err
		}
		return fmt.Errorf("%s", utils.NeedAdminRestartMsg)
	}
	currentDevice = dev

	// 配置失败意味着这张网卡不可用, 必须回收后返回, 否则会留下"起来了但不通"的网卡
	if err := SetTunIP(&dev, NetStackIP, 32); err != nil {
		log.Printf("[netstack] 设置网卡IP失败: %v", err)
		cleanupStackResources()
		return fmt.Errorf("设置网卡IP: %v", err)
	}

	if err := netstack_stack.CreateNIC(nicID, dev); err != nil {
		cleanupStackResources()
		return fmt.Errorf("设备注册: %v", err)
	}

	remoteIP := net.ParseIP(NetStackIP).To4()
	if remoteIP == nil {
		cleanupStackResources()
		return fmt.Errorf("无效的网卡IP: %s", NetStackIP)
	}
	protoAddr := tcpip.ProtocolAddress{
		Protocol: ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.AddressWithPrefix{
			Address:   tcpip.AddrFromSlice(remoteIP),
			PrefixLen: 32,
		},
	}
	if err := netstack_stack.AddProtocolAddress(nicID, protoAddr, stack.AddressProperties{}); err != nil {
		cleanupStackResources()
		return fmt.Errorf("绑定IP地址: %v", err)
	}

	if err := setNetStack(netstack_stack, nicID); err != nil {
		cleanupStackResources()
		return err
	}

	// 只在建栈时安装一次 Forwarder；之后 SetForWarder 只更新 currentTunnel。
	netstack_stack.SetTransportProtocolHandler(tcp.ProtocolNumber, NewTcpForwarder(netstack_stack).HandlePacket)
	netstack_stack.SetTransportProtocolHandler(udp.ProtocolNumber, NewUdpForwarder(netstack_stack).HandlePacket)

	return nil
}

func Start(adapterName, destIP string) error {
	mu.Lock()
	defer mu.Unlock()

	if adapterName != "" {
		NetStackName = adapterName
	}
	if destIP != "" {
		NetStackIP = destIP
	}

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
	failCount := 0

	for {
		interval := healthCheckInterval
		if failCount > 0 {
			interval = healthCheckRetryInterval
		}

		select {
		case <-healthStopCh:
			log.Printf("[netstack] 健康检查已停止")
			return
		case <-time.After(interval):
		}

		mu.Lock()
		healthy := false
		switch {
		case netstack_stack == nil:
			// 协议栈缺失(首次失败或上次重建失败)时同样要继续尝试,
			// 原实现这里直接 continue, 导致失败后永远不再重建
			log.Printf("[netstack] 健康检查: 协议栈未就绪, 尝试重建...")
			rebuildStack()
			healthy = netstack_stack != nil
		default:
			nicInfo, ok := netstack_stack.NICInfo()[nicID]
			if !ok || nicInfo.Flags.Running == false {
				log.Printf("[netstack] 健康检查: NIC %d 异常 (exists=%v), 尝试重建协议栈...", nicID, ok)
				rebuildStack()
				healthy = netstack_stack != nil
			} else {
				healthy = true
			}
		}
		mu.Unlock()

		if healthy {
			failCount = 0
		} else {
			failCount++
			log.Printf("[netstack] 重建协议栈失败, 将在%v后重试(第%d次)", healthCheckRetryInterval, failCount)
		}
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
		cleanupStackResources()
		return
	}

	initStackDone = true
	log.Printf("[netstack] 协议栈重建成功")
}

func SetForWarder(stun_quic_conn tls2.Conn) {
	setTunnel(stun_quic_conn)
}
