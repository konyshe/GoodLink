package tls2

import (
	"context"
	"net"
	"time"

	kcp "github.com/xtaci/kcp-go/v5"
	"github.com/xtaci/smux"
)

func smuxConf() *smux.Config {
	cfg := smux.DefaultConfig()
	cfg.Version = 2
	cfg.KeepAliveInterval = 10 * time.Second
	cfg.KeepAliveTimeout = 120 * time.Second
	return cfg
}

// applyKCP 明文、无 FEC；stream 模式配合 smux。
func applyKCP(s *kcp.UDPSession) {
	s.SetStreamMode(true)
	s.SetNoDelay(1, 20, 2, 1)
	s.SetWindowSize(1024, 1024)
	s.SetACKNoDelay(true)
}

type kcpConn struct {
	sess     *smux.Session
	kcpSess  *kcp.UDPSession
	listener *kcp.Listener
}

type kcpStream struct {
	*smux.Stream
}

type kcpListener struct {
	ln *kcp.Listener
}

func (s *kcpStream) CancelRead(_ uint64) {}

func wrapKCP(sess *kcp.UDPSession, client bool, ln *kcp.Listener) (Conn, error) {
	applyKCP(sess)
	var (
		mux *smux.Session
		err error
	)
	if client {
		mux, err = smux.Client(sess, smuxConf())
	} else {
		mux, err = smux.Server(sess, smuxConf())
	}
	if err != nil {
		sess.Close()
		return nil, err
	}
	return &kcpConn{sess: mux, kcpSess: sess, listener: ln}, nil
}

func dialKCP(conn net.PacketConn, remoteAddr net.Addr) (Conn, error) {
	// block=nil：不加密；dataShards/parityShards=0：无 FEC
	sess, err := kcp.NewConn2(remoteAddr, nil, 0, 0, conn)
	if err != nil {
		return nil, err
	}
	return wrapKCP(sess, true, nil)
}

func listenKCP(conn net.PacketConn) (Listener, error) {
	ln, err := kcp.ServeConn(nil, 0, 0, conn)
	if err != nil {
		return nil, err
	}
	return &kcpListener{ln: ln}, nil
}

func (c *kcpConn) OpenStreamSync(ctx context.Context) (Stream, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	st, err := c.sess.OpenStream()
	if err != nil {
		return nil, err
	}
	return &kcpStream{Stream: st}, nil
}

func (c *kcpConn) AcceptStream(ctx context.Context) (Stream, error) {
	type result struct {
		st  *smux.Stream
		err error
	}
	ch := make(chan result, 1)
	go func() {
		st, err := c.sess.AcceptStream()
		ch <- result{st, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		return &kcpStream{Stream: r.st}, nil
	}
}

func (c *kcpConn) CloseWithError(_ uint64, _ string) error {
	if c.sess != nil {
		_ = c.sess.Close()
	}
	if c.kcpSess != nil {
		_ = c.kcpSess.Close()
	}
	if c.listener != nil {
		_ = c.listener.Close()
	}
	return nil
}

func (c *kcpConn) LocalAddr() net.Addr {
	if c.kcpSess != nil {
		return c.kcpSess.LocalAddr()
	}
	return nil
}

func (c *kcpConn) RemoteAddr() net.Addr {
	if c.kcpSess != nil {
		return c.kcpSess.RemoteAddr()
	}
	return nil
}

func (l *kcpListener) Accept(ctx context.Context) (Conn, error) {
	type result struct {
		sess *kcp.UDPSession
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		sess, err := l.ln.AcceptKCP()
		ch <- result{sess, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.err != nil {
			return nil, r.err
		}
		return wrapKCP(r.sess, false, l.ln)
	}
}

func (l *kcpListener) Close() error {
	return l.ln.Close()
}
