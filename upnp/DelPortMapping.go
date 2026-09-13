package upnp

import "strconv"

type DelPortMapping struct {
	upnp *Upnp
}

func (this *DelPortMapping) Send(remotePort int, protocol string) bool {
	if this.upnp == nil {
		return false
	}
	_, status, err := this.upnp.soapPost("DeletePortMapping", []Node{
		{Name: "NewExternalPort", Content: strconv.Itoa(remotePort)},
		{Name: "NewProtocol", Content: protocol},
		{Name: "NewRemoteHost"},
	})
	return err == nil && status == 200
}
