//go:build windows

package ui2

import (
	"encoding/json"
	"errors"
	"gogo"
	"log"

	"goodlink/config"
	"goodlink/pro"
	_ "goodlink/pro"
	"goodlink/tools"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
)

var (
	m_radio             *widget.RadioGroup
	m_key_validated     *widget.Entry
	m_local_ui          *LocalUI
	m_remote_ui         *RemoteUI
	m_key_create_button *widget.Button
	m_key_paste_button  *widget.Button
	m_log_view          *LogLabel
)

const (
	M_APP_TITLE = "GoodLink"
)

func LogInit(m_log_view *LogLabel) {
	gogo.Log().RegistInfo(func(content string) {
		SetLogLabel(content)
		log.Println(content)
	})
	gogo.Log().RegistDebug(func(content string) {
		SetLogLabel(content)
	})
	gogo.Log().RegistError(func(content string) {
		SetLogLabel(content)
		fyne.LogError("error: ", errors.New(content))
	})
}

func GetMainUI(myWindow *fyne.Window) *fyne.Container {
	var configInfo config.ConfigInfo
	json.Unmarshal(gogo.Utils().FileReadAll("goodlink.json"), &configInfo)
	log.Println(configInfo)

	m_key_validated = widget.NewEntry()
	m_key_validated.SetPlaceHolder("自定义16-24字节长度")
	if len(configInfo.TunKey) > 0 {
		m_key_validated.SetText(configInfo.TunKey)
	}

	m_key_create_button = widget.NewButton("生成密钥", func() {
		m_key_validated.SetText(tools.RandomString(24))
	})
	key_copy_button := widget.NewButton("复制密钥", func() {
		clipboard.WriteAll(m_key_validated.Text)
	})
	m_key_paste_button = widget.NewButton("粘贴密钥", func() {
		if s, err := clipboard.ReadAll(); err == nil {
			m_key_validated.SetText(s)
		}
	})

	m_local_ui = NewLocalUI(myWindow, &configInfo)
	localUI_Container := m_local_ui.GetContainer()

	m_remote_ui = NewRemoteUI(myWindow, &configInfo)
	remoteUI_Container := m_remote_ui.GetContainer()

	m_radio = widget.NewRadioGroup([]string{"Remote", "Local"}, nil)
	m_radio.Horizontal = true
	m_radio.OnChanged = func(value string) {
		switch value {
		case "Remote":
			localUI_Container.Hide()
			remoteUI_Container.Show()
		case "Local":
			remoteUI_Container.Hide()
			localUI_Container.Show()
		default:
			m_radio.SetSelected("Local")
		}
	}
	if len(configInfo.WorkType) > 0 {
		m_radio.SetSelected(configInfo.WorkType)
	} else {
		m_radio.SetSelected("Local")
	}

	m_log_view = NewLogLabel("等待启动")
	LogInit(m_log_view)

	m_start_button_stats = 0
	m_start_button_activity = widget.NewActivity()
	m_start_button = widget.NewButton("点击启动", start_button_click)
	m_start_button.Importance = widget.HighImportance
	m_start_button.Resize(fyne.NewSize(100, 40))
	m_start_button.Disable()
	go func() {
		pro.Init("", "", 0)
		m_start_button.Enable()
	}()

	return container.New(layout.NewVBoxLayout(),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作端侧: "), m_radio),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接密钥: "), m_key_validated),
		container.NewGridWithColumns(3, m_key_create_button, key_copy_button, m_key_paste_button),
		localUI_Container, remoteUI_Container,
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("当前状态: "), m_log_view),
		container.NewStack(m_start_button, m_start_button_activity))
}
