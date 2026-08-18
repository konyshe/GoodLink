package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const (
	UIConfigFileName = "goodlink.json"
	LocalModeTUN     = "tun"
	LocalModeForward = "forward"
)

type UIForwardRule struct {
	Proto  string `json:"proto"`
	Listen string `json:"listen"`
	Remote string `json:"remote"`
}

type UIConfig struct {
	WorkType     string          `json:"work_type"`
	TunKey       string          `json:"tun_key"`
	LocalMode    string          `json:"local_mode"`
	ForwardRules []UIForwardRule `json:"forward_rules"`
}

func FromUI() bool {
	return Arg_ui != nil && *Arg_ui
}

func LoadUIConfig(path string) (UIConfig, error) {
	cfg := UIConfig{
		LocalMode:    LocalModeTUN,
		ForwardRules: []UIForwardRule{},
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.LocalMode == "" {
		cfg.LocalMode = LocalModeTUN
	}
	if cfg.ForwardRules == nil {
		cfg.ForwardRules = []UIForwardRule{}
	}
	return cfg, nil
}

func SaveUIConfig(path string, cfg UIConfig) error {
	if err := NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func NormalizeAndValidate(cfg *UIConfig) error {
	if cfg == nil {
		return fmt.Errorf("配置为空")
	}
	cfg.LocalMode = strings.ToLower(strings.TrimSpace(cfg.LocalMode))
	if cfg.LocalMode == "" {
		cfg.LocalMode = LocalModeTUN
	}
	if cfg.LocalMode != LocalModeTUN && cfg.LocalMode != LocalModeForward {
		return fmt.Errorf("无效的工作模式: %s", cfg.LocalMode)
	}
	if cfg.ForwardRules == nil {
		cfg.ForwardRules = []UIForwardRule{}
	}

	seen := make(map[string]struct{}, len(cfg.ForwardRules))
	out := make([]UIForwardRule, 0, len(cfg.ForwardRules))
	for i, r := range cfg.ForwardRules {
		proto := strings.ToLower(strings.TrimSpace(r.Proto))
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("第%d条规则协议无效", i+1)
		}
		listenHost, listenPort, err := parseHostPortIPv4(r.Listen)
		if err != nil {
			return fmt.Errorf("第%d条规则本地地址: %v", i+1, err)
		}
		remoteHost, remotePort, err := parseHostPortIPv4(r.Remote)
		if err != nil {
			return fmt.Errorf("第%d条规则远端地址: %v", i+1, err)
		}
		listen := net.JoinHostPort(listenHost, strconv.Itoa(listenPort))
		key := proto + "|" + listen
		if _, ok := seen[key]; ok {
			return fmt.Errorf("第%d条规则监听地址重复: %s %s", i+1, proto, listen)
		}
		seen[key] = struct{}{}
		out = append(out, UIForwardRule{
			Proto:  proto,
			Listen: listen,
			Remote: net.JoinHostPort(remoteHost, strconv.Itoa(remotePort)),
		})
	}
	cfg.ForwardRules = out
	return nil
}

func (c UIConfig) ForwardFingerprint() string {
	b, _ := json.Marshal(c.ForwardRules)
	return c.LocalMode + "\n" + string(b)
}

func parseHostPortIPv4(addr string) (string, int, error) {
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return "", 0, err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.To4() == nil {
		return "", 0, fmt.Errorf("必须是IPv4地址: %s", host)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("端口无效: %s", portStr)
	}
	return ip.To4().String(), port, nil
}
