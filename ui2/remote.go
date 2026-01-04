//go:build windows

package ui2

import (
	"goodlink/config"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type RemoteUI struct {
	box_remote_ip   *ipEntry
	box_remote_port *portEntry
	radio           *widget.RadioGroup
}

func (c *RemoteUI) Disable() {
	c.box_remote_ip.Disable()
	c.box_remote_port.Disable()
	c.radio.Disable()
}

func (c *RemoteUI) Enable() {
	c.box_remote_ip.Enable()
	c.box_remote_port.Enable()
	c.radio.Enable()
}

func (c *RemoteUI) GetRemoteType() string {
	return c.radio.Selected
}

func (c *RemoteUI) GetRemoteIP() string {
	return c.box_remote_ip.Text
}

func (c *RemoteUI) GetRemotePort() string {
	return c.box_remote_port.Text
}

func (c *RemoteUI) GetRemoteAddr() (string, error) {
	if c.box_remote_ip.Validate() != nil {
		return "", c.box_remote_ip.Validate()
	}
	if c.box_remote_port.Validate() != nil {
		return "", c.box_remote_port.Validate()
	}
	return c.box_remote_ip.Text + ":" + c.box_remote_port.Text, nil
}

func (c *RemoteUI) GetContainer() *fyne.Container {
	// 当前返回空容器，保留接口以便未来扩展
	return container.NewVBox()
}

func NewRemoteUI(myWindow *fyne.Window, configInfo *config.ConfigInfo) *RemoteUI {
	c := &RemoteUI{
		radio:           widget.NewRadioGroup([]string{"代理模式", "转发模式"}, nil),
		box_remote_ip:   NewIpEntry(configInfo.RemoteIP),
		box_remote_port: NewPortEntry(configInfo.RemotePort),
	}

	c.radio.Horizontal = true
	c.radio.OnChanged = func(value string) {
		switch value {
		case "转发模式":
			c.box_remote_ip.SetText(configInfo.RemoteIP)
			c.box_remote_ip.ResetPlaceHolder()
			c.box_remote_ip.Enable()

			c.box_remote_port.SetText(configInfo.RemotePort)
			c.box_remote_port.ResetPlaceHolder()
			c.box_remote_port.Enable()
		default:
			if c.box_remote_ip.Validate() == nil {
				configInfo.RemoteIP = c.box_remote_ip.Text
			}
			c.box_remote_ip.SetText("")
			c.box_remote_ip.SetPlaceHolder("无需配置")
			c.box_remote_ip.Disable()

			if c.box_remote_port.Validate() == nil {
				configInfo.RemotePort = c.box_remote_port.Text
			}
			c.box_remote_port.SetText("")
			c.box_remote_port.SetPlaceHolder("无需配置")
			c.box_remote_port.Disable()
		}
	}

	switch configInfo.RemoteType {
	case "转发模式":
		c.radio.SetSelected(configInfo.RemoteType)
		c.box_remote_ip.SetText(configInfo.RemoteIP)
		c.box_remote_port.SetText(configInfo.RemotePort)
	default:
		c.radio.SetSelected("代理模式")
	}

	return c
}
