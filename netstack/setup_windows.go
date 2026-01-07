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

	// 创建 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // 跳过证书验证
			},
		},
		Timeout: downloadTimeout,
	}

	var lastErr error
	// 重试机制
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			log.Printf("第 %d 次尝试下载 wintun.dll...", attempt)
			time.Sleep(retryDelay)
		}

		// 执行下载
		data, err := downloadFile(client)
		if err != nil {
			lastErr = fmt.Errorf("下载失败: %w", err)
			continue
		}

		// 验证文件大小（wintun.dll 通常不会太小）
		if len(data) < 1024 {
			lastErr = fmt.Errorf("下载的文件大小异常: %d 字节", len(data))
			continue
		}

		// 写入文件
		if !go2.FileAppend(wintunDllName, data) {
			lastErr = fmt.Errorf("写入文件失败")
			continue
		}

		// 验证文件是否成功写入
		if info, err := os.Stat(wintunDllName); err != nil {
			lastErr = fmt.Errorf("验证文件失败: %w", err)
			continue
		} else if info.Size() != int64(len(data)) {
			lastErr = fmt.Errorf("文件大小不匹配: 期望 %d 字节，实际 %d 字节", len(data), info.Size())
			continue
		}

		log.Printf("wintun.dll 下载成功，文件大小: %d 字节", len(data))
		return nil
	}

	return fmt.Errorf("下载 wintun.dll 失败，已重试 %d 次: %w", maxRetries, lastErr)
}

// downloadFile 下载文件内容
func downloadFile(client *http.Client) ([]byte, error) {

	url := fmt.Sprintf("%s/%s/wintun.dll", wintunDllURL, runtime.GOARCH)

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码异常: %d %s", resp.StatusCode, resp.Status)
	}

	// 读取响应体
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	return data, nil
}
