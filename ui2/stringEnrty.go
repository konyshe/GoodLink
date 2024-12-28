package ui2

import (
	"fyne.io/fyne/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

type StringEntry struct {
	widget.Entry
}

func (n *StringEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}

func NewStringEntry(content string) *StringEntry {
	e := &StringEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(value string) error {
		return nil
	}
	e.SetPlaceHolder(content)
	return e
}
