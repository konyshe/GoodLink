//go:build unix

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func KillProcess(pid int) {
	if pid == os.Getpid() {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

func CleanupOrphanedCmdProcesses() {
	selfPid := os.Getpid()
	exePath := GetCmdExePath()

	out, err := exec.Command("pgrep", "-f", exePath).Output()
	if err != nil {
		return
	}

	for _, field := range strings.Fields(string(out)) {
		var pid int
		if _, err := fmt.Sscanf(field, "%d", &pid); err == nil && pid != selfPid {
			KillProcess(pid)
		}
	}
}
