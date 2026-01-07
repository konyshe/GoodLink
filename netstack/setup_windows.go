//go:build windows

package netstack

import (
	"crypto/tls"
	"fmt"
	"go2"
	"goodlink/winipcfg"
	"io"
	"log"
	"net/http"
	"net/netip"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

func SetTunIP(wintunEP *Device, ip string, mask int) error {
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

// CleanupOldAdapter 清理之前可能残留的虚拟网卡
func CleanupOldAdapter(name string) {
	// 尝试打开并删除之前创建的适配器
	adapter, err := wintun.OpenAdapter(name)
	if err != nil {
		// 适配器不存在，无需清理
		return
	}

	log.Printf("发现残留的虚拟网卡 %s，正在清理...", name)

	// 强制关闭适配器
	adapter.Close()

	// 使用 Uninstall 方法尝试完全卸载
	err = wintun.Uninstall()
	if err != nil {
		log.Printf("清理虚拟网卡警告: %v", err)
	} else {
		log.Printf("虚拟网卡 %s 已清理", name)
	}
}

// DeleteAdapterByGUID 通过 GUID 删除适配器（备用方法）
func DeleteAdapterByGUID(guid *windows.GUID) error {
	adapter, err := wintun.OpenAdapter("")
	if err != nil {
		return err
	}
	adapter.Close()
	return nil
}

func InitWintunDll() error {
	if _, err := os.Stat("wintun.dll"); os.IsExist(err) {
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
	if resp, err = client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/wintun-0.14.1/amd64/wintun.dll"); err != nil {
		return err
	}
	defer resp.Body.Close()

	if res, err = io.ReadAll(resp.Body); err != nil {
		return err
	}

	go2.FileAppend("wintun.dll", res)

	return nil
}
