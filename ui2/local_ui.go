package ui2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type LocalUI struct {
	local_ip string
	port_box *portEntry
	radio    *widget.RadioGroup
}

func (c *LocalUI) Disable() {
	c.port_box.Disable()
	c.radio.Disable()
}

func (c *LocalUI) Enable() {
	c.port_box.Enable()
	c.radio.Enable()
}

func (c *LocalUI) GetContainer() *fyne.Container {
	return container.NewVBox(
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("访问权限: "), c.radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("访问端口: "), c.port_box),
	)
}

func NewLocalUI(myWindow *fyne.Window) *LocalUI {
	c := &LocalUI{
		port_box: NewPortEntry(),
		radio:    widget.NewRadioGroup([]string{"只允许本机", "允许局域网"}, nil),
	}

	c.radio.OnChanged = func(value string) {
		switch value {
		case "只允许本机":
			c.local_ip = "127.0.0.1"
		case "允许局域网":
			c.local_ip = "0.0.0.0"
		default:
			c.radio.SetSelected("只允许本机")
		}
	}
	c.radio.SetSelected("只允许本机")
	c.radio.Horizontal = true

	return c
}
