package upnp

import "strconv"

type AddPortMapping struct {
	upnp *Upnp
}

func (this *AddPortMapping) Send(localPort, remotePort int, protocol string) bool {
	if this.upnp == nil {
		return false
	}
	_, status, err := this.upnp.soapPost("AddPortMapping", []Node{
		{Name: "NewExternalPort", Content: strconv.Itoa(remotePort)},
		{Name: "NewInternalPort", Content: strconv.Itoa(localPort)},
		{Name: "NewProtocol", Content: protocol},
		{Name: "NewEnabled", Content: "1"},
		{Name: "NewInternalClient", Content: this.upnp.LocalHost},
		{Name: "NewLeaseDuration", Content: "0"},
		{Name: "NewPortMappingDescription", Content: "goodlink"},
		{Name: "NewRemoteHost"},
	})
	return err == nil && status == 200
}
