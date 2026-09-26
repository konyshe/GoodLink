package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

const (
	UIConfigFileName = "goodlink.json"
	LocalModeTUN     = "tun"
	LocalModeForward = "forward"
	TransportQUIC    = "quic"
	TransportKCP     = "kcp"

	maxServiceNameRunes = 32
	tunIPPrefix         = "192.17."
	tunSegmentMin       = 19
	tunSegmentMax       = 254
)

type UIForwardRule struct {
	Proto  string `json:"proto"`
	Listen string `json:"listen"`
	Remote string `json:"remote"`
}

type UIService struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	WorkType        string          `json:"work_type"`
	TunKey          string          `json:"tun_key"`
	LocalMode       string          `json:"local_mode"`
	Transport       string          `json:"transport"`
	TunIP           string          `json:"tun_ip"`
	AutoStart       bool            `json:"auto_start"`
	ForwardRules    []UIForwardRule `json:"forward_rules"`
	AllowLocalProxy *bool           `json:"allow_local_proxy,omitempty"`
}

// LocalProxyAllowed 旧配置没有该字段时视为允许 Local 开启代理。
func (s UIService) LocalProxyAllowed() bool {
	return s.AllowLocalProxy == nil || *s.AllowLocalProxy
}

type UIConfig struct {
	Services []UIService `json:"services"`
}

// legacyUIConfig 用于识别升级前的扁平 goodlink.json。
type legacyUIConfig struct {
	Services     []UIService     `json:"services"`
	WorkType     string          `json:"work_type"`
	TunKey       string          `json:"tun_key"`
	LocalMode    string          `json:"local_mode"`
	Transport    string          `json:"transport"`
	AutoStart    bool            `json:"auto_start"`
	ForwardRules []UIForwardRule `json:"forward_rules"`
}

func FromUI() bool {
	return Arg_ui != nil && *Arg_ui
}

var uiConfigPathOnce sync.Once
var uiConfigPath string

// UIConfigPath 返回程序所在目录下的 goodlink.json 完整路径。
func UIConfigPath() string {
	uiConfigPathOnce.Do(func() {
		exe, err := os.Executable()
		if err != nil {
			uiConfigPath = UIConfigFileName
			return
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		uiConfigPath = filepath.Join(filepath.Dir(exe), UIConfigFileName)
	})
	return uiConfigPath
}

func newServiceID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return hex.EncodeToString(b)
}

func LoadUIConfig(path string) (UIConfig, error) {
	cfg := UIConfig{Services: []UIService{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return cfg, nil
	}
	var raw legacyUIConfig
	if err := json.Unmarshal(data, &raw); err != nil {
		return cfg, err
	}
	if len(raw.Services) > 0 {
		cfg.Services = raw.Services
		return cfg, nil
	}
	if legacyHasValue(raw) {
		cfg.Services = []UIService{{
			ID:           newServiceID(),
			Name:         "服务1",
			WorkType:     raw.WorkType,
			TunKey:       raw.TunKey,
			LocalMode:    raw.LocalMode,
			Transport:    raw.Transport,
			TunIP:        "192.17.19.1",
			AutoStart:    raw.AutoStart,
			ForwardRules: raw.ForwardRules,
		}}
	}
	return cfg, nil
}

func legacyHasValue(raw legacyUIConfig) bool {
	return strings.TrimSpace(raw.WorkType) != "" ||
		strings.TrimSpace(raw.TunKey) != "" ||
		strings.TrimSpace(raw.LocalMode) != "" ||
		strings.TrimSpace(raw.Transport) != "" ||
		raw.AutoStart ||
		len(raw.ForwardRules) > 0
}

func SaveUIConfig(path string, cfg UIConfig) error {
	if err := NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		// 重命名失败时不能删除 path 后重试: 一旦二次重命名仍失败,
		// 用户原有的 goodlink.json 会被彻底删除且无法恢复。
		// 改为覆盖写入已有文件, 失败时至少保留原文件内容。
		if err2 := copyFileOverwrite(tmp, path); err2 != nil {
			_ = os.Remove(tmp)
			return fmt.Errorf("保存配置失败: %v(覆盖写入失败: %v)", err, err2)
		}
		_ = os.Remove(tmp)
	}
	return nil
}

