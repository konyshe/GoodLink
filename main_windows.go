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
	myWindow      fyne.Window
)

const (
	M_APP_TITLE = "GoodLink"
)

func LogInit(log_view *widget.Label) {
	gogo.Log().RegistInfo(func(content string) {
		log_view.SetText(content)
		log.Println(content)
		myWindow.Resize(myWindow.Content().MinSize())
	})
	gogo.Log().RegistDebug(func(content string) {
		log.Println(content)
		myWindow.Resize(myWindow.Content().MinSize())
	})
	gogo.Log().RegistError(func(content string) {
		log_view.SetText(content)
		log.Println(content)
		myWindow.Resize(myWindow.Content().MinSize())
	})
}

func RemoteUI() *fyne.Container {
	remote_addr_box := ui2.NewIpPortEntry("127.0.0.1:22")
	remote_addr_box2 := container.New(layout.NewVBoxLayout(), widget.NewRichTextWithText("转发目标地址: "), remote_addr_box)

	radio := widget.NewRadioGroup([]string{"代理模式", "转发模式"}, nil)
	radio.OnChanged = func(value string) {
		switch value {
		case "代理模式":
			remote_addr_box2.Hide()
		case "转发模式":
			remote_addr_box2.Show()
		default:
			radio.SetSelected("代理模式")
		}
		myWindow.Resize(myWindow.Content().MinSize())
	}
	radio.SetSelected("代理模式")
	radio.Horizontal = true

	return container.NewVBox(
		container.New(layout.NewVBoxLayout(), widget.NewRichTextWithText("连接密钥: "), keyValidated),
		container.New(layout.NewHBoxLayout(), widget.NewRichTextWithText("工作模式: "), radio),
		remote_addr_box2,
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
		myWindow.Resize(myWindow.Content().MinSize())
	}
	radio.SetSelected("只允许本机")
	radio.Horizontal = true

	return container.NewVBox(
		container.New(layout.NewVBoxLayout(), widget.NewRichTextWithText("连接密钥: "), keyValidated),
		container.New(layout.NewHBoxLayout(), widget.NewRichTextWithText("访问权限: "), radio),
		container.New(layout.NewVBoxLayout(), widget.NewRichTextWithText("监听端口: "), entryValidated),
	)
}

func main() {
	myApp := app.New()
	myApp.Settings().SetTheme(&theme.MyTheme{})
	icon, _ := fyne.LoadResourceFromPath("./theme/favicon.png")
	myApp.SetIcon(icon)
	myWindow = myApp.NewWindow(M_APP_TITLE)

	if desk, ok := myApp.(desktop.App); ok {
		m := fyne.NewMenu(M_APP_TITLE,
			fyne.NewMenuItem("Show", func() {
				myWindow.Show()
			}))
		desk.SetSystemTrayMenu(m)
	}

	keyValidated = ui2.NewStringEntry("请自定义16-32长度字符串")
	keyValidated.CreateRenderer().Layout(keyValidated.Size())

	localUI := LocalUI()
	remoteUI := RemoteUI()

	radio := widget.NewRadioGroup([]string{"Remote", "Local"}, func(value string) {
		log.Println("Radio set to", value)
		myWindow.Resize(myWindow.Content().MinSize())
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
		myWindow.Resize(myWindow.Content().MinSize())
	}
	radio.SetSelected("Local")
	radio.Horizontal = true
	fixed := container.New(layout.NewHBoxLayout(), widget.NewRichTextWithText("请选择工作端: "), radio)

	log_view := widget.NewLabel("等待启动")
	LogInit(log_view)
	log_view2 := container.New(layout.NewHBoxLayout(), widget.NewRichTextWithText("状态: "), log_view)

	start_button := widget.NewButton("点击启动", func() {
		log_view.SetText("正在启动...")
		myWindow.Resize(myWindow.Content().MinSize())
	})

	myWindow.SetContent(container.New(layout.NewVBoxLayout(), fixed, localUI, remoteUI, log_view2, start_button))
	myWindow.SetCloseIntercept(func() {
		myWindow.Resize(myWindow.Content().MinSize())
		myWindow.Hide()
	})
	myWindow.ShowAndRun()
}
