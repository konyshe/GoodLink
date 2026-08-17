//go:build windows

package ui2

import (
	"fmt"
	"sync"
)

const (
	// 日志最大条目数，避免内存占用过大
	maxLogEntries = 500
)

var (
	m_log_entries []string
	m_log_mutex   sync.RWMutex
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

	// 通过 SSE 推送到浏览器（UILogPrintF 可能从 goroutine 调用）
	broadcastLog(content)
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

func snapshotLogs() []string {
	m_log_mutex.RLock()
	defer m_log_mutex.RUnlock()
	out := make([]string, len(m_log_entries))
	copy(out, m_log_entries)
	return out
}
