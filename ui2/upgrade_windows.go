//go:build windows

package ui2

import (
	"time"

	"golang.org/x/sys/windows"
)

func waitForPidExit(pid int, timeout time.Duration) {
	if pid <= 0 {
		return
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)

	ms := uint32(windows.INFINITE)
	if timeout > 0 {
		ms = uint32(timeout / time.Millisecond)
	}
	_, _ = windows.WaitForSingleObject(h, ms)
}
