package upnp

import (
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type DeviceDesc struct {
	upnp *Upnp
}

func (this *DeviceDesc) Send() bool {
	if this.upnp == nil || this.upnp.Gateway == nil || this.upnp.Gateway.Host == "" {
		return false
	}
	descPath := this.upnp.Gateway.DeviceDescUrl
	if descPath == "" {
		descPath = "/"
	} else if !strings.HasPrefix(descPath, "/") {
		descPath = "/" + descPath
	}

	req, err := http.NewRequest("GET", "http://"+this.upnp.Gateway.Host+descPath, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "goodlink")

	resp, err := this.upnp.client().Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	resultBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	this.resolve(string(resultBody))
	return this.upnp.CtrlUrl != ""
}

func (this *DeviceDesc) resolve(resultStr string) {
	if this.upnp == nil || this.upnp.Gateway == nil {
		return
	}
	base, err := url.Parse("http://" + this.upnp.Gateway.Host + this.upnp.Gateway.DeviceDescUrl)
	if err != nil {
		return
	}
	controlURL := parseControlURLFromDesc(resultStr, this.upnp.Gateway.ServiceType)
	if controlURL == "" {
		return
	}
	host, path := resolveControlURL(base, controlURL)
	if path == "" {
		return
	}
	// 控制URL必须与发现阶段的网关同主机: 设备描述里的控制URL是网络可控数据,
	// 允许改写 host 会把后续所有 SOAP 请求(含删除端口映射)发往攻击者主机
	if host != "" && host != this.upnp.Gateway.Host {
		log.Printf("upnp: 忽略指向其它主机的控制URL: %s", host)
		return
	}
	this.upnp.CtrlUrl = path
}

func parseControlURLFromDesc(resultStr, serviceType string) string {
	decoder := xml.NewDecoder(strings.NewReader(resultStr))
	lastLabel := ""
	isUpnpServer := false

	for t, err := decoder.Token(); err == nil; t, err = decoder.Token() {
		switch token := t.(type) {
		case xml.StartElement:
			if isUpnpServer {
				lastLabel = token.Name.Local
			}
		case xml.CharData:
			content := strings.TrimSpace(string(token))
			if content == "" {
				continue
			}
			if content == serviceType {
				isUpnpServer = true
				continue
			}
			if isUpnpServer && lastLabel == "controlURL" {
				return content
			}
		}
	}
	return ""
}

func resolveControlURL(base *url.URL, controlURL string) (host, path string) {
	if base == nil {
		return "", ""
	}
	ref, err := url.Parse(strings.TrimSpace(controlURL))
	if err != nil {
		return "", ""
	}
	resolved := base.ResolveReference(ref)
	path = resolved.EscapedPath()
	if path == "" {
		path = "/"
	}
	if resolved.RawQuery != "" {
		path += "?" + resolved.RawQuery
	}
	return resolved.Host, path
}
