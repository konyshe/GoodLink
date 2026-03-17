//go:build windows

package ui2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
)

var (
	trayApp desktop.App
	// Pre-generated tray icon data (ICO bytes), set by InitTrayIcons.
	trayIconIdle    []byte
	trayIconWarning []byte
	trayIconDanger  []byte
	trayIconSuccess []byte
)

// InitTrayIcons sets the 4 pre-generated tray icon bytes (ICO format).
// Icons should be from assert/tray_idle.ico, tray_warning.ico, tray_danger.ico, tray_success.ico.
func InitTrayIcons(idle, warning, danger, success []byte) {
	trayIconIdle = idle
	trayIconWarning = warning
	trayIconDanger = danger
	trayIconSuccess = success
}

// iconForState returns the tray icon resource for the given button state.
func iconForState(state buttonState) fyne.Resource {
	var data []byte
	switch {
	case state == buttonStateIdle, state == buttonStateInitializing:
		data = trayIconIdle
	case state == buttonStateStarting, state == buttonStateConnecting, state == buttonStateConnectingNat4, state == buttonStateStopping:
		data = trayIconWarning
	case state == buttonStateConnectingNat4ToNat4:
		data = trayIconDanger
	case state == buttonStateConnected, state == buttonStateRunning:
		data = trayIconSuccess
	default:
		data = trayIconIdle
	}
	if len(data) == 0 {
		return nil
	}
	return fyne.NewStaticResource("tray_icon.ico", data)
}

func SetTrayApp(state buttonState) {
	icon := iconForState(state)
	if icon != nil {
		trayApp.SetSystemTrayIcon(icon)
	}
}

func UpdateTrayIcon(state buttonState) {
	if trayApp == nil {
		return
	}
	icon := iconForState(state)
	if icon != nil {
		trayApp.SetSystemTrayIcon(icon)
	}
}
