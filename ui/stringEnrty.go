package ui

import (
	"fyne.io/fyne/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

type stringEntry struct {
	widget.Entry
}

func (n *stringEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}

func NewStringEntry(content string) *stringEntry {
	e := &stringEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(value string) error {
		return nil
	}
	e.SetPlaceHolder(content)
	return e
}
