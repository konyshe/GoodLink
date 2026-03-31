package proxy

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"goodlink/config"
	"io"
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
	Proto      byte // 0x00 TCP, 0x01 UDP（与 Remote process_stream 一致）
}

var ForwardRules []ForwardRule

func appendForwardRuleEntries(csv string, proto byte) bool {
	if csv == "" {
		return true
	}
	entries := strings.Split(csv, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
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
			Proto:      proto,
		})
	}
	return true
}

func CheckForwardArgs() bool {
	ForwardRules = nil

	if config.Arg_local_proxy_addr != "" {
		ForwardRules = append(ForwardRules, ForwardRule{
			ListenAddr: config.Arg_local_proxy_addr,
			RemoteIP:   net.IPv4(127, 0, 0, 1),
			RemotePort: PROXY_PORT,
			Proto:      0x00,
		})
	}

	if !appendForwardRuleEntries(config.Arg_local_forward_tcp_addrs, 0x00) {
		return false
	}
	if !appendForwardRuleEntries(config.Arg_local_forward_udp_addrs, 0x01) {
		return false
	}

	return len(ForwardRules) > 0
}

// ForwardRunner 本地转发监听器（TCP 或 UDP），隧道重连时通过 SetQuicConn/ClearQuicConn 热替换 QUIC。
type ForwardRunner interface {
	SetQuicConn(conn *quic.Conn)
	ClearQuicConn()
	Serve()
	Close()
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

// ForwardUDPClient 管理 UDP 监听：每个入站数据报对应一条 QUIC 流。
type ForwardUDPClient struct {
	pc         *net.UDPConn
	mu         sync.RWMutex
	quicConn   *quic.Conn
	remoteIP   net.IP
	remotePort uint16
}

type udpWriteBack struct {
	pc   *net.UDPConn
	addr net.Addr
}

func (u *udpWriteBack) Write(b []byte) (int, error) {
	return u.pc.WriteTo(b, u.addr)
}

func NewForwardRunners() ([]ForwardRunner, error) {
	runners := make([]ForwardRunner, 0, len(ForwardRules))
	for _, rule := range ForwardRules {
		switch rule.Proto {
		case 0x01:
			udpAddr, err := net.ResolveUDPAddr("udp4", rule.ListenAddr)
			if err != nil {
				for _, r := range runners {
					r.Close()
				}
				return nil, err
			}
			pc, err := net.ListenUDP("udp4", udpAddr)
			if err != nil {
				for _, r := range runners {
					r.Close()
				}
				return nil, err
			}
			log.Printf("[proxy] UDP转发监听: %s -> %s:%d", rule.ListenAddr, rule.RemoteIP, rule.RemotePort)
			runners = append(runners, &ForwardUDPClient{
				pc:         pc,
				remoteIP:   rule.RemoteIP,
				remotePort: rule.RemotePort,
			})
		default:
			ln, err := net.Listen("tcp", rule.ListenAddr)
			if err != nil {
				for _, r := range runners {
					r.Close()
				}
				return nil, err
			}
			log.Printf("[proxy] TCP转发监听: %s -> %s:%d", rule.ListenAddr, rule.RemoteIP, rule.RemotePort)
			runners = append(runners, &ForwardClient{
				listener:   ln,
				remoteIP:   rule.RemoteIP,
				remotePort: rule.RemotePort,
			})
		}
	}
	return runners, nil
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

func (p *ForwardUDPClient) SetQuicConn(conn *quic.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = conn
}

func (p *ForwardUDPClient) ClearQuicConn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = nil
}

func (p *ForwardUDPClient) getQuicConn() *quic.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.quicConn
}

func (p *ForwardUDPClient) Serve() {
	buf := make([]byte, 65535)
	for {
		n, clientAddr, err := p.pc.ReadFrom(buf)
		if err != nil {
			log.Printf("[proxy] UDP转发监听异常: %v", err)
			return
		}
		if n == 0 {
			continue
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])
		quicConn := p.getQuicConn()
		if quicConn == nil {
			log.Println("[proxy] 隧道未就绪，丢弃UDP数据报")
			continue
		}
		go p.handleDatagram(payload, clientAddr, quicConn)
	}
}

func (p *ForwardUDPClient) handleDatagram(payload []byte, clientAddr net.Addr, quicConn *quic.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		log.Println("[proxy] UDP转发打开QUIC流失败:", err)
		return
	}

	ioBuf := go2pool.Malloc(HEAD_LEN)
	ioBuf[0] = 0x01 // UDP协议标识
	copy(ioBuf[1:5], p.remoteIP.To4())
	binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], p.remotePort)
	if _, err := stream.Write(ioBuf[:HEAD_LEN]); err != nil {
		go2pool.Free(ioBuf)
		log.Println("[proxy] UDP转发写入头部失败:", err)
		stream.CancelRead(0)
		stream.Close()
		return
	}
	go2pool.Free(ioBuf)

	if _, err := stream.Write(payload); err != nil {
		log.Println("[proxy] UDP转发写入载荷失败:", err)
		stream.CancelRead(0)
		stream.Close()
		return
	}

	wb := &udpWriteBack{pc: p.pc, addr: clientAddr}
	cpBuf := go2pool.Malloc(32 * 1024)
	defer go2pool.Free(cpBuf)
	_, err = io.CopyBuffer(wb, stream, cpBuf)
	if err != nil && err != io.EOF {
		log.Println("[proxy] UDP转发读流失败:", err)
	}
	stream.CancelRead(0)
	stream.Close()
}

func (p *ForwardUDPClient) Close() {
	if p.pc != nil {
		p.pc.Close()
	}
}
