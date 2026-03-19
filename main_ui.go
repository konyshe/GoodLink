//go:build windows && !cmd

package main

import (
	"embed"
	"goodlink/config"
	_ "goodlink/pro"
	"goodlink/theme"
	"goodlink/ui2"
	"goodlink/utils"
	goodlink_config "goodlink3/config"

	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
)

//go:embed assert/tray_idle.ico assert/tray_warning.ico assert/tray_danger.ico assert/tray_success.ico
var trayIcons embed.FS

const (
	M_APP_TITLE = "Goodlink"
)

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

	myApp := app.New()
	myApp.Settings().SetTheme(&theme.MyTheme{})
	myWindow := myApp.NewWindow(M_APP_TITLE + "  v" + config.GetVersion())

	// 监听显示窗口请求
	// Fyne的Show()方法会自动处理线程安全，可以直接在goroutine中调用
	go func() {
		for range utils.GetShowWindowChan() {
			// Fyne会自动处理线程安全，直接调用Show()
			myWindow.Show()
			myWindow.RequestFocus()
		}
	}()

	idle, _ := trayIcons.ReadFile("assert/tray_idle.ico")
	warning, _ := trayIcons.ReadFile("assert/tray_warning.ico")
	danger, _ := trayIcons.ReadFile("assert/tray_danger.ico")
	success, _ := trayIcons.ReadFile("assert/tray_success.ico")

	if desk, ok := myApp.(desktop.App); ok {
		// 创建菜单项
		openItem := fyne.NewMenuItem("打开主程序", func() {
			// 系统托盘菜单回调已经在主线程中执行，可以直接调用Show()
			myWindow.Show()
			myWindow.RequestFocus()
		})
		quitItem := fyne.NewMenuItem("退出", func() {
			ui2.StopCmdProcess()
			myApp.Quit()
		})

		// 创建菜单，确保只有一个退出选项
		m := fyne.NewMenu("",
			openItem,
			fyne.NewMenuItemSeparator(),
			quitItem)
		desk.SetSystemTrayMenu(m)

		ui2.InitTrayIcons(desk, idle, warning, danger, success)
	}

	myWindow.SetContent(ui2.GetMainUI(&myWindow))

	// 设置窗口初始大小：宽度等于高度（正方形窗口）
	minSize := myWindow.Content().MinSize()
	myWindow.Resize(fyne.NewSize(minSize.Width*2, minSize.Height))

	myWindow.SetCloseIntercept(func() {
		myWindow.Hide()
	})

	// 程序退出时停止子进程
	myApp.Lifecycle().SetOnStopped(func() {
		ui2.StopCmdProcess()
	})

	myWindow.ShowAndRun()
}
