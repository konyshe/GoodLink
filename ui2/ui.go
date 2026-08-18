package ui2

import (
	"fmt"
	"go2"
	"log"
	"net"
	"time"

	"goodlink/config"
	"goodlink/stun2"
)

const (
	goodlinkFileName = config.UIConfigFileName
	M_APP_TITLE      = "Goodlink"
	workTypeLocal    = "Local"
	workTypeRemote   = "Remote"
)

func Init() {
	m_log_entries = make([]string, 0, maxLogEntries)

	cfg, err := config.LoadUIConfig(goodlinkFileName)
	if err != nil {
		log.Println("读取 goodlink.json 失败:", err)
	} else {
		log.Println(cfg)
	}

	if err := config.NormalizeAndValidate(&cfg); err != nil {
		log.Println("goodlink.json 转发规则无效, 已忽略:", err)
		cfg.ForwardRules = []config.UIForwardRule{}
		if cfg.LocalMode != config.LocalModeTUN && cfg.LocalMode != config.LocalModeForward {
			cfg.LocalMode = config.LocalModeTUN
		}
	}

	// 如果密钥为空，自动生成密钥
	if len(cfg.TunKey) == 0 {
		cfg.TunKey = string(go2.RandomBytes(config.TunKeyByteLen))
		log.Println("自动生成密钥:", cfg.TunKey)
	}
	if cfg.WorkType == "" {
		cfg.WorkType = workTypeLocal
	}

	stateMu.Lock()
	m_work_type = cfg.WorkType
	m_tun_key = cfg.TunKey
	m_local_mode = cfg.LocalMode
	m_forward_rules = cfg.ForwardRules
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
