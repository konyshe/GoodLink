package ui2

import (
	"fyne.io/fyne/v2/widget"
)

var (
	m_log_label *LogLabel
)

type LogLabel struct {
	widget.Label
}

func NewLogLabel(content string) *LogLabel {
	m_log_label = &LogLabel{}
	m_log_label.ExtendBaseWidget(m_log_label)
	m_log_label.SetText(content)
	return m_log_label
}

func SetLogLabel(content string) {
	if m_log_label != nil {
		m_log_label.SetText(content)
	}
}
