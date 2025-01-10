//go:build windows

package ui2

import (
	"encoding/json"
	"errors"
	"fmt"
	"gogo"
	"log"
	"time"

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
	m_radio_work_type   *widget.RadioGroup
	m_validated_key     *widget.Entry
	m_ui_local          *LocalUI
	m_ui_remote         *RemoteUI
	m_button_key_create *widget.Button
	m_button_key_paste  *widget.Button
	m_view_log          *LogLabel
	m_view_time         widget.Label
)

const (
	M_APP_TITLE = "GoodLink"
)

func UILogPrintF(a ...any) {
	var content string
	if len(a) > 1 {
		content = fmt.Sprintf(a[0].(string), a[1:]...)
	} else {
		content = a[0].(string)
	}
	log.Println(content)

	if len(content) > 32 {
		content = content[:32]
	}
	SetLogLabel(content)

	m_view_time.SetText(time.Now().Format("2006/01/02 15:04:05"))
}

func UILogInit() {
	m_view_log = NewLogLabel("等待启动")

	gogo.Log().RegistInfo(func(content string) {
		UILogPrintF(content)
	})
	gogo.Log().RegistDebug(func(content string) {
		UILogPrintF(content)
	})
	gogo.Log().RegistError(func(content string) {
		UILogPrintF(content)
		fyne.LogError("error: ", errors.New(content))
	})
}

func GetMainUI(myWindow *fyne.Window) *fyne.Container {
	var configInfo config.ConfigInfo
	json.Unmarshal(gogo.Utils().FileReadAll("goodlink.json"), &configInfo)
	log.Println(configInfo)

	m_validated_key = widget.NewEntry()
	m_validated_key.SetPlaceHolder("自定义16-24字节长度")
	if len(configInfo.TunKey) > 0 {
		m_validated_key.SetText(configInfo.TunKey)
	}

	m_button_key_create = widget.NewButton("生成密钥", func() {
		m_validated_key.SetText(tools.RandomString(24))
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
	localUI_Container := m_ui_local.GetContainer()

	m_ui_remote = NewRemoteUI(myWindow, &configInfo)
	remoteUI_Container := m_ui_remote.GetContainer()

	m_radio_work_type = widget.NewRadioGroup([]string{"Remote", "Local"}, nil)
	m_radio_work_type.Horizontal = true
	m_radio_work_type.OnChanged = func(value string) {
		switch value {
		case "Remote":
			localUI_Container.Hide()
			remoteUI_Container.Show()
			(*myWindow).Resize((*myWindow).Content().MinSize())
		default:
			remoteUI_Container.Hide()
			localUI_Container.Show()
			(*myWindow).Resize((*myWindow).Content().MinSize())
		}
	}
	if configInfo.WorkType == "" {
		configInfo.WorkType = "Local"
	}
	m_radio_work_type.SetSelected(configInfo.WorkType)

	UILogInit()

	m_stats_start_button = 0
	m_activity_start_button = widget.NewActivity()
	m_button_start = widget.NewButton("点击启动", start_button_click)
	m_button_start.Importance = widget.HighImportance
	m_button_start.Resize(fyne.NewSize(100, 40))
	m_button_start.Disable()
	go func() {
		if err := pro.Init(config.Arg_redis_addr, config.Arg_redis_pass, config.Arg_redis_id); err != nil {
			UILogPrintF(err.Error())
			return
		}
		m_button_start.Enable()
	}()

	return container.New(layout.NewVBoxLayout(),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("工作端侧: "), m_radio_work_type),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("连接密钥: "), m_validated_key),
		container.NewGridWithColumns(3, m_button_key_create, key_copy_button, m_button_key_paste),
		localUI_Container, remoteUI_Container,
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("更新状态: "), m_view_log),
		container.New(layout.NewFormLayout(), widget.NewRichTextWithText("更新时间: "), &m_view_time),
		container.NewStack(m_button_start, m_activity_start_button))
}
