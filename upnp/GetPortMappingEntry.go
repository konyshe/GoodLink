package upnp

import (
	"encoding/xml"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// PortMappingEntry 端口映射条目
type PortMappingEntry struct {
	ExternalPort   int
	Protocol       string
	InternalPort   int
	InternalClient string
	Description    string
	Enabled        bool
}

type GetPortMappingEntry struct {
	upnp *Upnp
}

// Send 发送请求获取指定索引的端口映射
// 返回端口映射条目和是否成功
// 当索引超出范围时返回 nil, false
func (this *GetPortMappingEntry) Send(index int) (*PortMappingEntry, bool) {
	request := this.buildRequest(index)
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, false
	}
	defer response.Body.Close()
	resultBody, _ := io.ReadAll(response.Body)
	if response.StatusCode == 200 {
		return this.resolve(string(resultBody)), true
	}
	return nil, false
}

func (this *GetPortMappingEntry) buildRequest(index int) *http.Request {
	// 请求头
	header := http.Header{}
	header.Set("Accept", "text/html, image/gif, image/jpeg, *; q=.2, */*; q=.2")
	header.Set("SOAPAction", `"`+this.upnp.Gateway.ServiceType+`#GetGenericPortMappingEntry"`)
	header.Set("Content-Type", "text/xml")
	header.Set("Connection", "Close")
	header.Set("Content-Length", "")

	// 请求体
	body := Node{Name: "SOAP-ENV:Envelope",
		Attr: map[string]string{"xmlns:SOAP-ENV": `"http://schemas.xmlsoap.org/soap/envelope/"`,
			"SOAP-ENV:encodingStyle": `"http://schemas.xmlsoap.org/soap/encoding/"`}}
	childOne := Node{Name: `SOAP-ENV:Body`}
	childTwo := Node{Name: `m:GetGenericPortMappingEntry`,
		Attr: map[string]string{"xmlns:m": `"` + this.upnp.Gateway.ServiceType + `"`}}

	// 添加索引参数
	childIndex := Node{Name: "NewPortMappingIndex", Content: strconv.Itoa(index)}
	childTwo.AddChild(childIndex)

	childOne.AddChild(childTwo)
	body.AddChild(childOne)
	bodyStr := body.BuildXML()

	// 请求
	request, _ := http.NewRequest("POST", "http://"+this.upnp.Gateway.Host+this.upnp.CtrlUrl,
		strings.NewReader(bodyStr))
	request.Header = header
	request.Header.Set("Content-Length", strconv.Itoa(len([]byte(bodyStr))))
	return request
}

func (this *GetPortMappingEntry) resolve(resultStr string) *PortMappingEntry {
	entry := &PortMappingEntry{}
	inputReader := strings.NewReader(resultStr)
	decoder := xml.NewDecoder(inputReader)

	var currentElement string
	for t, err := decoder.Token(); err == nil; t, err = decoder.Token() {
		switch token := t.(type) {
		case xml.StartElement:
			currentElement = token.Name.Local
		case xml.EndElement:
			currentElement = ""
		case xml.CharData:
			content := strings.TrimSpace(string(token))
			if content == "" {
				continue
			}
			switch currentElement {
			case "NewExternalPort":
				entry.ExternalPort, _ = strconv.Atoi(content)
			case "NewProtocol":
				entry.Protocol = content
			case "NewInternalPort":
				entry.InternalPort, _ = strconv.Atoi(content)
			case "NewInternalClient":
				entry.InternalClient = content
			case "NewPortMappingDescription":
				entry.Description = content
			case "NewEnabled":
				entry.Enabled = content == "1"
			}
		}
	}
	return entry
}
