package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// 获取 cmd 版本可执行文件路径
func GetCmdExePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "goodlink-windows-amd64-cmd.exe"
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, "goodlink-windows-amd64-cmd.exe")
}

// killProcess 强制终止进程（使用 taskkill 确保终止）
func KillProcess(pid int) {
	// 先尝试用 taskkill /F /T 强制终止进程树
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
}

// CleanupOrphanedCmdProcesses 清理遗留的cmd进程（通过进程名查找并终止）
func CleanupOrphanedCmdProcesses() {
	cmdExeName := filepath.Base(GetCmdExePath())

	// 使用 tasklist 查找所有匹配的进程
	cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", cmdExeName), "/FO", "CSV", "/NH")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		// 如果 tasklist 失败，尝试直接使用 taskkill 按进程名终止
		killCmd := exec.Command("taskkill", "/F", "/IM", cmdExeName)
		killCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		killCmd.Run()
		return
	}

	// 解析输出，提取PID
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// CSV格式: "进程名","PID","会话名","会话#","内存使用"
		// 例如: "goodlink-windows-amd64-cmd.exe","1234","Console","1","12345 K"
		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			// 移除引号
			pidStr := strings.Trim(parts[1], "\"")
			var pid int
			if _, err := fmt.Sscanf(pidStr, "%d", &pid); err == nil {
				KillProcess(pid)
			}
		}
	}
}
