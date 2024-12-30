//go:build windows

package ui2

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

func NewPortEntry(port string) *portEntry {
	content := "范围: 1024-65535"
	e := &portEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(port string) error {
		if n, err := strconv.Atoi(port); err == nil && n >= 1024 && n <= 65535 {
			return nil
		}
		return errors.New("请输入正确的端口号")
	}
	e.SetPlaceHolder(content)
	e.SetText(port)
	return e
}
