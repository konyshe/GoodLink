package pro

import "log"

const (
	TagStatusPrefix          = "[GOODLINK_STATUS]"
	TagStatusConnecting      = "connecting"
	TagStatusConnectingNAT4  = "connecting_nat4"
	TagStatusConnected       = "connected"
	TagStatusRunning         = "running"
	TagStatusVersionMismatch = "version_mismatch"
	TagStatusNeedAdmin       = "need_admin"

	TagProxyPrefix = "[GOODLINK_PROXY]"
)

// LogStatus 输出带 TagStatusPrefix 的状态行，供 UI 等解析
func UpdateStartButtonStatue(status string) {
	log.Printf("%s%s", TagStatusPrefix, status)
}

// ReportProxyAddr 输出内置代理监听地址，供 UI 解析（不进入运行日志）。
func ReportProxyAddr(addr string) {
	log.Printf("%s%s", TagProxyPrefix, addr)
}
