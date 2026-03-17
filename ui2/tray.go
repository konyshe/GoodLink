//go:build windows

package ui2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"image/color"
)

var (
	trayApp         desktop.App
	currentDotColor color.NRGBA
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

func iconForDotColor(c color.NRGBA) fyne.Resource {
	var data []byte
	if c.R == DotColorIdle.R && c.G == DotColorIdle.G && c.B == DotColorIdle.B {
		data = trayIconIdle
	} else if c.R == DotColorWarning.R && c.G == DotColorWarning.G && c.B == DotColorWarning.B {
		data = trayIconWarning
	} else if c.R == DotColorDanger.R && c.G == DotColorDanger.G && c.B == DotColorDanger.B {
		data = trayIconDanger
	} else if c.R == DotColorSuccess.R && c.G == DotColorSuccess.G && c.B == DotColorSuccess.B {
		data = trayIconSuccess
	} else {
		data = trayIconIdle
	}
	if len(data) == 0 {
		return nil
	}
	return fyne.NewStaticResource("tray_icon.ico", data)
}

func SetTrayApp(desk desktop.App) {
	trayApp = desk
	icon := iconForDotColor(DotColorIdle)
	if icon != nil {
		trayApp.SetSystemTrayIcon(icon)
		currentDotColor = DotColorIdle
	}
}

func UpdateTrayIcon(dotColor color.NRGBA) {
	if trayApp == nil {
		return
	}
	if dotColor == currentDotColor {
		return
	}
	currentDotColor = dotColor
	icon := iconForDotColor(dotColor)
	if icon != nil {
		trayApp.SetSystemTrayIcon(icon)
	}
}
