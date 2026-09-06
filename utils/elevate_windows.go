//go:build windows

package utils

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

const ElevateCancelledMsg = "已取消管理员授权, TUN模式无法启动"

// RelaunchAsAdmin 通过 UAC 以管理员身份重新启动自身。
// 用户在 UAC 对话框点击取消时返回 ElevateCancelledMsg 错误。
func RelaunchAsAdmin(args ...string) error {
	exe := GetCmdExePath()

	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	var argPtr *uint16
	if len(args) > 0 {
		argPtr, err = windows.UTF16PtrFromString(quoteArgs(args))
		if err != nil {
			return err
		}
	}

	if err := windows.ShellExecute(0, verb, file, argPtr, cwd, windows.SW_SHOWNORMAL); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return errors.New(ElevateCancelledMsg)
		}
		return fmt.Errorf("以管理员权限启动失败: %v", err)
	}
	return nil
}

func quoteArgs(args []string) string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if a == "" || strings.ContainsAny(a, " \t\"") {
			out = append(out, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
			continue
		}
		out = append(out, a)
	}
	return strings.Join(out, " ")
}
