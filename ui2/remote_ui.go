package ui2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type RemoteUI struct {
	remote_ip2      string
	remote_port2    string
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
	return container.NewVBox(
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作模式: "), c.radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("转发目标地址: "), c.box_remote_ip),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("转发目标端口: "), c.box_remote_port),
	)
}

func NewRemoteUI(myWindow *fyne.Window) *RemoteUI {
	c := &RemoteUI{
		radio:           widget.NewRadioGroup([]string{"代理模式", "转发模式"}, nil),
		box_remote_ip:   NewIpEntry(),
		box_remote_port: NewPortEntry(),
	}

	c.radio.OnChanged = func(value string) {
		switch value {
		case "代理模式":
			c.box_remote_ip.Disable()
			c.remote_ip2 = c.box_remote_ip.Text
			c.box_remote_ip.SetText("不需要设置")

			c.box_remote_port.Disable()
			c.remote_port2 = c.box_remote_port.Text
			c.box_remote_port.SetText("不需要设置")
		case "转发模式":
			c.box_remote_ip.SetText(c.remote_ip2)
			c.box_remote_ip.Enable()

			c.box_remote_port.SetText(c.remote_port2)
			c.box_remote_port.Enable()
		default:
			c.radio.SetSelected("代理模式")
		}
	}
	c.radio.SetSelected("代理模式")
	c.radio.Horizontal = true

	return c
}
