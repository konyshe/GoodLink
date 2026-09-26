package proxy

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	go2pool "go2/pool"
	"goodlink/tls2"
	"io"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// 监听出错后的重试参数: 临时错误(如 fd 耗尽)直接退出会让该端口永久失效,
	// 因此退避重试; 连续失败超过阈值才判定不可恢复
	serveRetryBase   = 50 * time.Millisecond
	serveRetryMax    = 1 * time.Second
	maxServeFailures = 10
	udpWriteTimeout  = 5 * time.Second
	maxUDPSessions   = 1024
	sessionIdleSweep = 30 * time.Second
)

type ForwardRule struct {
	ListenAddr string
	RemoteIP   net.IP
	RemotePort uint16
	Proto      byte // 0x00 TCP, 0x01 UDP（与 Remote process_stream 一致）
}

type ForwardRunner interface {
	SetQuicConn(conn tls2.Conn)
	ClearQuicConn()
	SetTarget(ip net.IP, port uint16)
	ListenAddr() string
	Serve()
	Close()
	// Failed 报告该监听是否已停止服务, 供 ForwardManager 在下一次热更新时重建
	Failed() bool
}

// retryServeError 判断监听错误是否值得继续等待。
// 原来 Accept / ReadFrom 一出错就 return, 一次临时错误(EMFILE 等)就会让该端口
// 在整个进程生命周期内静默失效, UI 仍然显示已连接。
func retryServeError(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	if !errors.As(err, &ne) {
		return false
	}
	return ne.Temporary() || ne.Timeout()
}

// serveBackoff 按连续失败次数递增退避, 避免持续错误时变成忙循环
func serveBackoff(failures int) time.Duration {
	d := serveRetryBase * time.Duration(failures)
	if d > serveRetryMax || d <= 0 {
		return serveRetryMax
	}
	return d
}

type ForwardClient struct {
	listener   net.Listener
	mu         sync.RWMutex
	quicConn   tls2.Conn
	remoteIP   net.IP
	remotePort uint16
	failed     atomic.Bool
}

// ForwardUDPClient 按客户端地址复用隧道流。每数据报开一条流会把 QUIC 握手打满。
type ForwardUDPClient struct {
	pc         *net.UDPConn
	mu         sync.RWMutex
	quicConn   tls2.Conn
	remoteIP   net.IP
	remotePort uint16

	sessMu   sync.Mutex
	sessions map[string]*udpFwdSession
	sweeping bool

	failed atomic.Bool
}

type udpFwdSession struct {
	mu        sync.Mutex
	stream    tls2.Stream
	closed    bool
	createdAt time.Time
	lastUsed  time.Time
}

func newForwardRunner(rule ForwardRule) (ForwardRunner, error) {
	switch rule.Proto {
	case 0x01:
		udpAddr, err := net.ResolveUDPAddr("udp4", rule.ListenAddr)
		if err != nil {
			return nil, err
		}
		pc, err := net.ListenUDP("udp4", udpAddr)
		if err != nil {
			return nil, err
		}
		log.Printf("[proxy] UDP转发监听: %s -> %s:%d", rule.ListenAddr, rule.RemoteIP, rule.RemotePort)
		return &ForwardUDPClient{
			pc:         pc,
			remoteIP:   rule.RemoteIP,
			remotePort: rule.RemotePort,
			sessions:   make(map[string]*udpFwdSession),
		}, nil
	default:
		ln, err := net.Listen("tcp", rule.ListenAddr)
		if err != nil {
			return nil, err
		}
		log.Printf("[proxy] TCP转发监听: %s -> %s:%d", rule.ListenAddr, rule.RemoteIP, rule.RemotePort)
		return &ForwardClient{
			listener:   ln,
			remoteIP:   rule.RemoteIP,
			remotePort: rule.RemotePort,
		}, nil
	}
}

func (p *ForwardClient) SetQuicConn(conn tls2.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = conn
}

func (p *ForwardClient) ClearQuicConn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = nil
}

func (p *ForwardClient) getQuicConn() tls2.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.quicConn
}

func (p *ForwardClient) SetTarget(ip net.IP, port uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.remoteIP = ip
	p.remotePort = port
}

func (p *ForwardClient) target() (net.IP, uint16) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remoteIP, p.remotePort
}

func (p *ForwardClient) Failed() bool {
	return p.failed.Load()
}

