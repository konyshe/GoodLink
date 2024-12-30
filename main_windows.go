//go:build windows

package main

import (
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
	myApp := app.New()
	myApp.Settings().SetTheme(&theme.MyTheme{})
	icon, _ := fyne.LoadResourceFromPath("./theme/favicon.png")
	myApp.SetIcon(icon)
	myWindow := myApp.NewWindow(M_APP_TITLE)

	if desk, ok := myApp.(desktop.App); ok {
		m := fyne.NewMenu(M_APP_TITLE,
			fyne.NewMenuItem("打开主程序", func() {
				myWindow.Show()
			}))
		desk.SetSystemTrayMenu(m)
	}

	myWindow.SetContent(ui2.GetMainUI(&myWindow))

	myWindow.SetCloseIntercept(func() {
		myWindow.Hide()
	})
	myWindow.ShowAndRun()
}
