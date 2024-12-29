//go:build windows

package main

import (
	"gogo"
	"log"

	"goodlink/theme"
	"goodlink/tools"
	"goodlink/ui2"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
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
	keyValidated.SetPlaceHolder("自定义16-32字节长度")

	key_create_button := widget.NewButton("生成密钥", func() {
		keyValidated.SetText(tools.RandomString(27))
	})
	key_copy_button := widget.NewButton("复制密钥", func() {
		clipboard.WriteAll(keyValidated.Text)
	})
	key_paste_button := widget.NewButton("粘贴密钥", func() {
		if s, err := clipboard.ReadAll(); err == nil {
			keyValidated.SetText(s)
		}
	})

	localUI := ui2.NewLocalUI(&myWindow)
	localUI_Container := localUI.GetContainer()

	remoteUI := ui2.NewRemoteUI(&myWindow)
	remoteUI_Container := remoteUI.GetContainer()

	radio := widget.NewRadioGroup([]string{"Remote", "Local"}, nil)
	radio.OnChanged = func(value string) {
		switch value {
		case "Remote":
			localUI_Container.Hide()
			remoteUI_Container.Show()
		case "Local":
			remoteUI_Container.Hide()
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

	start_button_stats := 0
	start_button_activity := widget.NewActivity()
	start_button := widget.NewButton("点击启动", func() {
		switch start_button_stats {
		case 0:
			radio.Disable()
			keyValidated.Disable()
			localUI.Disable()
			key_create_button.Disable()
			key_paste_button.Disable()
			start_button_activity.Start()
			start_button_activity.Show()
			log_view.SetText("正在启动...")
			start_button_stats = 1
		case 1:
			radio.Enable()
			keyValidated.Enable()
			localUI.Enable()
			key_create_button.Enable()
			key_paste_button.Enable()
			start_button_activity.Stop()
			start_button_activity.Hide()
			log_view.SetText("正在停止...")
			start_button_stats = 0
		}
		myWindow.Resize(myWindow.Content().MinSize())
	})
	start_button.Importance = widget.HighImportance

	myWindow.SetContent(container.New(layout.NewVBoxLayout(),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("选择工作端: "), radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接密钥: "), keyValidated),
		container.NewGridWithColumns(3, key_create_button, key_copy_button, key_paste_button),
		localUI_Container, remoteUI_Container,
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("状态: "), log_view),
		container.NewStack(start_button, start_button_activity)))

	myWindow.SetCloseIntercept(func() {
		myWindow.Resize(myWindow.Content().MinSize())
		myWindow.Hide()
	})
	myWindow.ShowAndRun()
}
