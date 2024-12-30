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
	m_button_key_create.Disable()
	m_button_key_paste.Disable()
	m_activity_start_button.Start()
	m_activity_start_button.Show()
	m_view_log.SetText(content)
	m_stats_start_button = 1
}

func enable_other() {
	m_radio_work_type.Enable()
	m_validated_key.Enable()
	m_ui_local.Enable()
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
	var local_addr string

	//先对需要填写的数据进行校验
	switch m_stats_start_button {
	case 0:
		if len(m_validated_key.Text) < 16 {
			SetLogLabel("请输入或点击生成连接密钥!")
			return
		}
		switch m_radio_work_type.Selected {
		case "Local":
			if local_addr, err = m_ui_local.GetLocalAddr(); err != nil {
				SetLogLabel(err.Error())
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
				defer m_mg_start.Done()
				time.Sleep(time.Second * 1)
				m_button_start.Enable()
				if m_stats_start_button == 1 {
					m_button_start.Importance = widget.WarningImportance
					m_button_start.SetText("点击关闭")
				}

				for m_stats_start_button == 1 {
					if pro.GetLocalStats() == 2 {
						m_view_log.SetText("连接成功")
						m_button_start.Importance = widget.SuccessImportance
						m_button_start.Refresh()
						break
					}
					time.Sleep(time.Millisecond * 200)
				}
				m_activity_start_button.Stop()
				m_activity_start_button.Hide()

				if m_stats_start_button == 0 {
					enable_other()
				}
			}()

			m_mg_start.Add(1)
			go func() {
				defer m_mg_start.Done()
				if err := pro.RunLocal(m_ui_local.GetConnType(), local_addr, m_validated_key.Text); err != nil {
					SetLogLabel(err.Error())
				}
				m_stats_start_button = 0
				m_button_start.Importance = widget.HighImportance
				m_button_start.SetText("点击启动")
			}()

		case "Remote":
			m_mg_start.Add(1)
			go func() {
				defer m_mg_start.Done()
				time.Sleep(time.Second * 1)
				m_log_label.SetText("启动成功, 停止需退出程序")
				m_button_start.Importance = widget.SuccessImportance
				m_button_start.SetText("停止需退出程序")
				m_activity_start_button.Stop()
				m_activity_start_button.Hide()

				if m_stats_start_button == 0 {
					enable_other()
				}
			}()

			m_mg_start.Add(1)
			go func() {
				defer m_mg_start.Done()
				if err := pro.RunRemote(remote_addr, m_validated_key.Text, 30*time.Second); err != nil {
					SetLogLabel(err.Error())
				}
				m_stats_start_button = 0
				m_button_start.Importance = widget.HighImportance
				m_button_start.SetText("点击启动")
			}()
		}

	case 1:
		m_button_start.Disable()
		m_view_log.SetText("正在停止...")
		m_stats_start_button = 0

		switch m_radio_work_type.Selected {
		case "Local":
			go func() {
				pro.StopLocal()
				m_mg_start.Wait()
				enable_other()
				m_view_log.SetText("等待连接")
				m_button_start.SetText("点击连接")
				m_button_start.Importance = widget.HighImportance
				m_button_start.Refresh()
				m_button_start.Enable()
			}()
		}
	}
}
