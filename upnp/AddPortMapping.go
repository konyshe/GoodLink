package upnp

import (
	// "log"
	// "fmt"

	"io"
	"net/http"
	"strconv"
	"strings"
)

type AddPortMapping struct {
	upnp        *Upnp
	http_client *http.Client
}

func (this *AddPortMapping) Send(localPort, remotePort int, protocol string) bool {
	request := this.buildRequest(localPort, remotePort, protocol)
	response, err := this.http_client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()

	resultBody, _ := io.ReadAll(response.Body)
	if response.StatusCode == 200 {
		this.resolve(string(resultBody))
		return true
	}
	return false
}

func (this *AddPortMapping) buildRequest(localPort, remotePort int, protocol string) *http.Request {
	//请求头
	header := http.Header{}
	header.Set("Accept", "text/html, image/gif, image/jpeg, *; q=.2, */*; q=.2")
	header.Set("SOAPAction", `"`+this.upnp.Gateway.ServiceType+`#AddPortMapping"`)
	header.Set("Content-Type", "text/xml")
	header.Set("Connection", "Close")
	header.Set("Content-Length", "")
	//请求体
	body := Node{Name: "SOAP-ENV:Envelope",
		Attr: map[string]string{"xmlns:SOAP-ENV": `"http://schemas.xmlsoap.org/soap/envelope/"`,
			"SOAP-ENV:encodingStyle": `"http://schemas.xmlsoap.org/soap/encoding/"`}}
	childOne := Node{Name: `SOAP-ENV:Body`}
	childTwo := Node{Name: `m:AddPortMapping`,
		Attr: map[string]string{"xmlns:m": `"` + this.upnp.Gateway.ServiceType + `"`}}

	childTwo.AddChild(Node{Name: "NewExternalPort", Content: strconv.Itoa(remotePort)})
	childTwo.AddChild(Node{Name: "NewInternalPort", Content: strconv.Itoa(localPort)})
	childTwo.AddChild(Node{Name: "NewProtocol", Content: protocol})
	childTwo.AddChild(Node{Name: "NewEnabled", Content: "1"})
	childTwo.AddChild(Node{Name: "NewInternalClient", Content: this.upnp.LocalHost})
	childTwo.AddChild(Node{Name: "NewLeaseDuration", Content: "0"})
	childTwo.AddChild(Node{Name: "NewPortMappingDescription", Content: "goodlink"})
	childTwo.AddChild(Node{Name: "NewRemoteHost"})

	childOne.AddChild(childTwo)
	body.AddChild(childOne)
	bodyStr := body.BuildXML()

	//请求
	request, _ := http.NewRequest("POST", "http://"+this.upnp.Gateway.Host+this.upnp.CtrlUrl,
		strings.NewReader(bodyStr))
	request.Header = header
	request.Header.Set("Content-Length", strconv.Itoa(len([]byte(bodyStr))))
	return request
}

func (this *AddPortMapping) resolve(resultStr string) {
}
