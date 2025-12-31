//go:build windows

package ui2

import (
	"encoding/json"
	"go2"
	"log"

	"goodlink/config"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/atotto/clipboard"
)

var (
	m_radio_work_type   *widget.RadioGroup
	m_validated_key     *widget.Entry
	m_ui_local          *LocalUI
	m_ui_remote         *RemoteUI
	m_button_key_create *widget.Button
	m_button_key_paste  *widget.Button
)

const (
	M_APP_TITLE = "GoodLink"
)

func GetMainUI(myWindow *fyne.Window) *fyne.Container {
	var configInfo config.ConfigInfo
	json.Unmarshal(go2.FileReadAll("goodlink.json"), &configInfo)
	log.Println(configInfo)

	m_validated_key = widget.NewEntry()
	m_validated_key.SetPlaceHolder("自定义16-24字节长度")
	if len(configInfo.TunKey) > 0 {
		m_validated_key.SetText(configInfo.TunKey)
	}

	m_button_key_create = widget.NewButton("生成密钥", func() {
		m_validated_key.SetText(string(go2.RandomBytes(24)))
	})
	key_copy_button := widget.NewButton("复制密钥", func() {
		clipboard.WriteAll(m_validated_key.Text)
	})
	m_button_key_paste = widget.NewButton("粘贴密钥", func() {
		if s, err := clipboard.ReadAll(); err == nil {
			m_validated_key.SetText(s)
		}
	})

	m_ui_local = NewLocalUI(myWindow, &configInfo)
	//localUI_Container := m_ui_local.GetContainer()

	m_ui_remote = NewRemoteUI(myWindow, &configInfo)
	//remoteUI_Container := m_ui_remote.GetContainer()

	m_radio_work_type = widget.NewRadioGroup([]string{"Remote", "Local"}, nil)
	m_radio_work_type.Horizontal = true
	/*
		m_radio_work_type.OnChanged = func(value string) {
			switch value {
			case "Remote":
				localUI_Container.Hide()
				remoteUI_Container.Show()
			default:
				remoteUI_Container.Hide()
				localUI_Container.Show()
			}
			(*myWindow).Resize((*myWindow).Content().MinSize())
		}
	*/
	if configInfo.WorkType == "" {
		configInfo.WorkType = "Local"
	}
	m_radio_work_type.SetSelected(configInfo.WorkType)

	m_stats_start_button = 0
	m_activity_start_button = widget.NewActivity()
	m_button_start = widget.NewButton("点击启动", start_button_click)
	m_button_start.Importance = widget.HighImportance

	// 初始化日志标签（用于兼容，但不再显示在UI中）
	NewLogLabel("等待启动")

	return container.New(layout.NewVBoxLayout(),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作端侧: "), m_radio_work_type),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接密钥: "), m_validated_key),
		container.NewGridWithColumns(3, m_button_key_create, key_copy_button, m_button_key_paste),
		//localUI_Container, remoteUI_Container,
		//widget.NewLabel("日志显示:"),
		NewLogList(),
		container.NewStack(m_button_start, m_activity_start_button),
		NewFooter())
}
