package upnp

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// validServiceType 校验 UPnP 服务类型。
// serviceType 来自 SSDP 响应里的 ST 字段(网络可控), 会被写进 xmlns:m 属性值;
// 不做校验时攻击者可用引号闭合属性, 构造出任意 SOAP 请求内容。
func validServiceType(serviceType string) bool {
	s := strings.TrimSpace(serviceType)
	if !strings.HasPrefix(s, "urn:") || len(s) > 256 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == ':' || r == '_':
		default:
			return false
		}
	}
	return true
}

func buildSOAPEnvelope(serviceType, action string, args []Node) string {
	body := Node{
		Name: "SOAP-ENV:Envelope",
		Attr: map[string]string{
			"xmlns:SOAP-ENV":         `"http://schemas.xmlsoap.org/soap/envelope/"`,
			"SOAP-ENV:encodingStyle": `"http://schemas.xmlsoap.org/soap/encoding/"`,
		},
	}
	childOne := Node{Name: "SOAP-ENV:Body"}
	childTwo := Node{
		Name: "m:" + action,
		Attr: map[string]string{"xmlns:m": `"` + serviceType + `"`},
	}
	for _, arg := range args {
		childTwo.AddChild(arg)
	}
	childOne.AddChild(childTwo)
	body.AddChild(childOne)
	return body.BuildXML()
}

func (u *Upnp) client() *http.Client {
	if u != nil && u.http_client != nil {
		return u.http_client
	}
	return &http.Client{Timeout: httpClientTimeout}
}

func (u *Upnp) soapPost(action string, args []Node) ([]byte, int, error) {
	if u == nil || u.Gateway == nil || u.Gateway.ServiceType == "" || u.CtrlUrl == "" {
		return nil, 0, errors.New("upnp: 未初始化")
	}
	if !validServiceType(u.Gateway.ServiceType) {
		return nil, 0, errors.New("upnp: 服务类型无效")
	}

	bodyStr := buildSOAPEnvelope(u.Gateway.ServiceType, action, args)
	req, err := http.NewRequest("POST", "http://"+u.Gateway.Host+u.CtrlUrl, strings.NewReader(bodyStr))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("SOAPAction", `"`+u.Gateway.ServiceType+`#`+action+`"`)
	req.Header.Set("Content-Type", "text/xml")
	req.Header.Set("Content-Length", strconv.Itoa(len(bodyStr)))

	resp, err := u.client().Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	result, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return result, resp.StatusCode, nil
}
