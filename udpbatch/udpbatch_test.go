package udpbatch

import (
	"encoding/binary"
	"net"
	"net/netip"
	"runtime"
	"testing"
	"time"
)

func TestGroupRun(t *testing.T) {
	a := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 1)
	b := netip.AddrPortFrom(netip.MustParseAddr("127.0.0.1"), 2)
	msg := func(addr netip.AddrPort, n int) Message {
		return Message{Buf: make([]byte, n), Addr: addr}
	}
	cases := []struct {
		name         string
		msgs         []Message
		count, total int
	}{
		{name: "empty", count: 0, total: 0},
		{name: "one", msgs: []Message{msg(a, 100)}, count: 1, total: 100},
		{name: "zero len stays single", msgs: []Message{msg(a, 0), msg(a, 0)}, count: 1, total: 0},
		{name: "same size", msgs: []Message{msg(a, 100), msg(a, 100), msg(a, 100)}, count: 3, total: 300},
		{name: "short tail", msgs: []Message{msg(a, 100), msg(a, 100), msg(a, 40)}, count: 3, total: 240},
		{name: "short ends run", msgs: []Message{msg(a, 100), msg(a, 40), msg(a, 100)}, count: 2, total: 140},
		{name: "longer breaks", msgs: []Message{msg(a, 100), msg(a, 101)}, count: 1, total: 100},
		{name: "addr breaks", msgs: []Message{msg(a, 100), msg(b, 100)}, count: 1, total: 100},
		{
			name:  "payload cap",
			msgs:  []Message{msg(a, 30000), msg(a, 30000), msg(a, 5001)},
			count: 2,
			total: 60000,
		},
		{
			name:  "payload exact",
			msgs:  []Message{msg(a, 30000), msg(a, 30000), msg(a, 5000)},
			count: 3,
			total: 65000,
		},
		{name: "oversize alone", msgs: []Message{msg(a, maxCoalescedPayload+1), msg(a, 100)}, count: 1, total: maxCoalescedPayload + 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			count, total := groupRun(tc.msgs, maxCoalescedPayload)
			if count != tc.count || total != tc.total {
				t.Fatalf("groupRun() = (%d, %d), want (%d, %d)", count, total, tc.count, tc.total)
			}
		})
	}
}

func TestWriteBatchEmpty(t *testing.T) {
	conn := listenUDP(t)
	w := NewWriter(conn)
	n, err := w.WriteBatch(nil)
	if err != nil || n != 0 {
		t.Fatalf("WriteBatch(nil) = %d, %v", n, err)
	}
}

func TestWriteBatchLoopback(t *testing.T) {
	rx1 := listenUDP(t)
	rx2 := listenUDP(t)
	tx := listenUDP(t)
	_ = rx1.SetReadBuffer(4 << 20)
	_ = rx2.SetReadBuffer(4 << 20)
	_ = tx.SetWriteBuffer(4 << 20)

	dst1 := rx1.LocalAddr().(*net.UDPAddr).AddrPort()
	dst2 := rx2.LocalAddr().(*net.UDPAddr).AddrPort()

	type item struct {
		id   int
		size int
		dst  netip.AddrPort
	}
	var plan []item
	add := func(dst netip.AddrPort, size, n int) {
		for i := 0; i < n; i++ {
			plan = append(plan, item{id: len(plan), size: size, dst: dst})
		}
	}
	add(dst1, 1, 1)
	add(dst1, 64, 3)
	add(dst1, 120, 20) // 同长度，走 sendmmsg / USO
	add(dst1, 120, 1)
	add(dst1, 40, 1) // 更短的尾包
	add(dst2, 200, 8)
	add(dst1, 1000, 2)

	msgs := make([]Message, len(plan))
	for i, it := range plan {
		msgs[i] = Message{Buf: stamp(it.id, it.size), Addr: it.dst}
	}
	w := NewWriter(tx)
	n, err := w.WriteBatch(msgs)
	if err != nil {
		t.Fatalf("WriteBatch: %v", err)
	}
	if n != len(msgs) {
		t.Fatalf("WriteBatch wrote %d, want %d", n, len(msgs))
	}

	got1 := map[int]int{}
	got2 := map[int]int{}
	collect(t, rx1, got1)
	collect(t, rx2, got2)
	if len(got1)+len(got2) != len(plan) {
		t.Fatalf("received %d packets, want %d", len(got1)+len(got2), len(plan))
	}
	for _, it := range plan {
		got := got1
		if it.dst == dst2 {
			got = got2
		}
		if got[it.id] != it.size {
			t.Fatalf("id %d on %v size %d, want %d", it.id, it.dst, got[it.id], it.size)
		}
	}
}

func TestWrapPassthrough(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("windows wraps when USO is available")
	}
	conn := listenUDP(t)
	if got := Wrap(conn); got != conn {
		t.Fatalf("Wrap = %T, want the original *net.UDPConn", got)
	}
	other := notUDP{}
	if got := Wrap(other); got != other {
		t.Fatalf("Wrap(non-UDP) = %v", got)
	}
}

type notUDP struct{ net.PacketConn }

func listenUDP(t *testing.T) *net.UDPConn {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func stamp(id, size int) []byte {
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(id)
	}
	if size >= 2 {
		binary.BigEndian.PutUint16(b[:2], uint16(id))
	}
	return b
}

func collect(t *testing.T, conn *net.UDPConn, got map[int]int) {
	t.Helper()
	buf := make([]byte, 2048)
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return
			}
			t.Fatalf("read: %v", err)
		}
		if n == 0 {
			t.Fatal("empty datagram")
		}
		id := int(buf[0])
		if n >= 2 {
			id = int(binary.BigEndian.Uint16(buf[:2]))
		}
		for i := 2; i < n; i++ {
			if buf[i] != byte(id) {
				t.Fatalf("id %d byte %d = %d", id, i, buf[i])
			}
		}
		if _, dup := got[id]; dup {
			t.Fatalf("duplicate id %d", id)
		}
		got[id] = n
	}
}
