//go:build windows

package main

import (
	"gogo"
	"log"

	"goodlink/theme"
	"goodlink/ui2"

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
	m_local_ip    string
	m_remote_addr string
	key_box       *fyne.Container
	keyValidated  *ui2.StringEntry
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

func RemoteUI() *fyne.Container {
	entryValidated := ui2.NewIpPortEntry("127.0.0.1:22")

	radio := widget.NewRadioGroup([]string{"代理模式", "转发模式"}, nil)
	radio.OnChanged = func(value string) {
		switch value {
		case "代理模式":
			entryValidated.SetText("")
			entryValidated.Disabled()
		case "转发模式":
			entryValidated.Enable()
		default:
			radio.SetSelected("代理模式")
		}
	}
	radio.SetSelected("代理模式")
	radio.Horizontal = true

	return container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("连接密钥: ", keyValidated)),
		widget.NewForm(
			widget.NewFormItem("工作模式: ", radio)),
		widget.NewForm(
			widget.NewFormItem("转发目标地址: ", entryValidated)),
	)
}

func LocalUI() *fyne.Container {
	entryValidated := ui2.NewPortEntry()

	radio := widget.NewRadioGroup([]string{"只允许本机", "允许局域网"}, nil)
	radio.OnChanged = func(value string) {
		switch value {
		case "只允许本机":
			m_local_ip = "127.0.0.1"
		case "允许局域网":
			m_local_ip = "0.0.0.0"
		default:
			radio.SetSelected("只允许本机")
		}
	}
	radio.SetSelected("只允许本机")
	radio.Horizontal = true

	return container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("连接密钥: ", keyValidated)),
		widget.NewForm(
			widget.NewFormItem("访问权限: ", radio)),
		widget.NewForm(
			widget.NewFormItem("监听端口: ", entryValidated)),
	)
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

	keyValidated = ui2.NewStringEntry("请自定义16-32长度字符串")

	localUI := LocalUI()
	remoteUI := RemoteUI()

	radio := widget.NewRadioGroup([]string{"Remote", "Local"}, func(value string) {
		log.Println("Radio set to", value)
	})
	radio.OnChanged = func(value string) {
		switch value {
		case "Remote":
			localUI.Hide()
			remoteUI.Show()

		case "Local":
			remoteUI.Hide()
			localUI.Show()

		default:
			radio.SetSelected("Local")
		}
	}
	radio.SetSelected("Local")
	radio.Horizontal = true
	fixed := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("请选择工作端: ", radio)),
	)

	log_view := widget.NewLabel("等待启动")
	LogInit(log_view)

	start_button := widget.NewButton("点击启动", func() {
		log_view.SetText("正在启动...")
	})

	myWindow.SetContent(container.New(layout.NewGridLayout(1), fixed, localUI, remoteUI, log_view, start_button))
	myWindow.Resize(fyne.NewSize(200, 100))
	myWindow.FixedSize()

	myWindow.SetCloseIntercept(func() {
		myWindow.Hide()
	})

	myWindow.ShowAndRun()
}
