package ui2

import "embed"

//go:embed icons/tray_idle.ico icons/tray_warning.ico icons/tray_danger.ico icons/tray_success.ico
var trayIconFS embed.FS

func InitTrayIcons() {
	idle, _ := trayIconFS.ReadFile("icons/tray_idle.ico")
	warning, _ := trayIconFS.ReadFile("icons/tray_warning.ico")
	danger, _ := trayIconFS.ReadFile("icons/tray_danger.ico")
	success, _ := trayIconFS.ReadFile("icons/tray_success.ico")
	applyTrayIcons(idle, warning, danger, success)
}
