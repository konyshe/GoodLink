package tls2

import (
	"context"
	"io"
	"net"
	"time"
)

// Conn 打洞成功后的多路复用会话（QUIC 或 KCP+smux）。
type Conn interface {
	OpenStreamSync(ctx context.Context) (Stream, error)
	AcceptStream(ctx context.Context) (Stream, error)
	CloseWithError(code uint64, msg string) error
	LocalAddr() net.Addr
	RemoteAddr() net.Addr
}

// Stream 一条转发/心跳流。CancelRead 在 smux 上为空操作。
type Stream interface {
	io.ReadWriteCloser
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	CancelRead(code uint64)
}

// Listener 被动端接受一条会话。
type Listener interface {
	Accept(ctx context.Context) (Conn, error)
	Close() error
}
