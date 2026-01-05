//go:build windows

package ui2

import (
	"fmt"
	"regexp"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// 匹配日志日期前缀的正则表达式，如 "2024/01/01 " 或 "2024-01-01 "，保留时间部分
var logDateTimeRegex = regexp.MustCompile(`^\d{4}[/-]\d{2}[/-]\d{2}\s+`)

const (
	// 日志最大条目数，避免内存占用过大
	maxLogEntries = 500
	// 日志显示行高度
	logRowHeight = 15
	// 日志显示行数
	logVisibleRows = 15
)

var (
	m_log_list    *widget.List
	m_log_entries []string
	m_log_mutex   sync.RWMutex
	m_log_scroll  *container.Scroll
)

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

func UILogPrintF(a ...any) {
	var content string

	switch len(a) {
	case 1:
		content = a[0].(string)
	default:
		content = fmt.Sprintf(a[0].(string), a[1:]...)
	}

	appendLogEntry(content)
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

	// 创建可滚动的容器包装 List，自适应高度
	logContainer := container.NewVScroll(m_log_list)
	// 设置最小高度，但允许根据窗口大小自动扩展
	minListHeight := float32(logVisibleRows * logRowHeight)
	logContainer.SetMinSize(fyne.NewSize(0, minListHeight))

	m_log_scroll = logContainer

	// 创建日志标题
	logTitle := widget.NewRichTextFromMarkdown("**运行日志**: ")

	// 组合标题和日志列表，使用 Border 布局让日志列表自适应
	logContent := container.NewBorder(
		logTitle,     // 顶部标题
		nil,          // 底部
		nil,          // 左侧
		nil,          // 右侧
		logContainer, // 中心（自适应区域）
	)

	return logContent
}
