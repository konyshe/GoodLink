package ui2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type LocalUI struct {
	localIP         string
	box_local_port  *portEntry
	radio1          *widget.RadioGroup
	radio_conn_type *widget.RadioGroup
	connType        int
}

func (c *LocalUI) GetConnType() int {
	return c.connType
}

func (c *LocalUI) Disable() {
	c.radio1.Disable()
	c.box_local_port.Disable()
	c.radio_conn_type.Disable()
}

func (c *LocalUI) Enable() {
	c.radio1.Enable()
	c.box_local_port.Enable()
	c.radio_conn_type.Enable()
}

func (c *LocalUI) GetRemoteAddr() (string, error) {
	return c.localIP + ":" + c.box_local_port.Text, c.box_local_port.Validate()
}

func (c *LocalUI) GetContainer() *fyne.Container {
	return container.NewVBox(
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接方式: "), c.radio_conn_type),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("访问权限: "), c.radio1),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("访问端口: "), c.box_local_port),
	)
}

func NewLocalUI(myWindow *fyne.Window) *LocalUI {
	c := &LocalUI{
		box_local_port:  NewPortEntry(),
		radio_conn_type: widget.NewRadioGroup([]string{"主动连接", "被动连接"}, nil),
		radio1:          widget.NewRadioGroup([]string{"允许本机", "允许局域网"}, nil),
	}

	c.radio1.OnChanged = func(value string) {
		switch value {
		case "允许本机":
			c.localIP = "127.0.0.1"
		case "允许局域网":
			c.localIP = "0.0.0.0"
		default:
			c.radio1.SetSelected("允许本机")
		}
	}
	c.radio1.SetSelected("允许本机")
	c.radio1.Horizontal = true

	c.radio_conn_type.OnChanged = func(value string) {
		switch value {
		case "主动连接":
			c.connType = 1
		case "被动连接":
			c.connType = 0
		default:
			c.radio_conn_type.SetSelected("被动连接")
		}
	}
	c.radio_conn_type.SetSelected("被动连接")
	c.radio_conn_type.Horizontal = true

	return c
}