func (p *ForwardClient) Serve() {
	// Serve 一旦返回, 该监听就彻底停止了: 必须标记失败, 让 ForwardManager 有机会重建
	defer p.failed.Store(true)

	failures := 0
	for {
		tcpConn, err := p.listener.Accept()
		if err != nil {
			if !retryServeError(err) {
				log.Printf("[proxy] 监听转发端口已停止: %v", err)
				return
			}
			failures++
			if failures > maxServeFailures {
				log.Printf("[proxy] 监听转发端口连续失败%d次, 停止重试: %v", failures, err)
				return
			}
			log.Printf("[proxy] 监听转发端口临时异常(%d/%d): %v", failures, maxServeFailures, err)
			time.Sleep(serveBackoff(failures))
			continue
		}
		failures = 0

		quicConn := p.getQuicConn()
		if quicConn == nil {
			log.Println("[proxy] 隧道未就绪，拒绝TCP连接")
			tcpConn.Close()
			continue
		}

		go p.handleConn(tcpConn, quicConn)
	}
}

func (p *ForwardClient) handleConn(tcpConn net.Conn, quicConn tls2.Conn) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		log.Println("[proxy] 打开QUIC流失败:", err)
		tcpConn.Close()
		return
	}

	remoteIP, remotePort := p.target()
	ip4 := remoteIP.To4()
	if ip4 == nil {
		// 缓冲池不清零: 目标IP无效时不能跳过写入, 否则残留字节会被当成目标地址
		log.Println("[proxy] 转发目标IP无效, 拒绝TCP连接")
		tcpConn.Close()
		stream.CancelRead(0)
		stream.Close()
		return
	}

	ioBuf := go2pool.Malloc(HEAD_LEN)

	ioBuf[0] = 0x00
	copy(ioBuf[1:5], ip4)
	binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], remotePort)

	if _, err := stream.Write(ioBuf[:HEAD_LEN]); err != nil {
		go2pool.Free(ioBuf)
		log.Println("[proxy] 写入头部失败:", err)
		tcpConn.Close()
		stream.CancelRead(0)
		stream.Close()
		return
	}
	go2pool.Free(ioBuf)

	Relay(tcpConn, stream)
}

func (p *ForwardClient) ListenAddr() string {
	if p.listener != nil {
		return p.listener.Addr().String()
	}
	return ""
}

func (p *ForwardClient) Close() {
	p.failed.Store(true)
	if p.listener != nil {
		p.listener.Close()
	}
}

func (p *ForwardUDPClient) SetQuicConn(conn tls2.Conn) {
	p.closeAllSessions()
	p.mu.Lock()
	p.quicConn = conn
	p.mu.Unlock()
}

func (p *ForwardUDPClient) ClearQuicConn() {
	p.mu.Lock()
	p.quicConn = nil
	p.mu.Unlock()
	p.closeAllSessions()
}

func (p *ForwardUDPClient) getQuicConn() tls2.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.quicConn
}

func (p *ForwardUDPClient) SetTarget(ip net.IP, port uint16) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.remoteIP = ip
	p.remotePort = port
}

func (p *ForwardUDPClient) target() (net.IP, uint16) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remoteIP, p.remotePort
}

func (s *udpFwdSession) write(b []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return io.EOF
	}
	// 必须设写超时: UDP 的 Serve 是单 goroutine 串行写,
	// 一条会话写不出去会让所有会话的数据报都堵在内核缓冲里被丢弃
	_ = s.stream.SetWriteDeadline(time.Now().Add(udpWriteTimeout))
	// 带长度前缀写入, 让对端能还原出数据报边界
	err := writeFrame(s.stream, b)
	s.lastUsed = time.Now()
	return err
}

// idleSince 返回该会话最后一次成功写入的时间(与 write 共用同一把锁)
func (s *udpFwdSession) idleSince() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastUsed
}

func (s *udpFwdSession) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.stream.CancelRead(0)
	_ = s.stream.Close()
}

func (p *ForwardUDPClient) closeAllSessions() {
	p.sessMu.Lock()
	old := p.sessions
	p.sessions = make(map[string]*udpFwdSession)
	p.sessMu.Unlock()
	for _, s := range old {
		s.close()
	}
}

func (p *ForwardUDPClient) dropSession(key string) {
	p.sessMu.Lock()
	s := p.sessions[key]
	delete(p.sessions, key)
	p.sessMu.Unlock()
	if s != nil {
		s.close()
	}
}

func (p *ForwardUDPClient) getSession(key string) *udpFwdSession {
	p.sessMu.Lock()
	defer p.sessMu.Unlock()
	return p.sessions[key]
}

