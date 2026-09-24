package config

import (
	"encoding/json"
	"os"
)

const RuntimeConfEnv = "GOODLINK_RUNTIME_CONF"

// RuntimeRedis 与 goodlink3 Redis 字段对齐, 只保留子进程需要的部分。
type RuntimeRedis struct {
	Addr    string `json:"addr"`
	TlsAddr string `json:"tls_addr"`
	Passwd  string `json:"passwd"`
	ID      int    `json:"id"`
}

// RuntimeConf 是主进程下发给子进程的运行时配置。
type RuntimeConf struct {
	Redis    RuntimeRedis `json:"redis"`
	StunList []string     `json:"stun_list"`
}

func ExportRuntimeConfEnv(conf RuntimeConf) string {
	b, err := json.Marshal(conf)
	if err != nil {
		return RuntimeConfEnv + "="
	}
	return RuntimeConfEnv + "=" + string(b)
}

func LoadRuntimeConfFromEnv() (RuntimeConf, bool) {
	raw := os.Getenv(RuntimeConfEnv)
	if raw == "" {
		return RuntimeConf{}, false
	}
	var conf RuntimeConf
	if err := json.Unmarshal([]byte(raw), &conf); err != nil {
		return RuntimeConf{}, false
	}
	return conf, true
}
