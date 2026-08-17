//go:build windows

package ui2

import (
	"encoding/json"
	"sync"

	"goodlink/config"
)

const (
	kindInitializing         = "initializing"
	kindIdle                 = "idle"
	kindStarting             = "starting"
	kindConnecting           = "connecting"
	kindConnectingNat4       = "connecting_nat4"
	kindConnectingNat4ToNat4 = "connecting_nat4_to_nat4"
	kindConnected            = "connected"
	kindRunning              = "running"
	kindStopping             = "stopping"
)

// 按钮状态类型
type buttonState struct {
	kind          string
	text          string
	importance    string
	enabled       bool
	activity      bool
	other_enabled bool
}

type ButtonInfo struct {
	Kind          string `json:"kind"`
	Text          string `json:"text"`
	Importance    string `json:"importance"`
	Enabled       bool   `json:"enabled"`
	Activity      bool   `json:"activity"`
	OthersEnabled bool   `json:"othersEnabled"`
}

type NATInfo struct {
	Ready  bool   `json:"ready"`
	IsNAT4 bool   `json:"isNAT4"`
	Text   string `json:"text"`
}

type UpgradeInfo struct {
	Need   bool   `json:"need"`
	Latest string `json:"latest"`
}

type UIState struct {
	Version  string      `json:"version"`
	WorkType string      `json:"workType"`
	TunKey   string      `json:"tunKey"`
	Button   ButtonInfo  `json:"button"`
	NAT      NATInfo     `json:"nat"`
	Upgrade  UpgradeInfo `json:"upgrade"`
	Logs     []string    `json:"logs,omitempty"`
}

// 预定义的按钮状态
var (
	buttonStateInitializing = buttonState{
		kind:          kindInitializing,
		text:          "检测网络中...",
		importance:    "high",
		enabled:       false,
		activity:      false,
		other_enabled: true,
	}
	buttonStateIdle = buttonState{
		kind:          kindIdle,
		text:          "点击启动",
		importance:    "high",
		enabled:       true,
		activity:      false,
		other_enabled: true,
	}
	buttonStateStarting = buttonState{
		kind:          kindStarting,
		text:          "启动中...",
		importance:    "warning",
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnecting = buttonState{
		kind:          kindConnecting,
		text:          "连接中...",
		importance:    "warning",
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnectingNat4 = buttonState{
		kind:          kindConnectingNat4,
		text:          "当前网络是NAT4, 连接中...",
		importance:    "warning",
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnectingNat4ToNat4 = buttonState{
		kind:          kindConnectingNat4ToNat4,
		text:          "两端网络都是NAT4, 连接中...",
		importance:    "danger",
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnected = buttonState{
		kind:          kindConnected,
		text:          "连接成功, 点击停止",
		importance:    "success",
		enabled:       true,
		activity:      false,
		other_enabled: false,
	}
	buttonStateRunning = buttonState{
		kind:          kindRunning,
		text:          "启动成功, 点击停止",
		importance:    "success",
		enabled:       true,
		activity:      false,
		other_enabled: false,
	}
	buttonStateStopping = buttonState{
		kind:          kindStopping,
		text:          "停止中...",
		importance:    "warning",
		enabled:       false,
		activity:      false,
		other_enabled: false,
	}
)

var (
	stateMu       sync.RWMutex
	m_work_type   string
	m_tun_key     string
	currentButton buttonState
	m_nat         = NATInfo{Text: "正在检测当前网络环境..."}
	m_upgrade     UpgradeInfo
	sseHubInst    = newSSEHub()
)

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
}

func broadcastLog(line string) {
	sseHubInst.broadcast(sseMessage("log", map[string]string{"line": line}))
}

func snapshot(includeLogs bool) UIState {
	stateMu.RLock()
	st := UIState{
		Version:  config.GetVersion(),
		WorkType: m_work_type,
		TunKey:   m_tun_key,
		Button: ButtonInfo{
			Kind:          currentButton.kind,
			Text:          currentButton.text,
			Importance:    currentButton.importance,
			Enabled:       currentButton.enabled,
			Activity:      currentButton.activity,
			OthersEnabled: currentButton.other_enabled,
		},
		NAT:     m_nat,
		Upgrade: m_upgrade,
	}
	stateMu.RUnlock()
	if includeLogs {
		st.Logs = snapshotLogs()
	}
	return st
}

// updateButtonState 更新启动按钮的状态，同时同步托盘图标
func updateButtonState(state buttonState) {
	stateMu.Lock()
	currentButton = state
	stateMu.Unlock()
	UpdateTrayIcon(state)
	broadcastState()
}

func othersEnabled() bool {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return currentButton.other_enabled
}

// 获取当前工作类型
func GetWorkType() string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return m_work_type
}

func getTunKey() string {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return m_tun_key
}

func setWorkTypeAndKey(workType, tunKey string) {
	stateMu.Lock()
	if workType == workTypeLocal || workType == workTypeRemote {
		m_work_type = workType
	}
	if tunKey != "" {
		m_tun_key = tunKey
	}
	stateMu.Unlock()
}

// SetNATHint 根据 STUN 检测结果显示 NAT 类型提示
func SetNATHint(isNAT4 bool) {
	stateMu.Lock()
	if isNAT4 {
		m_nat = NATInfo{Ready: true, IsNAT4: true, Text: "当前网络为NAT4"}
	} else {
		m_nat = NATInfo{Ready: true, IsNAT4: false, Text: "当前网络为NAT1-NAT3"}
	}
	stateMu.Unlock()
	broadcastState()
}

func setUpgradeInfo(need bool, latest string) {
	stateMu.Lock()
	m_upgrade = UpgradeInfo{Need: need, Latest: latest}
	stateMu.Unlock()
	broadcastState()
}