// copyFileOverwrite 用 src 的内容覆盖 dst, 用于 os.Rename 不可用时的降级路径。
func copyFileOverwrite(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func NormalizeAndValidate(cfg *UIConfig) error {
	if cfg == nil {
		return fmt.Errorf("配置为空")
	}
	if cfg.Services == nil {
		cfg.Services = []UIService{}
	}

	ids := make(map[string]struct{}, len(cfg.Services))
	names := make(map[string]struct{}, len(cfg.Services))
	keys := make(map[string]string, len(cfg.Services))
	ips := make(map[string]struct{}, len(cfg.Services))
	listens := make(map[string]string)

	for i := range cfg.Services {
		svc := &cfg.Services[i]
		label := fmt.Sprintf("服务%d", i+1)

		svc.ID = strings.TrimSpace(svc.ID)
		if svc.ID == "" {
			svc.ID = newServiceID()
		}
		if _, ok := ids[svc.ID]; ok {
			return fmt.Errorf("服务ID重复: %s", svc.ID)
		}
		ids[svc.ID] = struct{}{}

		name, err := normalizeServiceName(svc.Name, names, i)
		if err != nil {
			return err
		}
		svc.Name = name
		names[name] = struct{}{}
		label = name

		svc.WorkType = strings.TrimSpace(svc.WorkType)
		svc.TunKey = strings.TrimSpace(svc.TunKey)
		if svc.TunKey != "" {
			if other, ok := keys[svc.TunKey]; ok {
				return fmt.Errorf("连接密钥与服务[%s]重复", other)
			}
			keys[svc.TunKey] = label
		}

		if err := normalizeServiceMode(svc, label); err != nil {
			return err
		}

		if strings.TrimSpace(svc.TunIP) == "" {
			svc.TunIP = AllocateTunIP(*cfg)
		}
		if err := validateTunIP(svc.TunIP); err != nil {
			return fmt.Errorf("服务[%s] %v", label, err)
		}
		if _, ok := ips[svc.TunIP]; ok {
			return fmt.Errorf("服务[%s] 网段地址重复: %s", label, svc.TunIP)
		}
		ips[svc.TunIP] = struct{}{}

		if svc.LocalMode == LocalModeForward {
			for _, r := range svc.ForwardRules {
				key := listenRuleKey(r)
				if other, ok := listens[key]; ok && other != svc.ID {
					return fmt.Errorf("服务[%s] 监听地址与服务[%s]冲突: %s %s", label, serviceNameByID(*cfg, other), strings.ToUpper(r.Proto), r.Listen)
				}
				listens[key] = svc.ID
			}
		}
	}
	keepAutoStart := false
	for i := range cfg.Services {
		if !cfg.Services[i].AutoStart {
			continue
		}
		if keepAutoStart {
			cfg.Services[i].AutoStart = false
			continue
		}
		keepAutoStart = true
	}
	return nil
}

func normalizeServiceName(name string, used map[string]struct{}, index int) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		for n := index + 1; ; n++ {
			candidate := fmt.Sprintf("服务%d", n)
			if _, ok := used[candidate]; !ok {
				return candidate, nil
			}
		}
	}
	if utf8.RuneCountInString(name) > maxServiceNameRunes {
		return "", fmt.Errorf("服务名称超过%d个字符: %s", maxServiceNameRunes, name)
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("服务名称包含非法字符")
		}
	}
	if _, ok := used[name]; ok {
		return "", fmt.Errorf("服务名称重复: %s", name)
	}
	return name, nil
}

func normalizeServiceMode(svc *UIService, label string) error {
	svc.LocalMode = strings.ToLower(strings.TrimSpace(svc.LocalMode))
	if svc.LocalMode == "" {
		svc.LocalMode = LocalModeTUN
	}
	if svc.LocalMode != LocalModeTUN && svc.LocalMode != LocalModeForward {
		return fmt.Errorf("服务[%s] 无效的工作模式: %s", label, svc.LocalMode)
	}
	svc.Transport = NormalizeTransport(svc.Transport)
	if svc.Transport != TransportQUIC && svc.Transport != TransportKCP {
		return fmt.Errorf("服务[%s] 无效的传输协议: %s", label, svc.Transport)
	}
	if svc.ForwardRules == nil {
		svc.ForwardRules = []UIForwardRule{}
	}
	seen := make(map[string]struct{}, len(svc.ForwardRules))
	out := make([]UIForwardRule, 0, len(svc.ForwardRules))
	for i, r := range svc.ForwardRules {
		proto := strings.ToLower(strings.TrimSpace(r.Proto))
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("服务[%s] 第%d条规则协议无效", label, i+1)
		}
		listenHost, listenPort, err := parseHostPortIPv4(r.Listen)
		if err != nil {
			return fmt.Errorf("服务[%s] 第%d条规则本地地址: %v", label, i+1, err)
		}
		remoteHost, remotePort, err := parseHostPortIPv4(r.Remote)
		if err != nil {
			return fmt.Errorf("服务[%s] 第%d条规则远端地址: %v", label, i+1, err)
		}
		listen := net.JoinHostPort(listenHost, strconv.Itoa(listenPort))
		key := proto + "|" + listen
		if _, ok := seen[key]; ok {
			return fmt.Errorf("服务[%s] 第%d条规则监听地址重复: %s %s", label, i+1, proto, listen)
		}
		seen[key] = struct{}{}
		out = append(out, UIForwardRule{
			Proto:  proto,
			Listen: listen,
			Remote: net.JoinHostPort(remoteHost, strconv.Itoa(remotePort)),
		})
	}
	svc.ForwardRules = out
	return nil
}

func serviceNameByID(cfg UIConfig, id string) string {
	if svc, ok := FindService(cfg, id); ok {
		return svc.Name
	}
	return id
}

