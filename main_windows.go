//go:build windows && !cmd

package main

import (
	"goodlink/config"
	"goodlink/pro"
	_ "goodlink/pro"
	"goodlink/theme"
	"goodlink/ui2"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
)

const (
	M_APP_TITLE = "GoodLink"
)

func main() {
	pro.SetVersion(GetVersion())

	config.Help(GetVersion())

	myApp := app.New()
	myApp.Settings().SetTheme(&theme.MyTheme{})
	myWindow := myApp.NewWindow(M_APP_TITLE + "  v" + GetVersion()) //myApp.Metadata().Version)

	if desk, ok := myApp.(desktop.App); ok {
		// 创建菜单项
		openItem := fyne.NewMenuItem("打开主程序", func() {
			myWindow.Show()
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
