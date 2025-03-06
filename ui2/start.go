//go:build windows

package ui2

import (
	"encoding/json"
	"gogo"
	"log"
	"sync"
	"time"

	"goodlink/config"
	"goodlink/pro"
	_ "goodlink/pro"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2/widget"
)

var (
	m_mg_start              sync.WaitGroup
	m_lock_start            sync.Mutex
	m_button_start          *widget.Button
	m_activity_start_button *widget.Activity
	m_stats_start_button    int
)

func disable_other(content string) {
	m_radio_work_type.Disable()
	m_validated_key.Disable()
	m_ui_local.Disable()
	m_ui_remote.Disable()
	m_button_key_create.Disable()
	m_button_key_paste.Disable()
	m_activity_start_button.Start()
	m_activity_start_button.Show()
	UILogPrintF(content)
	m_stats_start_button = 1
}

func enable_other() {
	m_radio_work_type.Enable()
	m_validated_key.Enable()
	m_ui_local.Enable()
	m_ui_remote.Enable()
	m_button_key_create.Enable()
	m_button_key_paste.Enable()
	m_activity_start_button.Stop()
	m_activity_start_button.Hide()
}

func start_button_click() {
	m_lock_start.Lock()
	defer m_lock_start.Unlock()

	var err error
	var remote_addr string

	//先对需要填写的数据进行校验
	switch m_stats_start_button {
	case 0:
		if len(m_validated_key.Text) < 16 {
			SetLogLabel("请输入或点击生成连接密钥!")
			return
		}
		switch m_radio_work_type.Selected {
		case "Local":
			if m_ui_local.GetLocalPort() == "" {
				SetLogLabel("请填写访问端口号")
				return
			}
		case "Remote":
			switch m_ui_remote.GetRemoteType() {
			case "代理模式":
				remote_addr = "" //代理模式, 这里必须设置为空
			case "转发模式":
				if remote_addr, err = m_ui_remote.GetRemoteAddr(); err != nil {
					SetLogLabel(err.Error())
					return
				}
			}
		}

		// 保存配置文件, 下次启动加载
		configByte, _ := json.Marshal(&config.ConfigInfo{
			WorkType:   m_radio_work_type.Selected,
			TunKey:     m_validated_key.Text,
			ConnType:   m_ui_local.GetConnType2(),
			LocalIP:    m_ui_local.GetLocalIP(),
			LocalPort:  m_ui_local.GetLocalPort(),
			RemoteType: m_ui_remote.GetRemoteType(),
			RemoteIP:   m_ui_remote.GetRemoteIP(),
			RemotePort: m_ui_remote.GetRemotePort(),
		})
		log.Println(string(configByte))
		gogo.Utils().FileDel("goodlink.json")
		gogo.Utils().FileAppend("goodlink.json", configByte)
	}

	switch m_stats_start_button {
	case 0:
		m_button_start.Disable()
		disable_other("正在启动...")

		switch m_radio_work_type.Selected {
		case "Local":
			m_mg_start.Add(1)
			go func() {
				defer func() {
					m_activity_start_button.Stop()
					m_activity_start_button.Hide()
					m_mg_start.Done()
				}()

				time.Sleep(time.Second * 1)
				if m_stats_start_button != 1 {
					return
				}
				m_button_start.Enable()
				m_button_start.Importance = widget.WarningImportance
				m_button_start.SetText("关闭连接")

				for m_stats_start_button == 1 {
					switch pro.GetLocalStats() {
					case 1:
						m_activity_start_button.Start()
						m_activity_start_button.Show()
						m_button_start.Importance = widget.WarningImportance
						m_button_start.Refresh()
					case 2:
						m_activity_start_button.Stop()
						m_activity_start_button.Hide()
						m_button_start.Importance = widget.SuccessImportance
						m_button_start.Refresh()
					}
					time.Sleep(time.Second * 1)
				}
			}()

			m_mg_start.Add(1)
			go func() {
				defer m_mg_start.Done()
				if err := pro.RunLocal(m_ui_local.GetConnType(), "0.0.0.0:"+m_ui_local.GetLocalPort(), m_validated_key.Text); err != nil {
					SetLogLabel(err.Error())
				}
				m_stats_start_button = 0
				m_button_start.Importance = widget.HighImportance
				m_button_start.SetText("点击启动")
				m_button_start.Enable()
				enable_other()
			}()

		case "Remote":
			m_mg_start.Add(1)
			go func() {
				defer m_mg_start.Done()
				time.Sleep(time.Second * 1)
				m_log_label.SetText("启动成功")
				m_button_start.Importance = widget.SuccessImportance
				m_button_start.SetText("点击停止")
				m_activity_start_button.Stop()
				m_activity_start_button.Hide()
				m_button_start.Enable()
			}()

			m_mg_start.Add(1)
			go func() {
				defer m_mg_start.Done()
				if err := pro.RunRemote(remote_addr, m_validated_key.Text); err != nil {
					SetLogLabel(err.Error())
				}
				m_stats_start_button = 0
				m_button_start.Importance = widget.HighImportance
				m_button_start.SetText("点击启动")
				enable_other()
			}()
		}

	case 1:
		m_button_start.Disable()
		UILogPrintF("正在停止...")
		m_stats_start_button = 0

		switch m_radio_work_type.Selected {
		case "Local":
			go func() {
				pro.StopLocal()
			}()

		case "Remote":
			go func() {
				pro.StopRemote()
			}()
		}

		m_mg_start.Wait()
		enable_other()
		UILogPrintF("等待启动")
		m_button_start.Importance = widget.HighImportance
		m_button_start.SetText("点击启动")
		m_button_start.Enable()
	}
}
