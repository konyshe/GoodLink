package ui2

import (
	"fmt"
	"os"
	"sync"
)

const maxLogEntries = 500

var (
	m_shared_logs []string
	m_svc_logs    = map[string][]string{}
	m_log_mutex   sync.RWMutex
)

func appendLogEntry(serviceID, content string) {
	fmt.Fprintln(os.Stderr, content)

	m_log_mutex.Lock()
	if serviceID == "" {
		m_shared_logs = appendRing(m_shared_logs, content)
	} else {
		m_svc_logs[serviceID] = appendRing(m_svc_logs[serviceID], content)
	}
	m_log_mutex.Unlock()

	broadcastLog(serviceID, content)
}

func appendRing(entries []string, content string) []string {
	entries = append(entries, content)
	if len(entries) > maxLogEntries {
		entries = entries[len(entries)-maxLogEntries:]
	}
	return entries
}

func UILogPrintF(a ...any) {
	appendLogEntry("", formatLog(a...))
}

func ServiceLogPrintF(serviceID string, a ...any) {
	appendLogEntry(serviceID, formatLog(a...))
}

func formatLog(a ...any) string {
	switch len(a) {
	case 1:
		return a[0].(string)
	default:
		return fmt.Sprintf(a[0].(string), a[1:]...)
	}
}

func snapshotSharedLogs() []string {
	m_log_mutex.RLock()
	defer m_log_mutex.RUnlock()
	out := make([]string, len(m_shared_logs))
	copy(out, m_shared_logs)
	return out
}

func snapshotServiceLogs(id string) []string {
	m_log_mutex.RLock()
	defer m_log_mutex.RUnlock()
	src := m_svc_logs[id]
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func dropServiceLogs(id string) {
	m_log_mutex.Lock()
	delete(m_svc_logs, id)
	m_log_mutex.Unlock()
}
