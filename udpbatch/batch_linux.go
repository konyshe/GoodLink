//go:build linux

package udpbatch

import (
	"io"
	"net"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

// batchConn 由 ipv4/ipv6.PacketConn 实现。两者的 Message 都是 socket.Message 的别名。
type batchConn interface {
	WriteBatch(ms []ipv4.Message, flags int) (int, error)
}

type linuxWriter struct {
	batch batchConn
	msgs  []ipv4.Message
	bufs  [][]byte
	addrs []net.UDPAddr
	raw   [][16]byte
}

// NewWriter 返回走 sendmmsg 的批量写。按本地地址族选择 ipv4 或 ipv6 路径。
func NewWriter(conn *net.UDPConn) Writer {
	var batch batchConn
	if ipv4Socket(conn) {
		batch = ipv4.NewPacketConn(conn)
	} else {
		batch = ipv6.NewPacketConn(conn)
	}
	return &linuxWriter{batch: batch}
}

// Wrap 原样返回。包装会让 kcp-go 和 quic-go 认不出 *net.UDPConn，退回逐包发送。
func Wrap(pc net.PacketConn) net.PacketConn { return pc }

func ipv4Socket(conn *net.UDPConn) bool {
	la, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || la == nil {
		return false
	}
	return la.IP.To4() != nil
}

func (w *linuxWriter) WriteBatch(msgs []Message) (int, error) {
	if len(msgs) == 0 {
		return 0, nil
	}
	w.grow(len(msgs))
	for i, m := range msgs {
		ip := m.Addr.Addr()
		if ip.Is4() {
			v4 := ip.As4()
			copy(w.raw[i][:4], v4[:])
			w.addrs[i].IP = w.raw[i][:4]
		} else {
			v6 := ip.As16()
			copy(w.raw[i][:], v6[:])
			w.addrs[i].IP = w.raw[i][:16]
		}
		w.addrs[i].Port = int(m.Addr.Port())
		w.addrs[i].Zone = ip.Zone()
		w.bufs[i] = m.Buf
		w.msgs[i] = ipv4.Message{
			Buffers: w.bufs[i : i+1],
			Addr:    &w.addrs[i],
		}
	}

	sent := 0
	for sent < len(msgs) {
		n, err := w.batch.WriteBatch(w.msgs[sent:], 0)
		if n > 0 {
			sent += n
		}
		if err != nil {
			return sent, err
		}
		if n == 0 {
			return sent, io.ErrShortWrite
		}
	}
	return sent, nil
}

func (w *linuxWriter) grow(n int) {
	if cap(w.msgs) < n {
		w.msgs = make([]ipv4.Message, n)
		w.bufs = make([][]byte, n)
		w.addrs = make([]net.UDPAddr, n)
		w.raw = make([][16]byte, n)
	}
	w.msgs = w.msgs[:n]
	w.bufs = w.bufs[:n]
	w.addrs = w.addrs[:n]
}
