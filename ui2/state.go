package ui2

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	"goodlink/config"
	"goodlink/pro"
	"goodlink/proxy"
	"goodlink/utils"
)

const (
	StatusDetecting      = 0
	StatusIdle           = 1
	StatusStarting       = 2
	StatusConnecting     = 3
	StatusConnectingNAT4 = 4
	StatusConnected      = 5
	StatusRunning        = 6
	StatusStopping       = 7
	StatusConnectFailed  = 8
)

type NATInfo struct {
	Ready  bool `json:"ready"`
	IsNAT4 bool `json:"isNAT4"`
}

type UpgradeInfo struct {
	Need        bool   `json:"need"`
	Latest      string `json:"latest"`
	Body        string `json:"body,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
	Downloading bool   `json:"downloading"`
	Percent     int    `json:"percent"`
	Done        bool   `json:"done"`
	Error       string `json:"error,omitempty"`
	CheckError  string `json:"checkError,omitempty"`
}

type ProxyInfo struct {
	Socks    string `json:"socks,omitempty"`
	HTTP     string `json:"http,omitempty"`
	Fallback bool   `json:"fallback,omitempty"`
	Denied   bool   `json:"denied,omitempty"`
}

type ServiceView struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	WorkType        string                 `json:"workType"`
	TunKey          string                 `json:"tunKey"`
	LocalMode       string                 `json:"localMode"`
	Transport       string                 `json:"transport"`
	TunIP           string                 `json:"tunIP"`
	AutoStart       bool                   `json:"autoStart"`
	AllowLocalProxy bool                   `json:"allowLocalProxy"`
	ForwardRules    []config.UIForwardRule `json:"forwardRules"`
	Status          int                    `json:"status"`
	Proxy           ProxyInfo              `json:"proxy"`
	Logs            []string               `json:"logs,omitempty"`
}

type UIState struct {
	Version  string        `json:"version"`
	IsAdmin  bool          `json:"isAdmin"`
	NAT      NATInfo       `json:"nat"`
	Upgrade  UpgradeInfo   `json:"upgrade"`
	Services []ServiceView `json:"services"`
	Logs     []string      `json:"logs,omitempty"`
}

type serviceState struct {
	cfg         config.UIService
	status      int
	proxyHost   string
	proxyPort   int
	proxyDenied bool
}

var (
	stateMu    sync.RWMutex
	m_services []*serviceState
	m_index    = map[string]*serviceState{}
	m_nat      NATInfo
	m_upgrade  UpgradeInfo
	sseHubInst = newSSEHub()

	runtimeConf config.RuntimeConf
)

func SetRuntimeConf(c config.RuntimeConf) {
	runtimeConf = c
}

type sseHub struct {
	mu      sync.Mutex
	clients map[chan []byte]struct{}
}

func newSSEHub() *sseHub {
	return &sseHub{clients: make(map[chan []byte]struct{})}
}

func (h *sseHub) subscribe() chan []byte {
	ch := make(chan []byte, 64)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *sseHub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
}

func (h *sseHub) broadcast(msg []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for ch := range h.clients {
		select {
		case ch <- msg:
		default:
		}
	}
}

func sseMessage(event string, v any) []byte {
	b, _ := json.Marshal(v)
	out := make([]byte, 0, 16+len(event)+len(b))
	out = append(out, "event: "...)
	out = append(out, event...)
	out = append(out, "\ndata: "...)
	out = append(out, b...)
	out = append(out, "\n\n"...)
	return out
}

func broadcastState() {
	sseHubInst.broadcast(sseMessage("state", snapshot(false)))
	refreshTray()
}

func broadcastLog(serviceID, line string) {
	sseHubInst.broadcast(sseMessage("log", map[string]string{
		"serviceId": serviceID,
		"line":      line,
	}))
}

func snapshot(includeLogs bool) UIState {
	stateMu.RLock()
	services := make([]ServiceView, 0, len(m_services))
	for _, svc := range m_services {
		view := serviceViewLocked(svc, includeLogs)
		services = append(services, view)
	}
	st := UIState{
		Version:  config.GetVersion(),
		IsAdmin:  utils.IsAdmin(),
		NAT:      m_nat,
		Upgrade:  m_upgrade,
		Services: services,
	}
	stateMu.RUnlock()
	if includeLogs {
		st.Logs = snapshotSharedLogs()
	}
	return st
}

func serviceViewLocked(svc *serviceState, includeLogs bool) ServiceView {
	rules := make([]config.UIForwardRule, len(svc.cfg.ForwardRules))
	copy(rules, svc.cfg.ForwardRules)
	localMode := svc.cfg.LocalMode
	if localMode == "" {
		localMode = config.LocalModeTUN
	}
	transport := svc.cfg.Transport
	if transport == "" {
		transport = config.TransportKCP
	}
	view := ServiceView{
		ID:              svc.cfg.ID,
		Name:            svc.cfg.Name,
		WorkType:        svc.cfg.WorkType,
		TunKey:          svc.cfg.TunKey,
		LocalMode:       localMode,
		Transport:       transport,
		TunIP:           svc.cfg.TunIP,
		AutoStart:       svc.cfg.AutoStart,
		AllowLocalProxy: svc.cfg.LocalProxyAllowed(),
		ForwardRules:    rules,
		Status:          svc.status,
		Proxy:           proxyInfoLocked(svc),
	}
	if includeLogs {
		view.Logs = snapshotServiceLogs(svc.cfg.ID)
	}
	return view
}

func makeProxyInfo(host string, port int, fallback bool) ProxyInfo {
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	return ProxyInfo{
		Socks:    "socks5://" + addr,
		HTTP:     "http://" + addr,
		Fallback: fallback,
	}
}

func proxyInfoLocked(svc *serviceState) ProxyInfo {
	if svc.cfg.WorkType != workTypeLocal {
		return ProxyInfo{}
	}
	if svc.proxyDenied {
		return ProxyInfo{Denied: true}
	}
	host := svc.proxyHost
	port := svc.proxyPort
	if host == "" || port <= 0 {
		return ProxyInfo{}
	}
	return makeProxyInfo(host, port, port != proxy.PROXY_PORT)
}

func lookupService(id string) (*serviceState, bool) {
	svc, ok := m_index[id]
	return svc, ok
}

func serviceIdle(svc *serviceState) bool {
	return svc.status == StatusDetecting || svc.status == StatusIdle || svc.status == StatusConnectFailed || svc.status == StatusConnectingNAT4
}

func setServiceStatus(id string, status int) {
	stateMu.Lock()
	if svc, ok := lookupService(id); ok {
		svc.status = status
	}
	stateMu.Unlock()
	broadcastState()
}

func getServiceStatus(id string) int {
	stateMu.RLock()
	defer stateMu.RUnlock()
	if svc, ok := lookupService(id); ok {
		return svc.status
	}
	return StatusIdle
}

func setProxyListen(id, addr string) {
	if strings.TrimSpace(addr) == pro.ProxyDenyToken {
		stateMu.Lock()
		if svc, ok := lookupService(id); ok {
			svc.proxyHost = ""
			svc.proxyPort = 0
			svc.proxyDenied = true
		}
		stateMu.Unlock()
		broadcastState()
		return
	}
	host, portStr, err := net.SplitHostPort(strings.TrimSpace(addr))
	if err != nil {
		return
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return
	}
	stateMu.Lock()
	if svc, ok := lookupService(id); ok {
		svc.proxyHost = host
		svc.proxyPort = port
		svc.proxyDenied = false
	}
	stateMu.Unlock()
	broadcastState()
}

func resetProxyListen(id string) {
	stateMu.Lock()
	if svc, ok := lookupService(id); ok {
		svc.proxyHost = ""
		svc.proxyPort = 0
		svc.proxyDenied = false
	}
	stateMu.Unlock()
	broadcastState()
}

func serviceName(id string) string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	if svc, ok := lookupService(id); ok && svc.cfg.Name != "" {
		return svc.cfg.Name
	}
	return id
}

func allServicesConfig() config.UIConfig {
	stateMu.RLock()
	defer stateMu.RUnlock()
	cfg := config.UIConfig{Services: make([]config.UIService, 0, len(m_services))}
	for _, svc := range m_services {
		cfg.Services = append(cfg.Services, svc.cfg)
	}
	return cfg
}

func replaceServices(list []config.UIService) {
	stateMu.Lock()
	m_services = make([]*serviceState, 0, len(list))
	m_index = map[string]*serviceState{}
	for _, item := range list {
		svc := &serviceState{cfg: item, status: StatusDetecting}
		m_services = append(m_services, svc)
		m_index[item.ID] = svc
	}
	stateMu.Unlock()
}

func mutateService(id string, fn func(*serviceState) error) error {
	stateMu.Lock()
	defer stateMu.Unlock()
	svc, ok := lookupService(id)
	if !ok {
		return fmt.Errorf("服务不存在")
	}
	return fn(svc)
}

func SetNATHint(isNAT4 bool) {
	stateMu.Lock()
	m_nat = NATInfo{Ready: true, IsNAT4: isNAT4}
	stateMu.Unlock()
	broadcastState()
}

func setUpgradeInfo(need bool, latest, body, downloadURL string) {
	stateMu.Lock()
	m_upgrade.Need = need
	m_upgrade.Latest = latest
	m_upgrade.CheckError = ""
	if body != "" {
		m_upgrade.Body = body
	}
	if downloadURL != "" {
		m_upgrade.DownloadURL = downloadURL
	}
	st := m_upgrade
	stateMu.Unlock()
	broadcastState()
	broadcastUpgrade(st)
}

func setUpgradeCheckError(msg string) {
	stateMu.Lock()
	m_upgrade.Need = false
	m_upgrade.CheckError = msg
	st := m_upgrade
	stateMu.Unlock()
	broadcastState()
	broadcastUpgrade(st)
}

func snapshotUpgrade() UpgradeInfo {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return m_upgrade
}

func setUpgradeProgress(downloading bool, percent int, done bool, errStr string) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	stateMu.Lock()
	m_upgrade.Downloading = downloading
	m_upgrade.Percent = percent
	m_upgrade.Done = done
	m_upgrade.Error = errStr
	st := m_upgrade
	stateMu.Unlock()
	broadcastUpgrade(st)
	if !downloading || done || errStr != "" {
		broadcastState()
	}
}

func broadcastUpgrade(info UpgradeInfo) {
	sseHubInst.broadcast(sseMessage("upgrade", info))
}

func aggregateStatus() int {
	stateMu.RLock()
	defer stateMu.RUnlock()
	best := StatusIdle
	rank := map[int]int{
		StatusIdle:           0,
		StatusDetecting:      0,
		StatusStarting:       1,
		StatusConnecting:     1,
		StatusStopping:       1,
		StatusConnected:      2,
		StatusRunning:        2,
		StatusConnectingNAT4: 3,
		StatusConnectFailed:  3,
	}
	for _, svc := range m_services {
		if rank[svc.status] > rank[best] {
			best = svc.status
		}
	}
	return best
}

func trayTooltip() string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	if len(m_services) == 0 {
		return M_APP_TITLE + "  v" + config.GetVersion()
	}
	lines := make([]string, 0, len(m_services))
	for _, svc := range m_services {
		lines = append(lines, svc.cfg.Name+" - "+statusLabel(svc.status))
	}
	return strings.Join(lines, "\n")
}

func statusLabel(status int) string {
	switch status {
	case StatusIdle:
		return "待启动"
	case StatusStarting:
		return "启动中"
	case StatusConnecting:
		return "连接中"
	case StatusConnectingNAT4:
		return "无法直连"
	case StatusConnected:
		return "已连接"
	case StatusRunning:
		return "运行中"
	case StatusStopping:
		return "停止中"
	case StatusConnectFailed:
		return "连接失败"
	default:
		return "检测中"
	}
}

func anyServiceActive() bool {
	procMu.Lock()
	for _, p := range procs {
		if p.started {
			procMu.Unlock()
			return true
		}
	}
	procMu.Unlock()

	stateMu.RLock()
	defer stateMu.RUnlock()
	for _, svc := range m_services {
		if !serviceIdle(svc) {
			return true
		}
	}
	return false
}
