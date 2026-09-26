package upnp

import "strconv"

type AddPortMapping struct {
	upnp *Upnp
}

func (this *AddPortMapping) Send(localPort, remotePort int, protocol string) bool {
	if this.upnp == nil {
		return false
	}

	// 协议统一归一化后再下发, 并拒绝非法端口:
	// 调用方散落在各处(部分传小写 udp), 这里的校验可以一次性覆盖全部入口
	proto := normalizeProtocol(protocol)
	if !validPort(localPort) || !validPort(remotePort) || proto == "" {
		return false
	}

	_, status, err := this.upnp.soapPost("AddPortMapping", []Node{
		{Name: "NewExternalPort", Content: strconv.Itoa(remotePort)},
		{Name: "NewInternalPort", Content: strconv.Itoa(localPort)},
		{Name: "NewProtocol", Content: proto},
		{Name: "NewEnabled", Content: "1"},
		{Name: "NewInternalClient", Content: this.upnp.LocalHost},
		{Name: "NewLeaseDuration", Content: "0"},
		{Name: "NewPortMappingDescription", Content: this.upnp.mappingDescription()},
		{Name: "NewRemoteHost"},
	})
	return err == nil && status == 200
}