func (p *ForwardUDPClient) newSession(key string, clientAddr net.Addr) (*udpFwdSession, error) {
	quicConn := p.getQuicConn()
	if quicConn == nil {
		return nil, io.ErrClosedPipe
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	stream, err := quicConn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}

	remoteIP, remotePort := p.target()
	ip4 := remoteIP.To4()
	if ip4 == nil {
		// 缓冲池不清零: 目标IP无效时不能跳过写入, 否则残留字节会被当成目标地址
		stream.CancelRead(0)
		_ = stream.Close()
		return nil, fmt.Errorf("转发目标IP无效")
	}
	ioBuf := go2pool.Malloc(HEAD_LEN)
	ioBuf[0] = 0x01
	copy(ioBuf[1:5], ip4)
	binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], remotePort)
	_, err = stream.Write(ioBuf[:HEAD_LEN])
	go2pool.Free(ioBuf)
	if err != nil {
		stream.CancelRead(0)
		_ = stream.Close()
		return nil, err
	}

	now := time.Now()
	sess := &udpFwdSession{stream: stream, createdAt: now, lastUsed: now}

	p.sessMu.Lock()
	if existing := p.sessions[key]; existing != nil {
		p.sessMu.Unlock()
		sess.close()
		return existing, nil
	}
	// 会话数必须有上限: 每条会话是一条隧道流 + 远端 socket + 两个 goroutine
	if len(p.sessions) >= maxUDPSessions {
		victim := p.evictOldestLocked()
		p.sessMu.Unlock()
		if victim != nil {
			log.Printf("[proxy] UDP会话数达到上限%d, 淘汰最旧会话", maxUDPSessions)
			victim.close()
		}
		p.sessMu.Lock()
	}
	p.sessions[key] = sess
	startSweeper := !p.sweeping
	if startSweeper {
		p.sweeping = true
	}
	p.sessMu.Unlock()

	if startSweeper {
		go p.sweepIdle()
	}

	go p.readBack(key, clientAddr, sess)
	return sess, nil
}

// evictOldestLocked 淘汰创建时间最早的会话, 调用方需持有 sessMu
func (p *ForwardUDPClient) evictOldestLocked() *udpFwdSession {
	var victimKey string
	var victim *udpFwdSession
	for k, s := range p.sessions {
		if victim == nil || s.createdAt.Before(victim.createdAt) {
			victimKey, victim = k, s
		}
	}
	if victimKey != "" {
		delete(p.sessions, victimKey)
	}
	return victim
}

// sweepIdle 回收长时间空闲的会话, 本地不再完全依赖对端的超时来回收
func (p *ForwardUDPClient) sweepIdle() {
	ticker := time.NewTicker(sessionIdleSweep)
	defer ticker.Stop()

	for range ticker.C {
		p.sessMu.Lock()
		if len(p.sessions) == 0 {
			p.sweeping = false
			p.sessMu.Unlock()
			return
		}
		now := time.Now()
		var victims []*udpFwdSession
		for k, s := range p.sessions {
			if now.Sub(s.idleSince()) > UDPIdleTimeout {
				delete(p.sessions, k)
				victims = append(victims, s)
			}
		}
		p.sessMu.Unlock()

		for _, s := range victims {
			s.close()
		}
	}
}

func (p *ForwardUDPClient) readBack(key string, clientAddr net.Addr, sess *udpFwdSession) {
	defer p.dropSession(key)

	buf := go2pool.Malloc(copyBufSize)
	defer go2pool.Free(buf)
	for {
		// 必须按帧重组: 隧道是字节流, 直接把任意长度的块写成 UDP 包会破坏数据报边界
		n, err := readFrame(sess.stream, buf)
		if n > 0 {
			if _, werr := p.pc.WriteTo(buf[:n], clientAddr); werr != nil {
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *ForwardUDPClient) Failed() bool {
	return p.failed.Load()
}

func (p *ForwardUDPClient) Serve() {
	// Serve 一旦返回, 该监听就彻底停止了: 必须标记失败, 让 ForwardManager 有机会重建
	defer p.failed.Store(true)

	buf := make([]byte, 65535)
	failures := 0
	for {
		n, clientAddr, err := p.pc.ReadFrom(buf)
		if err != nil {
			if !retryServeError(err) {
				log.Printf("[proxy] UDP转发监听已停止: %v", err)
				return
			}
			failures++
			if failures > maxServeFailures {
				log.Printf("[proxy] UDP转发监听连续失败%d次, 停止重试: %v", failures, err)
				return
			}
			log.Printf("[proxy] UDP转发监听临时异常(%d/%d): %v", failures, maxServeFailures, err)
			time.Sleep(serveBackoff(failures))
			continue
		}
		failures = 0
		if n == 0 {
			continue
		}
		if p.getQuicConn() == nil {
			log.Println("[proxy] 隧道未就绪，丢弃UDP数据报")
			continue
		}

		key := clientAddr.String()
		sess := p.getSession(key)
		if sess == nil {
			sess, err = p.newSession(key, clientAddr)
			if err != nil {
				log.Println("[proxy] UDP转发打开流失败:", err)
				continue
			}
		}
		if err := sess.write(buf[:n]); err != nil {
			p.dropSession(key)
		}
	}
}

func (p *ForwardUDPClient) ListenAddr() string {
	if p.pc != nil {
		return p.pc.LocalAddr().String()
	}
	return ""
}

func (p *ForwardUDPClient) Close() {
	p.failed.Store(true)
	p.closeAllSessions()
	if p.pc != nil {
		p.pc.Close()
	}
}
