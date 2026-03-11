package proxy

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"goodlink/config"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

type ForwardRule struct {
	ListenAddr string
	RemoteIP   net.IP
	RemotePort uint16
}

var ForwardRules []ForwardRule

func CheckForwardArgs() bool {
	ForwardRules = nil

	if config.Arg_local_proxy_addr != "" {
		ForwardRules = append(ForwardRules, ForwardRule{
			ListenAddr: config.Arg_local_proxy_addr,
			RemoteIP:   net.IPv4(127, 0, 0, 1),
			RemotePort: PROXY_PORT,
		})
	}

	if config.Arg_local_forward_addrs != "" {
		entries := strings.Split(config.Arg_local_forward_addrs, ",")
		for _, entry := range entries {
			entry = strings.TrimSpace(entry)
			if entry == "" {
				continue
			}
			// 格式: listenHost:listenPort@remoteHost:remotePort
			parts := strings.SplitN(entry, "@", 2)
			if len(parts) != 2 {
				log.Printf("[proxy] 转发地址格式错误(需要 listenHost:listenPort@remoteHost:remotePort): %s", entry)
				ForwardRules = nil
				return false
			}
			listenHost, listenPort, err := net.SplitHostPort(parts[0])
			if err != nil {
				log.Printf("[proxy] 转发监听地址解析失败: %s, %v", parts[0], err)
				ForwardRules = nil
				return false
			}
			listenAddr := net.JoinHostPort(listenHost, listenPort)
			remoteHost, remotePortStr, err := net.SplitHostPort(parts[1])
			if err != nil {
				log.Printf("[proxy] 转发目标地址解析失败: %s, %v", parts[1], err)
				ForwardRules = nil
				return false
			}
			remoteIP := net.ParseIP(remoteHost)
			if remoteIP == nil {
				log.Printf("[proxy] 转发目标IP解析失败: %s", remoteHost)
				ForwardRules = nil
				return false
			}
			remotePort, err := strconv.Atoi(remotePortStr)
			if err != nil || remotePort <= 0 || remotePort > 65535 {
				log.Printf("[proxy] 转发目标端口无效: %s", remotePortStr)
				ForwardRules = nil
				return false
			}
			ForwardRules = append(ForwardRules, ForwardRule{
				ListenAddr: listenAddr,
				RemoteIP:   remoteIP.To4(),
				RemotePort: uint16(remotePort),
			})
		}
	}

	return len(ForwardRules) > 0
}

// ForwardClient 管理 TCP 监听和 QUIC 隧道转发。
// listener 只创建一次，隧道重连时通过 SetQuicConn/ClearQuicConn 热替换 QUIC 连接。
type ForwardClient struct {
	listener   net.Listener
	mu         sync.RWMutex
	quicConn   *quic.Conn
	remoteIP   net.IP
	remotePort uint16
}

func NewForwardClients() ([]*ForwardClient, error) {
	clients := make([]*ForwardClient, 0, len(ForwardRules))
	for _, rule := range ForwardRules {
		ln, err := net.Listen("tcp", rule.ListenAddr)
		if err != nil {
			for _, c := range clients {
				c.Close()
			}
			return nil, err
		}
		log.Printf("[proxy] TCP转发监听: %s -> %s:%d", rule.ListenAddr, rule.RemoteIP, rule.RemotePort)
		clients = append(clients, &ForwardClient{
			listener:   ln,
			remoteIP:   rule.RemoteIP,
			remotePort: rule.RemotePort,
		})
	}
	return clients, nil
}

func (p *ForwardClient) SetQuicConn(conn *quic.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = conn
}

func (p *ForwardClient) ClearQuicConn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = nil
}

func (p *ForwardClient) getQuicConn() *quic.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.quicConn
}

func (p *ForwardClient) Serve() {
	for {
		tcpConn, err := p.listener.Accept()
		if err != nil {
			log.Printf("[proxy] 监听转发端口异常: %v", err)
			return
		}

		quicConn := p.getQuicConn()
		if quicConn == nil {
			log.Println("[proxy] 隧道未就绪，拒绝TCP连接")
			tcpConn.Close()
			continue
		}

		go p.handleConn(tcpConn, quicConn)
	}
}

func (p *ForwardClient) handleConn(tcpConn net.Conn, quicConn *quic.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		log.Println("[proxy] 打开QUIC流失败:", err)
		tcpConn.Close()
		return
	}

	ioBuf := go2pool.Malloc(HEAD_LEN)

	ioBuf[0] = 0x00 // TCP协议标识
	copy(ioBuf[1:5], p.remoteIP.To4())
	binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], p.remotePort)

	if _, err := stream.Write(ioBuf[:HEAD_LEN]); err != nil {
		go2pool.Free(ioBuf)
		log.Println("[proxy] 写入头部失败:", err)
		tcpConn.Close()
		stream.CancelRead(0)
		stream.Close()
		return
	}
	go2pool.Free(ioBuf)

	go ForwardT2Q(tcpConn, stream)
	go ForwardQ2T(stream, tcpConn)
}

func (p *ForwardClient) Close() {
	if p.listener != nil {
		p.listener.Close()
	}
}
