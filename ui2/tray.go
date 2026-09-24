//go:build windows

package ui2

import (
	"sync/atomic"

	"github.com/getlantern/systray"
)

var (
	// Pre-generated tray icon data (ICO bytes), set by InitTrayIcons.
	trayIconIdle    []byte
	trayIconWarning []byte
	trayIconDanger  []byte
	trayIconSuccess []byte
	trayReady       atomic.Bool
)

func applyTrayIcons(idle, warning, danger, success []byte) {
	trayIconIdle = idle
	trayIconWarning = warning
	trayIconDanger = danger
	trayIconSuccess = success
}

func iconBytesForStatus(status int) []byte {
	switch status {
	case StatusIdle, StatusDetecting:
		return trayIconIdle
	case StatusStarting, StatusConnecting, StatusStopping:
		return trayIconWarning
	case StatusConnectingNAT4, StatusConnectFailed:
		return trayIconDanger
	case StatusConnected, StatusRunning:
		return trayIconSuccess
	default:
		return trayIconIdle
	}
}

func UpdateTrayIcon(status int) {
	if !trayReady.Load() {
		return
	}
	data := iconBytesForStatus(status)
	if len(data) > 0 {
		systray.SetIcon(data)
	}
}

func refreshTray() {
	if !trayReady.Load() {
		return
	}
	UpdateTrayIcon(aggregateStatus())
	systray.SetTooltip(trayTooltip())
}

func RunLoop(uiURL string) {
	systray.Run(func() {
		SetupTray(uiURL)
	}, OnTrayExit)
}

func SetupTray(uiURL string) {
	trayReady.Store(true)
	refreshTray()

	// 创建菜单，确保只有一个退出选项
	mOpen := systray.AddMenuItem("打开控制页面", "打开控制页面")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("退出", "退出")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				OpenBrowser(uiURL)
			case <-mQuit.ClickedCh:
				StopCmdProcess()
				systray.Quit()
			}
		}
	}()
}

// OnTrayExit 程序退出时停止子进程
func OnTrayExit() {
	StopCmdProcess()
}
