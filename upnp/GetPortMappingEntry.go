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
	return parsePortMappingEntry(string(body)), true
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
