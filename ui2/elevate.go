package ui2

import (
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"goodlink/utils"
)

// forceAutoConnect 由 --auto-connect 设置, 仅生效一次, 不写入 goodlink.json
var forceAutoConnect atomic.Bool

// portRetry 由 --wait-pid 设置, 表示本次是接替旧进程启动, 需要等固定端口释放
var portRetry atomic.Bool

func SetForceAutoConnect(enabled bool) {
	forceAutoConnect.Store(enabled)
}

func SetPortRetry(enabled bool) {
	portRetry.Store(enabled)
}

func portRetryEnabled() bool {
	return portRetry.Load()
}

func consumeForceAutoConnect() bool {
	return forceAutoConnect.Swap(false)
}

// WaitForPidExit 等待指定进程退出, 供提权重启时等待旧进程释放单实例锁与监听端口
func WaitForPidExit(pid int, timeout time.Duration) {
	waitForPidExit(pid, timeout)
}

// requestElevatedRestart 以管理员权限重新启动 Goodlink, 并让新进程自动继续本次连接。
// 返回 nil 表示提权进程已启动, 当前进程即将退出。
func requestElevatedRestart() error {
	if err := utils.RelaunchAsAdmin(
		"--no-browser",
		"--auto-connect",
		"--wait-pid", strconv.Itoa(os.Getpid()),
	); err != nil {
		return err
	}

	UILogPrintF("正在以管理员权限重新启动Goodlink...")
	setElevating(true)

	go func() {
		time.Sleep(600 * time.Millisecond)
		StopCmdProcess()
		os.Exit(0)
	}()
	return nil
}
