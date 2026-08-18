package proxy

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"

	"goodlink/config"

	"github.com/quic-go/quic-go"
)

type forwardEntry struct {
	key    string
	rule   ForwardRule
	runner ForwardRunner
}

// ForwardManager 管理本地转发监听器：按 proto+listen 做 diff，热更新时不断隧道。
type ForwardManager struct {
	mu      sync.Mutex
	entries []forwardEntry
	quic    *quic.Conn
	closed  bool
}

func NewForwardManager() *ForwardManager {
	return &ForwardManager{}
}

func listenKey(rule ForwardRule) string {
	return fmt.Sprintf("%d|%s", rule.Proto, rule.ListenAddr)
}

func RulesFromUI(rules []config.UIForwardRule) ([]ForwardRule, error) {
	out := make([]ForwardRule, 0, len(rules))
	for i, r := range rules {
		var proto byte
		switch strings.ToLower(strings.TrimSpace(r.Proto)) {
		case "udp":
			proto = 0x01
		case "tcp", "":
			proto = 0x00
		default:
			return nil, fmt.Errorf("第%d条规则协议无效: %s", i+1, r.Proto)
		}
		listenHost, listenPort, err := net.SplitHostPort(r.Listen)
		if err != nil {
			return nil, fmt.Errorf("第%d条规则本地地址解析失败: %v", i+1, err)
		}
		remoteHost, remotePortStr, err := net.SplitHostPort(r.Remote)
		if err != nil {
			return nil, fmt.Errorf("第%d条规则远端地址解析失败: %v", i+1, err)
		}
		remoteIP := net.ParseIP(remoteHost)
		if remoteIP == nil || remoteIP.To4() == nil {
			return nil, fmt.Errorf("第%d条规则远端IP无效: %s", i+1, remoteHost)
		}
		remotePort, err := strconv.Atoi(remotePortStr)
		if err != nil || remotePort <= 0 || remotePort > 65535 {
			return nil, fmt.Errorf("第%d条规则远端端口无效: %s", i+1, remotePortStr)
		}
		out = append(out, ForwardRule{
			ListenAddr: net.JoinHostPort(listenHost, listenPort),
			RemoteIP:   remoteIP.To4(),
			RemotePort: uint16(remotePort),
			Proto:      proto,
		})
	}
	return out, nil
}

func (m *ForwardManager) Apply(rules []ForwardRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}

	wanted := make(map[string]ForwardRule, len(rules))
	for _, rule := range rules {
		wanted[listenKey(rule)] = rule
	}

	existing := make(map[string]forwardEntry, len(m.entries))
	for _, e := range m.entries {
		existing[e.key] = e
	}

	var added []forwardEntry
	for key, rule := range wanted {
		if _, ok := existing[key]; ok {
			continue
		}
		runner, err := newForwardRunner(rule)
		if err != nil {
			for _, a := range added {
				a.runner.Close()
			}
			return err
		}
		if m.quic != nil {
			runner.SetQuicConn(m.quic)
		}
		added = append(added, forwardEntry{key: key, rule: rule, runner: runner})
	}

	next := make([]forwardEntry, 0, len(wanted))
	for _, e := range m.entries {
		rule, keep := wanted[e.key]
		if !keep {
			e.runner.Close()
			continue
		}
		if e.rule.RemotePort != rule.RemotePort || !e.rule.RemoteIP.Equal(rule.RemoteIP) {
			e.runner.SetTarget(rule.RemoteIP, rule.RemotePort)
			log.Printf("[proxy] 更新转发目标: %s -> %s:%d", rule.ListenAddr, rule.RemoteIP, rule.RemotePort)
			e.rule = rule
		}
		next = append(next, e)
	}
	for _, a := range added {
		go a.runner.Serve()
		next = append(next, a)
	}
	m.entries = next
	return nil
}

func (m *ForwardManager) SetQuic(conn *quic.Conn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return
	}
	m.quic = conn
	for _, e := range m.entries {
		if conn == nil {
			e.runner.ClearQuicConn()
		} else {
			e.runner.SetQuicConn(conn)
		}
	}
}

func (m *ForwardManager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	m.quic = nil
	for _, e := range m.entries {
		e.runner.ClearQuicConn()
		e.runner.Close()
	}
	m.entries = nil
}
