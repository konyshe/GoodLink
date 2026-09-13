package upnp

import (
	"encoding/xml"
	"strings"
)

type ExternalIPAddress struct {
	upnp *Upnp
}

func (this *ExternalIPAddress) Send() bool {
	if this.upnp == nil {
		return false
	}
	body, status, err := this.upnp.soapPost("GetExternalIPAddress", nil)
	if err != nil || status != 200 {
		return false
	}
	ip := parseExternalIP(string(body))
	if ip == "" {
		return false
	}
	this.upnp.GatewayOutsideIP = ip
	return true
}

func parseExternalIP(resultStr string) string {
	decoder := xml.NewDecoder(strings.NewReader(resultStr))
	found := false
	for t, err := decoder.Token(); err == nil; t, err = decoder.Token() {
		switch token := t.(type) {
		case xml.StartElement:
			if token.Name.Local == "NewExternalIPAddress" {
				found = true
			}
		case xml.EndElement:
			if token.Name.Local == "NewExternalIPAddress" {
				found = false
			}
		case xml.CharData:
			if found {
				ip := strings.TrimSpace(string(token))
				if ip != "" {
					return ip
				}
			}
		}
	}
	return ""
}