func validateTunIP(ip string) error {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || parsed.To4() == nil {
		return fmt.Errorf("网段地址无效: %s", ip)
	}
	b := parsed.To4()
	if b[0] != 192 || b[1] != 17 || b[3] != 1 || b[2] < 1 {
		return fmt.Errorf("网段地址必须是 192.17.x.1: %s", ip)
	}
	return nil
}

func tunSegmentOf(ip string) (int, bool) {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil || parsed.To4() == nil {
		return 0, false
	}
	b := parsed.To4()
	if b[0] != 192 || b[1] != 17 || b[3] != 1 {
		return 0, false
	}
	return int(b[2]), true
}

func (s UIService) tunSegment() (int, bool) {
	return tunSegmentOf(s.TunIP)
}

// AdapterName 由 TunIP 第三段派生网卡名, 不落盘。
func (s UIService) AdapterName() string {
	n, ok := s.tunSegment()
	if !ok {
		return "Goodlink"
	}
	return fmt.Sprintf("Goodlink-%d", n)
}

// TunIfaceIP 返回同网段网卡地址 192.17.N.2。
func (s UIService) TunIfaceIP() string {
	n, ok := s.tunSegment()
	if !ok {
		return ""
	}
	return fmt.Sprintf("%s%d.2", tunIPPrefix, n)
}

func (s UIService) ForwardFingerprint() string {
	b, _ := json.Marshal(s.ForwardRules)
	return s.LocalMode + "\n" + string(b)
}

// AllocateTunIP 从 19 起找第一个空闲第三段, 返回 192.17.N.1。
func AllocateTunIP(cfg UIConfig) string {
	used := map[int]struct{}{}
	for _, s := range cfg.Services {
		if n, ok := tunSegmentOf(s.TunIP); ok {
			used[n] = struct{}{}
		}
	}
	for n := tunSegmentMin; n <= tunSegmentMax; n++ {
		if _, ok := used[n]; !ok {
			return fmt.Sprintf("%s%d.1", tunIPPrefix, n)
		}
	}
	return ""
}

// AllocateServiceName 取「服务N」里第一个未被占用的 N。
func AllocateServiceName(cfg UIConfig) string {
	used := map[string]struct{}{}
	for _, s := range cfg.Services {
		used[strings.TrimSpace(s.Name)] = struct{}{}
	}
	for n := 1; n < 10000; n++ {
		name := fmt.Sprintf("服务%d", n)
		if _, ok := used[name]; !ok {
			return name
		}
	}
	return "服务"
}

func FindService(cfg UIConfig, id string) (*UIService, bool) {
	for i := range cfg.Services {
		if cfg.Services[i].ID == id {
			return &cfg.Services[i], true
		}
	}
	return nil, false
}

func loadUIConfigFile() UIConfig {
	cfg, err := LoadUIConfig(UIConfigPath())
	if err == nil {
		_ = NormalizeAndValidate(&cfg)
	}
	return cfg
}

// GetService 按 id 读取本进程负责的服务。
func GetService(id string) (UIService, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return UIService{}, fmt.Errorf("未指定服务")
	}
	cfg := loadUIConfigFile()
	svc, ok := FindService(cfg, id)
	if !ok {
		return UIService{}, fmt.Errorf("服务不存在: %s", id)
	}
	return *svc, nil
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

func listenRuleKey(r UIForwardRule) string {
	return strings.ToLower(strings.TrimSpace(r.Proto)) + "|" + r.Listen
}

// ListenConflicts 探测规则中尚未被 skip 覆盖的本地监听是否已被占用。
// skip 用于跳过本程序已持有的监听（例如转发子进程正在使用的端口）。
func ListenConflicts(rules, skip []UIForwardRule) []string {
	owned := make(map[string]struct{}, len(skip))
	for _, r := range skip {
		owned[listenRuleKey(r)] = struct{}{}
	}
	var conflicts []string
	seen := make(map[string]struct{})
	for _, r := range rules {
		key := listenRuleKey(r)
		if _, ok := owned[key]; ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if listenOccupied(r.Proto, r.Listen) {
			proto := strings.ToUpper(strings.TrimSpace(r.Proto))
			if proto == "" {
				proto = "TCP"
			}
			conflicts = append(conflicts, proto+" "+r.Listen)
		}
	}
	return conflicts
}

func listenOccupied(proto, addr string) bool {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "udp":
		udpAddr, err := net.ResolveUDPAddr("udp4", addr)
		if err != nil {
			return true
		}
		pc, err := net.ListenUDP("udp4", udpAddr)
		if err != nil {
			return true
		}
		pc.Close()
		return false
	default:
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return true
		}
		ln.Close()
		return false
	}
}

func FormatListenConflicts(conflicts []string) string {
	if len(conflicts) == 0 {
		return ""
	}
	return "本地端口已被占用: " + strings.Join(conflicts, "、")
}

// NormalizeTransport 空值视为 kcp。
func NormalizeTransport(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return TransportKCP
	}
	return s
}
