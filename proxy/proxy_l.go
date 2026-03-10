package proxy

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

// ProxyClient 管理 TCP 监听和 QUIC 隧道转发。
// listener 只创建一次，隧道重连时通过 SetQuicConn/ClearQuicConn 热替换 QUIC 连接。
type ProxyClient struct {
	listener net.Listener
	mu       sync.RWMutex
	quicConn *quic.Conn
}

func NewProxyClient(listenAddr string) (*ProxyClient, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, err
	}
	log.Printf("[proxy] TCP代理监听: %s", listenAddr)
	return &ProxyClient{listener: ln}, nil
}

func (p *ProxyClient) SetQuicConn(conn *quic.Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = conn
	log.Println("[proxy] QUIC连接已设置，开始转发")
}

func (p *ProxyClient) ClearQuicConn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.quicConn = nil
	log.Println("[proxy] QUIC连接已清除，暂停转发")
}

func (p *ProxyClient) getQuicConn() *quic.Conn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.quicConn
}

func (p *ProxyClient) Serve() {
	for {
		tcpConn, err := p.listener.Accept()
		if err != nil {
			log.Printf("[proxy] listener已关闭: %v", err)
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

func (p *ProxyClient) handleConn(tcpConn net.Conn, quicConn *quic.Conn) {
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
	ipv4Bytes := tcpConn.LocalAddr().(*net.TCPAddr).IP.To4()
	copy(ioBuf[1:5], ipv4Bytes[:])
	binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], uint16(PROXY_PORT))

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

func (p *ProxyClient) Close() {
	if p.listener != nil {
		p.listener.Close()
		log.Println("[proxy] TCP代理监听已关闭")
	}
}
