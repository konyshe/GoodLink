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
	keyValidated  *widget.Entry
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
	remote_addr_box := ui2.NewIpPortEntry("例如: 127.0.0.1:3389")

	radio := widget.NewRadioGroup([]string{"代理模式", "转发模式"}, nil)
	radio.OnChanged = func(value string) {
		switch value {
		case "代理模式":
			m_remote_addr = remote_addr_box.Text
			remote_addr_box.SetText("不需要设置")
			remote_addr_box.Disable()
		case "转发模式":
			remote_addr_box.Enable()
			remote_addr_box.SetText(m_remote_addr)
		default:
			radio.SetSelected("代理模式")
		}
		myWindow.Resize(myWindow.Content().MinSize())
	}
	radio.SetSelected("代理模式")
	radio.Horizontal = true

	return container.NewVBox(
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作模式: "), radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("转发目标地址: "), remote_addr_box),
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

	keyValidated = widget.NewEntry()
	keyValidated.SetPlaceHolder("请自定义16-32长度字符串")

	localUI := ui2.NewLocalUI(&myWindow)
	localUI_Container := localUI.GetContainer()
	remoteUI := RemoteUI()

	radio := widget.NewRadioGroup([]string{"Remote", "Local"}, nil)
	radio.OnChanged = func(value string) {
		switch value {
		case "Remote":
			localUI_Container.Hide()
			remoteUI.Show()
		case "Local":
			remoteUI.Hide()
			localUI_Container.Show()
		default:
			radio.SetSelected("Local")
		}
		myWindow.Resize(myWindow.Content().MinSize())
	}
	radio.SetSelected("Local")
	radio.Horizontal = true

	log_view := widget.NewLabel("等待启动")
	LogInit(log_view)

	a2 := widget.NewActivity()

	ret := 0

	start_button := widget.NewButton("点击启动", func() {
		switch ret {
		case 0:
			radio.Disable()
			keyValidated.Disable()
			localUI.Disable()
			a2.Start()
			a2.Show()
			log_view.SetText("正在启动...")
			ret = 1
		case 1:
			radio.Enable()
			keyValidated.Enable()
			localUI.Enable()
			a2.Stop()
			a2.Hide()
			log_view.SetText("正在停止...")
			ret = 0
		}
		myWindow.Resize(myWindow.Content().MinSize())
	})
	start_button.Importance = widget.HighImportance

	myWindow.SetContent(container.New(layout.NewVBoxLayout(),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("请选择工作端: "), radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接密钥: "), keyValidated),
		localUI_Container, remoteUI,
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("状态: "), log_view),
		container.NewStack(start_button, a2)))

	myWindow.SetCloseIntercept(func() {
		myWindow.Resize(myWindow.Content().MinSize())
		myWindow.Hide()
	})
	myWindow.ShowAndRun()
}
