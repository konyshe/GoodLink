//go:build windows

package main

import (
	"gogo"
	"log"
	"time"

	"goodlink/theme"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

var (
	m_run_state float64
)

const (
	M_USE_LOCAL_HTTP_SVR = true
)

const (
	M_APP_TITLE = "GoodLink"
)

func LogInit(log_view *widget.Label) {
	gogo.Log().RegistInfo(func(content string) {
		log_view.SetText(content)
		log.Println(content)
	})
	gogo.Log().RegistDebug(func(content string) {
		log.Println(content)
	})
	gogo.Log().RegistError(func(content string) {
		log_view.SetText(content)
		log.Println(content)
	})
}

func main2() {
	m_run_state = 0.1

	if M_USE_LOCAL_HTTP_SVR {
		gogo.Log().Info("正在初始化端口1...")
	}
	m_run_state = 0.3

	m_run_state = 0.4

	gogo.Utils().TimeSleepSecond(1)

	gogo.Log().Info("正在建立加密隧道...")

	m_run_state = 0.8

	gogo.Log().Info("正在创建服务...")

	m_run_state = 1

	gogo.Log().Info("访问地址：%s")
}

func main() {
	myApp := app.New()
	myApp.Settings().SetTheme(&theme.MyTheme{})
	icon, _ := fyne.LoadResourceFromPath("./theme/favicon.png")
	myApp.SetIcon(icon)
	myWindow := myApp.NewWindow(M_APP_TITLE)

	if desk, ok := myApp.(desktop.App); ok {
		m := fyne.NewMenu(M_APP_TITLE,
			fyne.NewMenuItem("Show", func() {
				myWindow.Show()
			}))
		desk.SetSystemTrayMenu(m)
	}

	log_view := widget.NewLabel("等待启动")
	LogInit(log_view)

	copy_view := widget.NewButton("点击启动", func() {
		log_view.SetText("正在启动...")
	})

	progress := widget.NewProgressBar()
	infinite := widget.NewProgressBarInfinite()

	go func() {
		for {
			time.Sleep(time.Millisecond * 250)
			progress.SetValue(m_run_state)
			if m_run_state >= 1 {
				copy_view.Enable()
			}
		}
	}()
	progress_view := container.NewVBox(progress, infinite)

	radio := widget.NewRadioGroup([]string{"Remote", "Local"}, func(value string) {
		log.Println("Radio set to", value)
	})
	radio.SetSelected("Local")
	radio.OnChanged = func(value string) {
		log.Println("Radio set to", value)
	}
	radio.Horizontal = true

	fixed := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("请选择工作端", radio)),
	)

	grid := container.New(layout.NewGridLayout(1), fixed, log_view, progress_view, copy_view)
	myWindow.SetContent(grid)
	myWindow.Resize(fyne.NewSize(400, 100))

	myWindow.SetCloseIntercept(func() {
		myWindow.Hide()
	})

	myWindow.ShowAndRun()
}
