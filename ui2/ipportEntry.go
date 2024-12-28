package ui2

import (
	"fyne.io/fyne/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

type ipportEntry struct {
	widget.Entry
}

func (n *ipportEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}

func NewIpPortEntry(content string) *ipportEntry {
	e := &ipportEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(value string) error {
		return nil
	}
	e.SetPlaceHolder(content)
	return e
}
