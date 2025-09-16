//go:build windows

package netstack

import (
	"crypto/tls"
	"fmt"
	"go2"
	"goodlink/winipcfg"
	"io"
	"net/http"
	"net/netip"
	"os"
	"time"
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
	if resp, err = client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/wintun.dll"); err != nil {
		return err
	}
	defer resp.Body.Close()

	if res, err = io.ReadAll(resp.Body); err != nil {
		return err
	}

	go2.Utils().FileAppend("wintun.dll", res)

	return nil
}
