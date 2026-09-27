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

// stillActive 是 Windows GetExitCodeProcess 对"进程仍在运行"的返回值(STILL_ACTIVE)。
// x/sys/windows 未导出该常量, 这里按 Win32 定义写死。
const stillActive uint32 = 259

func workerPidCandidates() []string {
	var out []string
	collect := func(dir string) {
		matches, _ := filepath.Glob(filepath.Join(dir, "goodlink-worker-*.pid"))
		out = append(out, matches...)
		legacy := filepath.Join(dir, workerPidFileName)
		if _, err := os.Stat(legacy); err == nil {
			out = append(out, legacy)
		}
	}
	if dir, err := exeDir(); err == nil {
		collect(dir)
	}
	if ad := os.Getenv("LOCALAPPDATA"); ad != "" {
		collect(filepath.Join(ad, "Goodlink"))
		out = append(out, filepath.Join(ad, "Goodlink", "worker.pid"))
	}
	return out
}

func workerPidName(serviceID string) string {
	id := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' {
			return '_'
		}
		return r
	}, serviceID)
	if id == "" {
		id = "default"
	}
	return "goodlink-worker-" + id + ".pid"
}

func WriteWorkerPid(serviceID string, pid int) {
	if pid <= 0 {
		return
	}
	content := []byte(strconv.Itoa(pid))
	name := workerPidName(serviceID)
	if dir, err := exeDir(); err == nil {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0644); err == nil {
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
	_ = os.WriteFile(filepath.Join(dir, name), content, 0644)
}

func RemoveWorkerPid(serviceID string) {
	name := workerPidName(serviceID)
	if dir, err := exeDir(); err == nil {
		_ = os.Remove(filepath.Join(dir, name))
	}
	if ad := os.Getenv("LOCALAPPDATA"); ad != "" {
		_ = os.Remove(filepath.Join(ad, "Goodlink", name))
	}
}

// RemoveAllWorkerPids 程序退出前统一清掉本程序写下的所有 pid 文件。
//
// 只在退出路径调用: 此刻子进程已被 StopCmdProcess 停止, 剩下的文件要么指向已死进程,
// 要么是上次异常退出留下的, 都没有保留价值。程序有单实例保护, 不会误伤其它实例。
func RemoveAllWorkerPids() {
	for _, p := range workerPidCandidates() {
		_ = os.Remove(p)
	}
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
		// 进程已经不在了: pid 文件不再指向任何东西, 留着只会永久堆积, 直接删。
		// (processImageMatches 也返回 false, 但那混了"进程存在但不匹配"的情况, 必须先分开。)
		if !processAlive(pid) {
			_ = os.Remove(p)
			continue
		}
		// 镜像路径不匹配时不能删除 pid 文件: 它可能属于别的正常实例,
		// 删掉会让对方失去清理能力。只跳过即可。
		if !processImageMatches(pid, self) {
			continue
		}
		KillProcess(pid)
		_ = os.Remove(p)
	}
}

// processAlive 判断 pid 对应的进程是否仍然存在。
// 用来区分"pid 文件已失效"(可以删) 与"进程还在但镜像不匹配"(要保留)。
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == stillActive
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
