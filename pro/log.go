package pro

import "log"

const (
	TagStatusPrefix = "[GOODLINK_STATUS]"
	TagProxyPrefix  = "[GOODLINK_PROXY]"

	StatusConnecting        = 1
	StatusConnectingNAT4    = 2
	StatusConnected         = 3
	StatusRunning           = 4
	StatusVersionMismatch   = 5
	StatusTransportMismatch = 6
	StatusNeedAdmin         = 7
	StatusConnectFailed     = 8
)

// UpdateStartButtonStatue 输出带 TagStatusPrefix 的整数状态，供 UI 解析。
func UpdateStartButtonStatue(status int) {
	log.Printf("%s%d", TagStatusPrefix, status)
}

// ReportProxyAddr 输出内置代理监听地址，供 UI 解析（不进入运行日志）。
func ReportProxyAddr(addr string) {
	log.Printf("%s%s", TagProxyPrefix, addr)
}
