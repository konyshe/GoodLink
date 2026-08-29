//go:build windows

package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
)

const workerPidFileName = "goodlink-worker.pid"

func workerPidCandidates() []string {
	var out []string
	if dir, err := exeDir(); err == nil {
		out = append(out, filepath.Join(dir, workerPidFileName))
	}
	if ad := os.Getenv("LOCALAPPDATA"); ad != "" {
		out = append(out, filepath.Join(ad, "Goodlink", "worker.pid"))
	}
	return out
}

func WriteWorkerPid(pid int) {
	if pid <= 0 {
		return
	}
	content := []byte(strconv.Itoa(pid))
	if dir, err := exeDir(); err == nil {
		p := filepath.Join(dir, workerPidFileName)
		if err := os.WriteFile(p, content, 0644); err == nil {
			return
		}
	}
	ad := os.Getenv("LOCALAPPDATA")
	if ad == "" {
		return
	}
	dir := filepath.Join(ad, "Goodlink")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, "worker.pid"), content, 0644)
}

func KillProcess(pid int) {
	if pid <= 0 || pid == os.Getpid() {
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(h)
	_ = windows.TerminateProcess(h, 1)
}

func CleanupOrphanedCmdProcesses() {
	self, err := filepath.Abs(GetCmdExePath())
	if err != nil {
		self = GetCmdExePath()
	}
	self = filepath.Clean(self)
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}

	for _, p := range workerPidCandidates() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var pid int
		if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &pid); err != nil || pid <= 0 || pid == os.Getpid() {
			_ = os.Remove(p)
			continue
		}
		if !processImageMatches(pid, self) {
			_ = os.Remove(p)
			continue
		}
		KillProcess(pid)
		_ = os.Remove(p)
	}
}

func processImageMatches(pid int, wantExe string) bool {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)

	n := uint32(32768)
	buf := make([]uint16, n)
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return false
	}
	got := windows.UTF16ToString(buf[:n])
	gotAbs, err := filepath.Abs(got)
	if err != nil {
		return false
	}
	gotAbs = filepath.Clean(gotAbs)
	if resolved, err := filepath.EvalSymlinks(gotAbs); err == nil {
		gotAbs = resolved
	}
	return strings.EqualFold(gotAbs, wantExe)
}
