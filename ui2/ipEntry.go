package ui2

import (
	"fyne.io/fyne/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

type ipEntry struct {
	widget.Entry
}

func (n *ipEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}

func NewIpEntry() *ipEntry {
	e := &ipEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(value string) error {
		return nil
	}
	e.SetPlaceHolder("例如: 127.0.0.1")
	return e
}
