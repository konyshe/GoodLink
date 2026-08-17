package ui2

import (
	"encoding/json"
	"fmt"
	"go2"
	"log"
	"net"
	"time"

	"goodlink/config"
	"goodlink/stun2"
	goodlink_config "goodlink3/config"
)

const (
	goodlinkFileName = "goodlink.json"
	M_APP_TITLE      = "Goodlink"
	workTypeLocal    = "Local"
	workTypeRemote   = "Remote"
)

func Init() {
	m_log_entries = make([]string, 0, maxLogEntries)

	var configInfo goodlink_config.ConfigInfo
	json.Unmarshal(go2.FileReadAll(goodlinkFileName), &configInfo)
	log.Println(configInfo)

	// 如果密钥为空，自动生成密钥
	if len(configInfo.TunKey) == 0 {
		configInfo.TunKey = string(go2.RandomBytes(config.TunKeyByteLen))
		log.Println("自动生成密钥:", configInfo.TunKey)
	}
	if configInfo.WorkType == "" {
		configInfo.WorkType = workTypeLocal
	}

	stateMu.Lock()
	m_work_type = configInfo.WorkType
	m_tun_key = configInfo.TunKey
	stateMu.Unlock()

	m_start_button_state = 0
	updateButtonState(buttonStateInitializing)

	// UI客户端自身的日志输出
	stun2.SetExtraLogSink(func(s string) {
		UILogPrintF(fmt.Sprintf("%s %s", time.Now().Format("2006/01/02 15:04:05"), s))
	})

	startUpgradeCheck()
	go detectNAT()
}

func detectNAT() {
	for {
		conn, err := net.ListenUDP("udp4", nil)
		if err != nil {
			UILogPrintF("NAT检测: UDP监听失败: " + err.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		_, wanPort1, wanPort2, _ := stun2.GetStunIpPort(conn)
		conn.Close()

		SetNATHint(wanPort1 != wanPort2)
		updateButtonState(buttonStateIdle)
		return
	}
}

func GenerateKey() string {
	key := string(go2.RandomBytes(config.TunKeyByteLen))
	stateMu.Lock()
	m_tun_key = key
	stateMu.Unlock()
	broadcastState()
	return key
}
