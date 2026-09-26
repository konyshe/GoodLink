package upnp

import (
	"encoding/xml"
	"strconv"
	"strings"
)

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

func (this *GetPortMappingEntry) Send(index int) (*PortMappingEntry, bool) {
	if this.upnp == nil {
		return nil, false
	}
	body, status, err := this.upnp.soapPost("GetGenericPortMappingEntry", []Node{
		{Name: "NewPortMappingIndex", Content: strconv.Itoa(index)},
	})
	if err != nil || status != 200 {
		return nil, false
	}

	// 不少路由器对越界的 index 仍返回 HTTP 200(报文为空或 SOAP fault),
	// 此时必须判定为枚举结束, 否则会一路跑到 maxPortMappingIndex
	entry := parsePortMappingEntry(string(body))

	// 端口不在合法范围内说明响应无效或解析失败, 继续枚举没有意义,
	// 同时也避免把 0 端口当成有效映射加入待删除列表
	if entry.ExternalPort < 1 || entry.ExternalPort > 65535 {
		return nil, false
	}

	return entry, true
}

func parsePortMappingEntry(resultStr string) *PortMappingEntry {
	entry := &PortMappingEntry{}
	decoder := xml.NewDecoder(strings.NewReader(resultStr))

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
				if v, convErr := strconv.Atoi(content); convErr == nil {
					entry.ExternalPort = v
				}
			case "NewProtocol":
				entry.Protocol = content
			case "NewInternalPort":
				if v, convErr := strconv.Atoi(content); convErr == nil {
					entry.InternalPort = v
				}
			case "NewInternalClient":
				entry.InternalClient = content
			case "NewPortMappingDescription":
				entry.Description = content
			case "NewEnabled":
				entry.Enabled = content == "1" || strings.EqualFold(content, "true")
			}
		}
	}
	return entry
}
