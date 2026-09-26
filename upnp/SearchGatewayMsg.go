package upnp

import (
	"context"
	"log"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
)

const ssdpMulticastAddr = "239.255.255.250:1900"

type Gateway struct {
	GatewayName   string //网关名称
	Host          string //网关ip和端口
	DeviceDescUrl string //网关设备描述路径
	Cache         string //cache
	ST            string
	USN           string
	ServiceType   string //提供upnp服务的服务类型
}

type SearchGateway struct {
	upnp *Upnp
}

func (this *SearchGateway) buildRequest(serviceType string) string {
	return "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: " + strconv.Itoa(ssdpMX) + "\r\n" +
		"ST: urn:schemas-upnp-org:service:" + serviceType + ":1\r\n\r\n"
}

func (this *SearchGateway) Send() bool {
	if this.upnp == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), ssdpSearchTimeout)
	defer cancel()

	results := make(chan string, 2)
	var wg sync.WaitGroup
	for _, serviceType := range []string{"WANPPPConnection", "WANIPConnection"} {
		wg.Add(1)
		go func(st string) {
			defer wg.Done()
			this.send(ctx, ssdpMulticastAddr, this.buildRequest(st), results)
		}(serviceType)
	}
	go func() {
		wg.Wait()
		close(results)
	}()

	result, ok := <-results
	if !ok || result == "" {
		this.upnp.Active = false
		return false
	}
	cancel()

	this.resolve(result)
	if this.upnp.Gateway == nil || this.upnp.Gateway.Host == "" || this.upnp.Gateway.ST == "" {
		this.upnp.Active = false
		return false
	}
	this.upnp.Gateway.ServiceType = this.upnp.Gateway.ST
	this.upnp.Active = true
	return true
}

func (this *SearchGateway) send(ctx context.Context, remoteAddr, searchMessage string, results chan<- string) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	remotAddr, err := net.ResolveUDPAddr("udp4", remoteAddr)
	if err != nil {
		return
	}

	localHost := ""
	if this.upnp != nil {
		localHost = this.upnp.LocalHost
	}
	locaAddr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(localHost, "0"))
	if err != nil {
		return
	}
	conn, err := net.ListenUDP("udp4", locaAddr)
	if err != nil {
		return
	}
	defer conn.Close()

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if _, err = conn.WriteToUDP([]byte(searchMessage), remotAddr); err != nil {
		return
	}

	buf := make([]byte, 4096)
	for {
		n, srcAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		if n == len(buf) {
			// UDP 数据报超过缓冲区会被内核静默截断, LOCATION 行可能只剩半截
			log.Printf("upnp: SSDP 响应疑似被截断(%d字节), 已忽略", n)
			continue
		}
		// SSDP 是组播发现: 同网段任何主机都能抢答成为"网关",
		// 不校验来源就等于信任任意主机提供的 LOCATION 与控制URL
		if !isLanGatewayAddr(srcAddr, localHost) {
			log.Printf("upnp: 忽略来源可疑的SSDP响应: %v", srcAddr)
			continue
		}
		result := string(buf[:n])
		if !strings.Contains(strings.ToUpper(result), "LOCATION:") {
			continue
		}
		select {
		case results <- result:
		case <-ctx.Done():
		}
		return
	}
}

// isLanGatewayAddr 判断 SSDP 响应来源是否可能是内网网关。
// 合法的 UPnP 网关一定位于内网, 来自公网地址或本机的响应都不是网关。
func isLanGatewayAddr(addr *net.UDPAddr, localHost string) bool {
	if addr == nil || addr.IP == nil {
		return false
	}
	ip := addr.IP.To4()
	if ip == nil {
		return false
	}
	if !ip.IsPrivate() && !ip.IsLinkLocalUnicast() {
		return false
	}
	// 排除本机发出的包(组播回环)
	if localHost != "" && ip.String() == localHost {
		return false
	}
	return true
}

func (this *SearchGateway) resolve(result string) {
	this.upnp.Gateway = parseGatewayFromSSDP(result)
}

func parseGatewayFromSSDP(result string) *Gateway {
	gw := &Gateway{}
	result = strings.ReplaceAll(result, "\r\n", "\n")
	for _, line := range strings.Split(result, "\n") {
		nameValues := strings.SplitN(line, ":", 2)
		if len(nameValues) < 2 {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(nameValues[0])) {
		case "ST":
			gw.ST = strings.TrimSpace(nameValues[1])
		case "CACHE-CONTROL":
			gw.Cache = strings.TrimSpace(nameValues[1])
		case "LOCATION":
			locationURL := strings.TrimSpace(nameValues[1])
			u, err := url.Parse(locationURL)
			if err == nil && u.Host != "" {
				gw.Host = u.Host
				path := u.RequestURI()
				if path == "" {
					path = "/"
				}
				gw.DeviceDescUrl = path
			}
		case "SERVER":
			gw.GatewayName = strings.TrimSpace(nameValues[1])
		case "USN":
			gw.USN = strings.TrimSpace(nameValues[1])
		}
	}
	return gw
}
