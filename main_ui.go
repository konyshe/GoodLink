//go:build windows && !cmd

package main

import (
	"embed"
	"goodlink/config"
	"goodlink/ui2"
	"goodlink/utils"
	goodlink_config "goodlink3/config"

	"github.com/getlantern/systray"
)

//go:embed assert/tray_idle.ico assert/tray_warning.ico assert/tray_danger.ico assert/tray_success.ico
var trayIcons embed.FS

func main() {
	// 检查单实例，如果不是第一个实例则退出
	// 必须在创建任何UI资源之前检查，避免影响已运行的实例
	if !utils.CheckSingleInstance() {
		// 已有实例运行，直接退出
		return
	}

	// 启动前清理遗留的cmd进程
	utils.CleanupOrphanedCmdProcesses()

	goodlink_config.DeleteLocalConfig()

	config.SetVersion(GetVersionFromAppConfig())

	config.Help()

	idle, _ := trayIcons.ReadFile("assert/tray_idle.ico")
	warning, _ := trayIcons.ReadFile("assert/tray_warning.ico")
	danger, _ := trayIcons.ReadFile("assert/tray_danger.ico")
	success, _ := trayIcons.ReadFile("assert/tray_success.ico")
	ui2.InitTrayIcons(idle, warning, danger, success)

	ui2.Init()

	uiURL, err := ui2.StartServer()
	if err != nil {
		return
	}

	// 监听显示窗口请求（二次启动时重新打开浏览器）
	go func() {
		for range utils.GetShowWindowChan() {
			ui2.OpenBrowser(uiURL)
		}
	}()

	ui2.OpenBrowser(uiURL)

	// 主线程运行托盘消息循环；退出时停止子进程
	systray.Run(func() {
		ui2.SetupTray(uiURL)
	}, ui2.OnTrayExit)
}
