//go:build windows

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// KillProcess 强制终止进程（使用 taskkill 确保终止）
func KillProcess(pid int) {
	if pid == os.Getpid() {
		return
	}
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
}

// CleanupOrphanedCmdProcesses 清理遗留的工作进程（通过进程名查找并终止，跳过当前 PID）
func CleanupOrphanedCmdProcesses() {
	selfPid := os.Getpid()
	cmdExeName := filepath.Base(GetCmdExePath())

	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", cmdExeName), "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV格式: "进程名","PID","会话名","会话#","内存使用"
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			pidStr := strings.Trim(parts[1], "\"")
			var pid int
			if _, err := fmt.Sscanf(pidStr, "%d", &pid); err == nil && pid != selfPid {
				KillProcess(pid)
			}
		}
	}
}
