//go:build windows

package ui2

import (
	"sync/atomic"

	"goodlink/config"

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

// InitTrayIcons sets the 4 pre-generated tray icon bytes (ICO format).
// Icons should be from assert/tray_idle.ico, tray_warning.ico, tray_danger.ico, tray_success.ico.
func InitTrayIcons(idle, warning, danger, success []byte) {
	trayIconIdle = idle
	trayIconWarning = warning
	trayIconDanger = danger
	trayIconSuccess = success
}

// iconBytesForState returns the tray icon bytes for the given button state.
func iconBytesForState(state buttonState) []byte {
	switch state.kind {
	case kindIdle, kindInitializing:
		return trayIconIdle
	case kindStarting, kindConnecting, kindConnectingNat4, kindStopping:
		return trayIconWarning
	case kindConnectingNat4ToNat4:
		return trayIconDanger
	case kindConnected, kindRunning:
		return trayIconSuccess
	default:
		return trayIconIdle
	}
}

func UpdateTrayIcon(state buttonState) {
	if !trayReady.Load() {
		return
	}
	data := iconBytesForState(state)
	if len(data) > 0 {
		systray.SetIcon(data)
	}
}

func SetupTray(uiURL string) {
	stateMu.RLock()
	btn := currentButton
	stateMu.RUnlock()

	trayReady.Store(true)
	UpdateTrayIcon(btn)
	systray.SetTooltip(M_APP_TITLE + "  v" + config.GetVersion())

	// 创建菜单，确保只有一个退出选项
	mOpen := systray.AddMenuItem("打开主程序", "在浏览器中打开")
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
