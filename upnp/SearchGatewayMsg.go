package upnp

import (
	"context"
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

	buf := make([]byte, 2048)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
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
