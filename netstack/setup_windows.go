//go:build windows

package netstack

import (
	"crypto/tls"
	"fmt"
	"gogo"
	"goodlink/winipcfg"
	"io"
	"net"
	"net/http"
	"net/netip"
	"time"

	"github.com/quic-go/quic-go"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
)

func SetWinTunIP(wintunEP *Device, ip string, mask int, ip2r string, mask2r int) error {
	var ipf netip.Prefix
	var err error

	// 将LUID转换为Windows网络接口标识
	// LUID: 本地唯一接口标识符（Local Unique Identifier）
	// 用于唯一标识网络接口，通常用于网络配置和管理
	link := winipcfg.LUID((*wintunEP).(*TUN).GetNt().LUID())

	// 将IP地址和掩码组合为CIDR格式（如192.168.1.1/24）
	if ipf, err = netip.ParsePrefix(fmt.Sprintf("%s/%d", ip, mask)); err != nil {
		return err
	}

	// 设置接口的主IP地址
	if err = link.SetIPAddresses([]netip.Prefix{ipf}); err != nil {
		return err
	}

	if ipf, err = netip.ParsePrefix(fmt.Sprintf("%s/%d", ip2r, mask2r)); err != nil {
		return err
	}

	// 配置路由规则参数
	routeData := &winipcfg.RouteData{
		Destination: ipf,                     // 目标网络（CIDR格式）
		NextHop:     netip.MustParseAddr(ip), // 下一跳地址（本机IP）
		Metric:      0,                       // 路由优先级（数值越小优先级越高）
	}

	// 应用路由配置到网络接口
	if err = link.SetRoutes([]*winipcfg.RouteData{
		routeData,
	}); err != nil {
		return err
	}

	return nil
}

func SetNetStackIP(s *stack.Stack, nicID tcpip.NICID, ip string, mask int, ip2r, mask2r string) error {
	// 设置IP地址
	// 配置网络接口的IPv4地址为192.168.3.3/24
	protocolAddr := tcpip.ProtocolAddress{
		Protocol: ipv4.ProtocolNumber,
		AddressWithPrefix: tcpip.AddressWithPrefix{
			Address:   tcpip.AddrFromSlice(net.ParseIP(ip).To4()), // 设置IP地址
			PrefixLen: mask,                                       // 设置子网掩码长度
		},
	}
	// 将IP地址添加到网络接口，设置为静态配置的主端点
	if err := s.AddProtocolAddress(nicID, protocolAddr, stack.AddressProperties{
		PEB:        stack.CanBePrimaryEndpoint, // 允许作为主端点
		ConfigType: stack.AddressConfigStatic,  // 使用静态配置
	}); err != nil {
		return fmt.Errorf("AddProtocolAddress failed: %v", err)
	}

	// 设置路由
	// 配置默认路由，将所有192.168.3.0/24网段的流量转发到该接口
	subnet, err := tcpip.NewSubnet(
		tcpip.AddrFromSlice(net.ParseIP(ip2r).To4()),   // 设置目标网段
		tcpip.MaskFromBytes(net.ParseIP(mask2r).To4()), // 设置子网掩码
	)
	if err != nil {
		return fmt.Errorf("NewSubnet failed: %v", err)
	}
	// 设置路由表，将指定网段的流量转发到NIC ID 1
	s.SetRouteTable([]tcpip.Route{
		{
			Destination: subnet, // 目标网段
			NIC:         nicID,  // 使用的网络接口
		},
	})

	// 设置网络接口为混杂模式
	// 允许接收所有网络数据包，用于网络数据包的转发
	s.SetPromiscuousMode(nicID, true)

	return nil
}

var (
	init_stack_suss = false
	netstack_stack  *stack.Stack
)

func InitWintunDll() error {
	if !gogo.Utils().FileExist("wintun.dll") {
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
	}

	return nil
}

// setupNetstack 初始化并配置网络栈
// 该函数负责创建协议栈、设置网络接口、配置IP地址和路由表
// 返回:
//   - *stack.Stack: 配置好的网络栈实例
//   - error: 初始化过程中的错误信息
func Start() error {
	InitWintunDll()

	if init_stack_suss {
		return nil
	}

	//Sudo()

	// 创建协议栈
	// 创建新的协议栈实例，配置网络层和传输层协议
	netstack_stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},  // 配置IPv4协议
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol}, // 配置TCP协议
	})

	// 使用Open函数创建TUN设备，设备名称为"GoodLink"，MTU为0表示使用系统默认值
	wintunEP, err := Open("GoodLink", 1400) //因为要加自定义的头，防止超出1500，造成不必要的性能损耗
	if err != nil {
		return fmt.Errorf("请管理员权限运行")
	}

	SetWinTunIP(&wintunEP, "192.17.19.1", 32, "192.17.19.1", 32)

	// 创建网络接口
	// 将TUN设备注册到协议栈中，使用NIC ID 1
	nicID := tcpip.NICID(1)
	if err := netstack_stack.CreateNIC(nicID, wintunEP); err != nil {
		return fmt.Errorf("设备注册: %v", err)
	}

	SetNetStackIP(netstack_stack, nicID, "192.17.19.1", 32, "192.17.19.0", "255.255.255.0")

	init_stack_suss = true

	return nil
}

func SetForWarder(stun_quic_conn quic.Connection) {
	// 设置TCP协议处理器
	netstack_stack.SetTransportProtocolHandler(tcp.ProtocolNumber, NewTcpForwarder(netstack_stack, stun_quic_conn).HandlePacket)
}

func GetRemoteIP() string {
	return "192.17.19.1"
}
