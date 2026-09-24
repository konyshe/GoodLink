package ui2

import (
	"fmt"
	"go2"
	"log"
	"net"
	"strings"
	"time"

	"goodlink/config"
	"goodlink/stun2"
)

const (
	M_APP_TITLE    = "Goodlink"
	workTypeLocal  = "Local"
	workTypeRemote = "Remote"
)

func Init() {
	m_shared_logs = make([]string, 0, maxLogEntries)

	cfg, err := config.LoadUIConfig(config.UIConfigPath())
	if err != nil {
		log.Println("读取 goodlink.json 失败:", err)
	}
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		log.Println("goodlink.json 无效, 已忽略无效项:", err)
		cfg = config.UIConfig{Services: []config.UIService{}}
	}
	if len(cfg.Services) == 0 {
		cfg.Services = []config.UIService{{
			Name:         "服务1",
			WorkType:     workTypeLocal,
			LocalMode:    config.LocalModeTUN,
			Transport:    config.TransportKCP,
			TunIP:        "192.17.19.1",
			ForwardRules: []config.UIForwardRule{},
		}}
	}
	for i := range cfg.Services {
		if len(cfg.Services[i].TunKey) == 0 {
			cfg.Services[i].TunKey = string(go2.RandomBytes(config.TunKeyByteLen))
		}
		if cfg.Services[i].WorkType == "" {
			cfg.Services[i].WorkType = workTypeLocal
		}
	}
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		log.Println("初始化服务配置失败:", err)
	}
	_ = config.SaveUIConfig(config.UIConfigPath(), cfg)
	replaceServices(cfg.Services)
	for _, svc := range m_services {
		svc.status = StatusDetecting
	}

	stun2.SetExtraLogSink(func(s string) {
		UILogPrintF(fmt.Sprintf("%s %s", time.Now().Format("2006/01/02 15:04:05"), s))
	})

	cleanupUpgradeResidues()
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
		stateMu.Lock()
		for _, svc := range m_services {
			if svc.status == StatusDetecting {
				svc.status = StatusIdle
			}
		}
		stateMu.Unlock()
		broadcastState()
		tryAutoStart()
		return
	}
}

func GenerateKey() string {
	return string(go2.RandomBytes(config.TunKeyByteLen))
}

func currentUIConfig() config.UIConfig {
	return allServicesConfig()
}

func HandleImportConfig(cfg config.UIConfig) error {
	if anyServiceActive() {
		return fmt.Errorf("busy")
	}
	if len(cfg.Services) == 0 && legacyImport(&cfg) {
		// migrated inside Load-style path by caller; fall through
	}
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	for i := range cfg.Services {
		wt := strings.TrimSpace(cfg.Services[i].WorkType)
		if wt == "" {
			wt = workTypeLocal
		}
		if wt != workTypeLocal && wt != workTypeRemote {
			return fmt.Errorf("无效的工作端侧: %s", wt)
		}
		cfg.Services[i].WorkType = wt
		if len(cfg.Services[i].TunKey) == 0 {
			cfg.Services[i].TunKey = string(go2.RandomBytes(config.TunKeyByteLen))
		}
	}
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		return err
	}
	replaceServices(cfg.Services)
	stateMu.Lock()
	for _, svc := range m_services {
		svc.status = StatusIdle
	}
	stateMu.Unlock()
	broadcastState()
	UILogPrintF("已导入配置")
	return nil
}

func legacyImport(cfg *config.UIConfig) bool {
	return cfg != nil && len(cfg.Services) == 0
}
