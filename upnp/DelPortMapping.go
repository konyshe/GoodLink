package upnp

import "strconv"

type DelPortMapping struct {
	upnp *Upnp
}

func (this *DelPortMapping) Send(remotePort int, protocol string) bool {
	if this.upnp == nil {
		return false
	}

	// 与 AddPortMapping 保持一致的归一化与端口校验:
	// 小写协议名会被路由器拒绝(402), 端口为 0 会删除到错误的映射
	proto := normalizeProtocol(protocol)
	if !validPort(remotePort) || proto == "" {
		return false
	}

	_, status, err := this.upnp.soapPost("DeletePortMapping", []Node{
		{Name: "NewExternalPort", Content: strconv.Itoa(remotePort)},
		{Name: "NewProtocol", Content: proto},
		{Name: "NewRemoteHost"},
	})
	return err == nil && status == 200
}
