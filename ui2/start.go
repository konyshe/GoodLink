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
	m_start_mg              sync.WaitGroup
	m_start_lock            sync.Mutex
	m_start_button          *widget.Button
	m_start_button_activity *widget.Activity
	m_start_button_stats    int
)

func start_button_click_1(content string) {
	m_start_button.Disable()
	m_radio.Disable()
	m_key_validated.Disable()
	m_local_ui.Disable()
	m_key_create_button.Disable()
	m_key_paste_button.Disable()
	m_start_button_activity.Start()
	m_start_button_activity.Show()
	m_log_view.SetText(content)
	m_start_button_stats = 1
}

func start_button_click_0() {
	m_radio.Enable()
	m_key_validated.Enable()
	m_local_ui.Enable()
	m_key_create_button.Enable()
	m_key_paste_button.Enable()
	m_start_button_activity.Stop()
	m_start_button_activity.Hide()
}

func start_button_click() {
	m_start_lock.Lock()
	defer m_start_lock.Unlock()

	var err error
	var remote_addr string
	var local_addr string

	//先对需要填写的数据进行校验
	switch m_start_button_stats {
	case 0:
		if len(m_key_validated.Text) < 16 {
			SetLogLabel("请输入或点击生成连接密钥!")
			return
		}
		switch m_radio.Selected {
		case "Local":
			if local_addr, err = m_local_ui.GetLocalAddr(); err != nil {
				SetLogLabel(err.Error())
				return
			}
		case "Remote":
			if remote_addr, err = m_remote_ui.GetRemoteAddr(); err != nil {
				SetLogLabel(err.Error())
				return
			}
		}

		log.Println(local_addr)

		configByte, _ := json.Marshal(&config.ConfigInfo{
			WorkType:   m_radio.Selected,
			TunKey:     m_key_validated.Text,
			ConnType:   m_local_ui.GetConnType2(),
			LocalIP:    m_local_ui.GetLocalIP(),
			LocalPort:  m_local_ui.GetLocalPort(),
			RemoteType: m_remote_ui.GetRemoteType(),
			RemoteIP:   m_remote_ui.GetRemoteIP(),
			RemotePort: m_remote_ui.GetRemotePort(),
		})
		log.Println(string(configByte))
		gogo.Utils().FileDel("goodlink.json")
		gogo.Utils().FileAppend("goodlink.json", configByte)
	}

	switch m_start_button_stats {
	case 0:
		start_button_click_1("正在启动...")

		go func() {
			time.Sleep(time.Second * 1)
			m_start_button.Enable()

			for m_start_button_stats == 1 {
				if pro.GetLocalStats() == 2 {
					m_log_view.SetText("连接成功")
					break
				}
				time.Sleep(time.Millisecond * 200)
			}
		}()

		switch m_radio.Selected {
		case "Local":
			m_start_mg.Add(1)
			go func() {
				defer m_start_mg.Done()
				if err := pro.RunLocal(m_local_ui.GetConnType(), remote_addr, m_key_validated.Text); err != nil {
					SetLogLabel(err.Error())
					m_start_button_stats = 0
					return
				}
			}()
		}

	case 1:
		m_start_button.Disable()
		m_log_view.SetText("正在停止...")
		m_start_button_stats = 0

		switch m_radio.Selected {
		case "Local":
			go func() {
				pro.StopLocal()
				m_start_mg.Wait()
				start_button_click_0()
			}()
		}
	}
}
