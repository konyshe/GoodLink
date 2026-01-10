//go:build windows

package netstack

import (
	"fmt"
	go2http "go2/http"
	"goodlink/winipcfg"
	"log"
	"net/netip"
	"os"
	"runtime"
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

const (
	// wintunDllURL wintun.dll 下载地址
	wintunDllURL = "https://gitee.com/konyshe/goodlink_conf/raw/master/wintun-0.14.1"
	// wintunDllName wintun.dll 文件名
	wintunDllName = "wintun.dll"
	// downloadTimeout 下载超时时间
	downloadTimeout = 30 * time.Second
	// maxRetries 最大重试次数
	maxRetries = 3
	// retryDelay 重试延迟时间
	retryDelay = 2 * time.Second
)

// InitWintunDll 初始化 wintun.dll，如果文件不存在则从网络下载
func InitWintunDll() error {
	// 检查文件是否已存在
	if _, err := os.Stat(wintunDllName); err == nil {
		log.Printf("wintun.dll 已存在，跳过下载")
		return nil
	}

	log.Printf("开始下载 wintun.dll...")

	url := fmt.Sprintf("%s/%s/%s", wintunDllURL, runtime.GOARCH, wintunDllName)

	/*
		// 带进度监控的下载
		err := go2http.DownloadWithProgress(wintunDllURL, wintunDllName,
		    func(downloaded, total int64) {
		        if total > 0 {
		            percent := float64(downloaded) / float64(total) * 100
		            log.Printf("下载进度: %.2f%% (%d/%d 字节)", percent, downloaded, total)
		        }
		    })

		// 自定义配置下载
		config := go2http.DefaultDownloadConfig("https://example.com/file.zip", "file.zip")
		config.MaxRetries = 5
		config.RetryDelay = 3 * time.Second
		config.Timeout = 60 * time.Second
		err := go2http.Download(config)
	*/

	return go2http.DownloadSimple(url, wintunDllName)
}
