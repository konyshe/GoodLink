package ui

import (
	"errors"
	"strconv"

	"fyne.io/fyne/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

type portEntry struct {
	widget.Entry
}

func (n *portEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}

func NewPortEntry() *portEntry {
	content := "请输入1024-65535范围的数字"
	e := &portEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(value string) error {
		if n, err := strconv.Atoi(value); err == nil && n >= 1024 && n <= 65535 {
			return nil
		}
		return errors.New(content)
	}
	e.SetPlaceHolder(content)
	return e
}
