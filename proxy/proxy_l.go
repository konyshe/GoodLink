package proxy

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"goodlink/tls2"
	"io"
	"log"
	"net"
	"sync"
	"time"
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
}

type ForwardClient struct {
	listener   net.Listener
	mu         sync.RWMutex
	quicConn   tls2.Conn
	remoteIP   net.IP
	remotePort uint16
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
}

type udpFwdSession struct {
	mu     sync.Mutex
	stream tls2.Stream
	closed bool
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

	ioBuf := go2pool.Malloc(HEAD_LEN)

	ioBuf[0] = 0x00
	copy(ioBuf[1:5], remoteIP.To4())
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
	_, err := s.stream.Write(b)
	return err
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
	ioBuf := go2pool.Malloc(HEAD_LEN)
	ioBuf[0] = 0x01
	copy(ioBuf[1:5], remoteIP.To4())
	binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], remotePort)
	_, err = stream.Write(ioBuf[:HEAD_LEN])
	go2pool.Free(ioBuf)
	if err != nil {
		stream.CancelRead(0)
		_ = stream.Close()
		return nil, err
	}

	sess := &udpFwdSession{stream: stream}
	p.sessMu.Lock()
	if existing := p.sessions[key]; existing != nil {
		p.sessMu.Unlock()
		sess.close()
		return existing, nil
	}
	p.sessions[key] = sess
	p.sessMu.Unlock()

	go p.readBack(key, clientAddr, sess)
	return sess, nil
}

func (p *ForwardUDPClient) readBack(key string, clientAddr net.Addr, sess *udpFwdSession) {
	defer p.dropSession(key)

	buf := go2pool.Malloc(copyBufSize)
	defer go2pool.Free(buf)
	for {
		n, err := sess.stream.Read(buf)
		if n > 0 {
			_, _ = p.pc.WriteTo(buf[:n], clientAddr)
		}
		if err != nil {
			return
		}
	}
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
	p.closeAllSessions()
	if p.pc != nil {
		p.pc.Close()
	}
}
