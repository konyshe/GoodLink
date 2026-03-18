package pro

import "log"

const (
	TagStatusPrefix          = "[GOODLINK_STATUS]"
	TagStatusConnecting      = "connecting"
	TagStatusConnectingNAT4  = "connecting_nat4"
	TagStatusConnected       = "connected"
	TagStatusRunning         = "running"
	TagStatusVersionMismatch = "version_mismatch"
)

// LogStatus 输出带 TagStatusPrefix 的状态行，供 UI 等解析
func UpdateStartButtonStatue(status string) {
	log.Printf("%s%s", TagStatusPrefix, status)
}
