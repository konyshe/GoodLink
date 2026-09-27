//go:build unix

package utils

import (
	"os"
	"path/filepath"
	"strconv"
)

func WriteWorkerPid(serviceID string, pid int) {}

func RemoveWorkerPid(serviceID string) {}

// RemoveAllWorkerPids Unix 侧不落盘 pid 文件, 无需清理, 仅为与 Windows 侧签名对称。
func RemoveAllWorkerPids() {}

func KillProcess(pid int) {
	// 与 Windows 侧保持一致: 拒绝非法 PID, 也绝不杀自己
	if pid <= 0 || pid == os.Getpid() {
		return
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}

// CleanupOrphanedCmdProcesses 清理上一次运行遗留的命令行子进程。
//
// 原来用 `pgrep -f <exePath>`: pgrep 的 pattern 是扩展正则(路径里的点号会被当通配符),
// 而 -f 匹配的是完整命令行, 于是 `vim goodlink.json`、`tail -f goodlink.log`
// 乃至 shell 本身都会被命中并被 SIGKILL, 属于误杀无关进程。
// 这里改为读取 /proc/<pid>/exe 与自身可执行文件比对, 与 Windows 侧的镜像路径比对语义一致。
func CleanupOrphanedCmdProcesses() {
	self := normalizeExePath(GetCmdExePath())
	if self == "" {
		return
	}

	selfPid := os.Getpid()
	parentPid := os.Getppid()

	// macOS 等没有 /proc 的系统直接跳过: 宁可漏清理, 也不能退化成模糊匹配
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		// 排除自身与父进程(父进程可能正是刚拉起本进程的托盘)
		if pid == selfPid || pid == parentPid {
			continue
		}

		exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil {
			// 无权限或进程已退出
			continue
		}
		if normalizeExePath(exe) != self {
			continue
		}
		KillProcess(pid)
	}
}

// normalizeExePath 把路径规整成绝对路径并解析符号链接,
// 避免同一个文件因相对路径或软链而比对失败
func normalizeExePath(path string) string {
	if path == "" {
		return ""
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	abs = filepath.Clean(abs)
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		abs = resolved
	}
	return abs
}
