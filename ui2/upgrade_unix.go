//go:build !windows

package ui2

import (
	"os"
	"syscall"
	"time"
)

func waitForPidExit(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return
		}
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}
