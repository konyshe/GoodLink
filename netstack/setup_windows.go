//go:build windows

package netstack

import (
	"fmt"
	"goodlink/winipcfg"
	"log"
	"net/netip"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

func SetTunIP(wintunEP *Device, ip string, mask int) error {
	tunDev := (*wintunEP).(*TUN)
	link := winipcfg.LUID(tunDev.GetNt().LUID())
	luid := uint64(link)

	ifacePrefix, err := tunInterfacePrefixFrom(ip)
	if err != nil {
		return err
	}
	destPrefix, err := netip.ParsePrefix(fmt.Sprintf("%s/%d", ip, mask))
	if err != nil {
		return err
	}

	// 1. 先设路由
	routeData := &winipcfg.RouteData{
		Destination: destPrefix,
		NextHop:     netip.IPv4Unspecified(),
		Metric:      0,
	}
	if err = link.SetRoutes([]*winipcfg.RouteData{routeData}); err != nil {
		return fmt.Errorf("set route %s via Goodlink: %w", destPrefix, err)
	}

	// 2. 网卡主 IP
	if err = link.SetIPAddresses([]netip.Prefix{ifacePrefix}); err != nil {
		return fmt.Errorf("set interface IP %s: %w", ifacePrefix, err)
	}

	// 3. IP 接口属性
	ipif, err := link.IPInterface(windows.AF_INET)
	if err != nil {
		return fmt.Errorf("get IP interface: %w", err)
	}
	ipif.RouterDiscoveryBehavior = winipcfg.RouterDiscoveryDisabled
	ipif.DadTransmits = 0
	ipif.ManagedAddressConfigurationSupported = false
	ipif.OtherStatefulConfigurationSupported = false
	ipif.UseAutomaticMetric = false
	ipif.Metric = 0
	ipif.WeakHostReceive = true
	ipif.WeakHostSend = true
	if tunDev.mtu > 0 {
		ipif.NLMTU = tunDev.mtu
	}
	if err = ipif.Set(); err != nil {
		return fmt.Errorf("set IP interface: %w", err)
	}

	if ifRow, err := link.Interface(); err == nil {
		log.Printf("TUN 网卡状态: AdminStatus=%d OperStatus=%d", ifRow.AdminStatus, ifRow.OperStatus)
	}

	log.Printf("TUN 配置: LUID=%d, 网卡 IP=%s, 路由=%s on-link (NextHop=0.0.0.0)", luid, ifacePrefix, destPrefix)
	return nil
}

func SetTunIP2(wintunEP *Device, ip string, mask int) error {
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

// CleanupOldAdapter 清理之前可能残留的虚拟网卡（仅关闭本适配器，不卸载驱动）
func CleanupOldAdapter(name string) {
	if err := InitWintunDll(); err != nil {
		return
	}
	adapter, err := wintun.OpenAdapter(name)
	if err != nil {
		return
	}

	log.Printf("发现残留的虚拟网卡 %s，正在清理...", name)
	adapter.Close()
	log.Printf("虚拟网卡 %s 已清理", name)
}

const wintunDllName = "wintun.dll"

func wintunDllPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), wintunDllName), nil
}

// InitWintunDll 确认 exe 同目录存在 wintun.dll，并从该目录加载。不从网络下载。
func InitWintunDll() error {
	p, err := wintunDllPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err != nil {
		return fmt.Errorf("请将 wintun.dll 放在程序同目录: %s", p)
	}
	if err := windows.SetDllDirectory(filepath.Dir(p)); err != nil {
		return fmt.Errorf("SetDllDirectory: %w", err)
	}
	return nil
}
