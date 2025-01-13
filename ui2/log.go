//go:build windows

package ui2

import (
	"errors"
	"fmt"
	"goodlink/utils"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

var (
	m_log_label *LogLabel
	m_view_time widget.Label
)

type LogLabel struct {
	widget.Label
}

func SetLogLabel(content string) {
	if m_log_label != nil {
		m_log_label.SetText(content)
		m_log_label.TextStyle = fyne.TextStyle{Bold: true}
	}
}

func UILogPrintF(a ...any) {
	var content string

	switch len(a) {
	case 1:
		content = a[0].(string)
	default:
		content = fmt.Sprintf(a[0].(string), a[1:]...)
	}

	log.Println(content)

	if len(content) > 32 {
		content = content[:32]
	}
	SetLogLabel(content)

	m_view_time.SetText(time.Now().Format("2006/01/02 15:04:05"))
}

func UILogInit() {
	utils.Log().RegistInfo(func(content string) {
		UILogPrintF(content)
	})
	utils.Log().RegistDebug(func(content string) {
		UILogPrintF(content)
	})
	utils.Log().RegistError(func(content string) {
		UILogPrintF(content)
		fyne.LogError("error: ", errors.New(content))
	})
}

func NewLogLabel(content string) *LogLabel {
	m_log_label = &LogLabel{}
	m_log_label.ExtendBaseWidget(m_log_label)
	m_log_label.SetText(content)

	UILogInit()

	return m_log_label
}
