//go:build windows

package netstack

import (
	"crypto/tls"
	"fmt"
	"gogo"
	"goodlink/winipcfg"
	"io"
	"net/http"
	"net/netip"
	"time"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

func SetWinTunIP(wintunEP *Device, ip string, mask int) error {
	var ipf netip.Prefix
	var err error

	link := winipcfg.LUID((*wintunEP).(*TUN).GetNt().LUID())

	// 将IP地址和掩码组合为CIDR格式（如192.168.1.1/24）
	if ipf, err = netip.ParsePrefix(fmt.Sprintf("%s/%d", ip, mask)); err != nil {
		return err
	}

	if err = link.SetIPAddresses([]netip.Prefix{ipf}); err != nil {
		return err
	}

	routeData := &winipcfg.RouteData{
		Destination: ipf,                     // 目标网络（CIDR格式）
		NextHop:     netip.MustParseAddr(ip), // 下一跳地址（本机IP）
		Metric:      0,                       // 路由优先级（数值越小优先级越高）
	}

	if err = link.SetRoutes([]*winipcfg.RouteData{
		routeData,
	}); err != nil {
		return err
	}

	return nil
}

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

func InitWintunDll() error {
	if gogo.Utils().FileExist("wintun.dll") {
		return nil
	}

	var res []byte
	var err error
	var resp *http.Response

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // 跳过证书验证
			},
		},
		Timeout: 3 * time.Second,
	}
	if resp, err = client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll"); err != nil {
		return err
	}
	defer resp.Body.Close()

	if res, err = io.ReadAll(resp.Body); err != nil {
		return err
	}

	gogo.Utils().FileAppend("wintun.dll", res)

	return nil
}

const (
	tunIP = "192.17.19.1"
)

func Start() error {
	InitWintunDll()

	if init_stack_suss {
		return nil
	}

	netstack_stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol},
	})

	wintunEP, err := Open("GoodLink", 1490) //因为要加自定义头，防止超出1500
	if err != nil {
		return fmt.Errorf("请管理员权限运行")
	}

	SetWinTunIP(&wintunEP, tunIP, 32)

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
	return tunIP
}
