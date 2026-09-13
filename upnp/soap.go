package upnp

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

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
