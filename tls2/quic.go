package tls2

import (
	"context"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

var quicCfg = &quic.Config{
	MaxIdleTimeout:     120 * time.Second,
	KeepAlivePeriod:    10 * time.Second,
	MaxIncomingStreams: 16 * 1024,
	Allow0RTT:          true,
}

type quicConn struct {
	c *quic.Conn
}

type quicStream struct {
	s *quic.Stream
}

type quicListener struct {
	ln *quic.EarlyListener
}

func dialQUIC(ctx context.Context, conn net.PacketConn, remoteAddr net.Addr) (Conn, error) {
	c, err := quic.DialEarly(ctx, conn, remoteAddr, GetClientTLSConfig(), quicCfg)
	if err != nil {
		return nil, err
	}
	return &quicConn{c: c}, nil
}

func listenQUIC(conn net.PacketConn) (Listener, error) {
	ln, err := quic.ListenEarly(conn, GetServerTLSConfig(), quicCfg)
	if err != nil {
		return nil, err
	}
	return &quicListener{ln: ln}, nil
}

func (c *quicConn) OpenStreamSync(ctx context.Context) (Stream, error) {
	s, err := c.c.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{s: s}, nil
}

func (c *quicConn) AcceptStream(ctx context.Context) (Stream, error) {
	s, err := c.c.AcceptStream(ctx)
	if err != nil {
		return nil, err
	}
	return &quicStream{s: s}, nil
}

func (c *quicConn) CloseWithError(code uint64, msg string) error {
	return c.c.CloseWithError(quic.ApplicationErrorCode(code), msg)
}

func (c *quicConn) LocalAddr() net.Addr  { return c.c.LocalAddr() }
func (c *quicConn) RemoteAddr() net.Addr { return c.c.RemoteAddr() }

func (s *quicStream) Read(p []byte) (int, error)  { return s.s.Read(p) }
func (s *quicStream) Write(p []byte) (int, error) { return s.s.Write(p) }
func (s *quicStream) Close() error                { return s.s.Close() }
func (s *quicStream) SetReadDeadline(t time.Time) error {
	return s.s.SetReadDeadline(t)
}
func (s *quicStream) SetWriteDeadline(t time.Time) error {
	return s.s.SetWriteDeadline(t)
}
func (s *quicStream) CancelRead(code uint64) {
	s.s.CancelRead(quic.StreamErrorCode(code))
}

func (l *quicListener) Accept(ctx context.Context) (Conn, error) {
	c, err := l.ln.Accept(ctx)
	if err != nil {
		return nil, err
	}
	return &quicConn{c: c}, nil
}

func (l *quicListener) Close() error {
	return l.ln.Close()
}
