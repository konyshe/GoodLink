//go:build !linux && !windows

package udpbatch

import "net"

type genericWriter struct {
	conn *net.UDPConn
}

// NewWriter 逐包 WriteTo。darwin 等平台没有 sendmmsg/USO。
func NewWriter(conn *net.UDPConn) Writer {
	return genericWriter{conn: conn}
}

// Wrap 原样返回。
func Wrap(pc net.PacketConn) net.PacketConn { return pc }

func (w genericWriter) WriteBatch(msgs []Message) (int, error) {
	for i, m := range msgs {
		if err := writeMsg(w.conn, m); err != nil {
			return i, err
		}
	}
	return len(msgs), nil
}
