//go:build windows

package ui2

import (
	"embed"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"image/color"
)

//go:embed tray_idle.png tray_warning.png tray_danger.png tray_success.png
var trayIconFS embed.FS

var (
	trayApp         desktop.App
	currentDotColor color.NRGBA
)

func InitTrayIcons() {
	// No-op: icons are embedded and selected by UpdateTrayIcon.
}

func iconForDotColor(c color.NRGBA) fyne.Resource {
	data, _ := trayIconFS.ReadFile(iconNameForDotColor(c))
	if len(data) == 0 {
		return nil
	}
	return fyne.NewStaticResource("tray_icon.png", data)
}

func iconNameForDotColor(c color.NRGBA) string {
	if c.R == DotColorIdle.R && c.G == DotColorIdle.G && c.B == DotColorIdle.B {
		return "tray_idle.png"
	}
	if c.R == DotColorWarning.R && c.G == DotColorWarning.G && c.B == DotColorWarning.B {
		return "tray_warning.png"
	}
	if c.R == DotColorDanger.R && c.G == DotColorDanger.G && c.B == DotColorDanger.B {
		return "tray_danger.png"
	}
	if c.R == DotColorSuccess.R && c.G == DotColorSuccess.G && c.B == DotColorSuccess.B {
		return "tray_success.png"
	}
	return "tray_idle.png"
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
