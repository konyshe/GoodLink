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
		m := fyne.NewMenu(M_APP_TITLE,
			fyne.NewMenuItem("打开主程序", func() {
				myWindow.Show()
			}),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("退出程序", func() {
				ui2.StopCmdProcess()
				myApp.Quit()
			}))
		desk.SetSystemTrayMenu(m)
	}

	myWindow.SetContent(ui2.GetMainUI(&myWindow))

	myWindow.SetCloseIntercept(func() {
		myWindow.Hide()
	})

	// 程序退出时停止子进程
	myApp.Lifecycle().SetOnStopped(func() {
		ui2.StopCmdProcess()
	})

	myWindow.ShowAndRun()
}
