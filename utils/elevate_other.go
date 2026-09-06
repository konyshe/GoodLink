//go:build !windows

package utils

import "errors"

// RelaunchAsAdmin 非 Windows 平台不做自动提权，提示用户手动以管理员权限重启。
func RelaunchAsAdmin(args ...string) error {
	return errors.New(NeedAdminRestartMsg)
}
