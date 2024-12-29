package ui2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type RemoteUI struct {
	remote_addr     string
	remote_addr_box *ipportEntry
	radio           *widget.RadioGroup
}

func (c *RemoteUI) Disable() {
	c.remote_addr_box.Disable()
	c.radio.Disable()
}

func (c *RemoteUI) Enable() {
	c.remote_addr_box.Enable()
	c.radio.Enable()
}

func (c *RemoteUI) GetContainer() *fyne.Container {
	return container.NewVBox(
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作模式: "), c.radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("转发地址: "), c.remote_addr_box),
	)
}

func NewRemoteUI(myWindow *fyne.Window) *RemoteUI {
	c := &RemoteUI{
		radio:           widget.NewRadioGroup([]string{"代理模式", "转发模式"}, nil),
		remote_addr_box: NewIpPortEntry("例如: 127.0.0.1:3389"),
	}

	c.radio.OnChanged = func(value string) {
		switch value {
		case "代理模式":
			c.remote_addr = c.remote_addr_box.Text
			c.remote_addr_box.SetPlaceHolder("不需要设置")
			c.remote_addr_box.SetText("")
			c.remote_addr_box.Disable()
		case "转发模式":
			c.remote_addr_box.Enable()
			c.remote_addr_box.SetPlaceHolder("例如: 127.0.0.1:3389")
			c.remote_addr_box.SetText(c.remote_addr)
		default:
			c.radio.SetSelected("代理模式")
		}
	}
	c.radio.SetSelected("代理模式")
	c.radio.Horizontal = true

	return c
}
