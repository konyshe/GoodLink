//go:build windows

package ui2

import (
	"errors"
	"regexp"

	"fyne.io/fyne/driver/mobile"
	"fyne.io/fyne/v2/widget"
)

var (
	// IPv4 验证正则表达式（编译一次，重复使用）
	ipv4Regex = regexp.MustCompile(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`)
)

type ipEntry struct {
	widget.Entry
}

func (n *ipEntry) Keyboard() mobile.KeyboardType {
	return mobile.NumberKeyboard
}

func (n *ipEntry) ResetPlaceHolder() {
	n.SetPlaceHolder("例如: 127.0.0.1")
}

func NewIpEntry(ip string) *ipEntry {
	e := &ipEntry{}
	e.ExtendBaseWidget(e)
	e.Validator = func(ip string) error {
		if ipv4Regex.MatchString(ip) {
			return nil
		}
		return errors.New("请输入正确的IP地址")
	}
	e.ResetPlaceHolder()
	e.SetText(ip)
	return e
}
