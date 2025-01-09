//go:build windows && !cmd

package main

import (
	"goodlink/config"
	_ "goodlink/pro"
	"goodlink/theme"
	"goodlink/tools"
	"goodlink/ui2"
	"log"
	"os"
	"time"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/driver/desktop"
)

const (
	M_APP_TITLE = "GoodLink"
)

func main2() {
	myApp := app.New()
	myApp.Settings().SetTheme(&theme.MyTheme{})
	icon, _ := fyne.LoadResourceFromPath("./theme/favicon.png")
	myApp.SetIcon(icon)
	myWindow := myApp.NewWindow(M_APP_TITLE + "  v" + myApp.Metadata().Version)

	if desk, ok := myApp.(desktop.App); ok {
		m := fyne.NewMenu(M_APP_TITLE+"  v"+myApp.Metadata().Version,
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

func main() {
	config.Help()

	tools.GuardStart(main2, 500*time.Millisecond, func(err error) {
		// if 0: err==nil; -1: err==255; -2: err==254; err==1: 1; err==2
		if err == nil {
			os.Exit(0)
		}
		log.Printf("   异常退出: %v", err)
		tools.DingF("error: %v", err)
	})
}
