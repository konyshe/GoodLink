//go:build windows

package ui2

import (
	"fmt"
	"log"
	"regexp"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 匹配日志日期前缀的正则表达式，如 "2024/01/01 " 或 "2024-01-01 "，保留时间部分
var logDateTimeRegex = regexp.MustCompile(`^\d{4}[/-]\d{2}[/-]\d{2}\s+`)

const (
	// 日志最大条目数，避免内存占用过大
	maxLogEntries = 500
	// 日志显示行高度
	logRowHeight = 20
	// 日志显示行数
	logVisibleRows = 20
)

var (
	m_log_label   *LogLabel
	m_log_list    *widget.List
	m_log_entries []string
	m_log_mutex   sync.RWMutex
	m_log_scroll  *container.Scroll
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

// appendLogEntry 追加日志条目到列表
func appendLogEntry(content string) {
	// 先添加日志条目
	m_log_mutex.Lock()
	m_log_entries = append(m_log_entries, content)

	// 限制日志数量
	if len(m_log_entries) > maxLogEntries {
		m_log_entries = m_log_entries[len(m_log_entries)-maxLogEntries:]
	}
	m_log_mutex.Unlock()

	// 在锁外刷新列表，避免死锁
	if m_log_list != nil {
		m_log_list.Refresh()
		// 滚动到最底部
		m_log_list.ScrollToBottom()
	}
}

// stripLogDateTime 去除日志内容前面的日期时间前缀
func stripLogDateTime(content string) string {
	return logDateTimeRegex.ReplaceAllString(content, "")
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

	// 去除日期时间前缀后追加到日志列表
	displayContent := stripLogDateTime(content)
	appendLogEntry(displayContent)

	// 保留原有的 LogLabel 更新（截断显示），也使用去除日期后的内容
	if len(displayContent) > 32 {
		SetLogLabel(displayContent[:32])
	} else {
		SetLogLabel(displayContent)
	}
}

func NewLogLabel(content string) *LogLabel {
	m_log_label = &LogLabel{}
	m_log_label.ExtendBaseWidget(m_log_label)
	m_log_label.SetText(content)

	return m_log_label
}

// NewLogList 创建日志显示列表组件，带滚动条，20行高度
func NewLogList() fyne.CanvasObject {
	// 初始化日志条目
	m_log_entries = make([]string, 0, maxLogEntries)

	// 创建 List 组件
	m_log_list = widget.NewList(
		// 返回条目数量
		func() int {
			m_log_mutex.RLock()
			defer m_log_mutex.RUnlock()
			return len(m_log_entries)
		},
		// 创建条目模板
		func() fyne.CanvasObject {
			label := widget.NewLabel("")
			label.TextStyle = fyne.TextStyle{Monospace: true}
			return label
		},
		// 更新条目内容
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			m_log_mutex.RLock()
			defer m_log_mutex.RUnlock()
			if id < len(m_log_entries) {
				content := m_log_entries[id]
				if label, ok := obj.(*widget.Label); ok {
					label.SetText(content)
				}
			}
		},
	)

	// 创建一个固定高度的容器包装 List
	// 20行 * 每行高度
	listHeight := float32(logVisibleRows * logRowHeight)
	logContainer := container.NewVScroll(m_log_list)
	logContainer.SetMinSize(fyne.NewSize(0, listHeight))

	m_log_scroll = logContainer

	// 创建日志标题
	logIcon := widget.NewIcon(theme.InfoIcon())
	logTitle := widget.NewRichTextFromMarkdown("**运行日志**")
	logTitleContainer := container.NewHBox(logIcon, logTitle)

	// 创建背景
	logBg := canvas.NewRectangle(bgColorCard)
	logBg.CornerRadius = cornerRadius

	// 组合标题和日志列表
	logContent := container.NewVBox(
		container.NewPadded(logTitleContainer),
		container.NewPadded(logContainer),
	)

	logWithBg := container.NewStack(logBg, logContent)

	return logWithBg
}
