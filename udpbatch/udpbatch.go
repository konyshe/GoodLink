// Package udpbatch 批量发送 UDP 数据报，用一次系统调用交出多包，降低发送路径的 CPU 开销。
//
// Linux 的 Writer 走 sendmmsg。Wrap 在 Linux 上原样返回，数据面仍把 *net.UDPConn
// 交给 kcp-go（sendmmsg）和 quic-go（GSO）。
//
// Windows 在系统支持 UDP Segmentation Offload 时，Wrap 返回异步合并写的
// PacketConn：连续同目的地、同长度的报文合成一次 WSASendMsg，用 UDP_SEND_MSG_SIZE
// 指定段长。
package udpbatch

import (
	"io"
	"net"
	"net/netip"
)

// 单次 USO 合并的载荷上限。留出 IP/UDP 头，避免顶满 65535。
const maxCoalescedPayload = 65000

// Message 是一条待发送的 UDP 数据报。
type Message struct {
	Buf  []byte
	Addr netip.AddrPort
}

// Writer 把多条数据报尽量合成更少的系统调用发出。
// WriteBatch 返回成功写入的报文条数。同一 Writer 不要并发调用。
type Writer interface {
	WriteBatch(msgs []Message) (int, error)
}

// groupRun 计算 msgs 开头有多少条可以合成一次 USO/GSO。
// 除最后一条外，每条长度都必须等于第一条；最后一条可以更短。
// 拼接后的载荷不超过 maxPayload。空数据报不能参与合并。
func groupRun(msgs []Message, maxPayload int) (count, total int) {
	if len(msgs) == 0 {
		return 0, 0
	}
	seg := len(msgs[0].Buf)
	addr := msgs[0].Addr
	if seg == 0 || seg > maxPayload {
		return 1, seg
	}
	total = seg
	count = 1
	for i := 1; i < len(msgs); i++ {
		m := msgs[i]
		n := len(m.Buf)
		if m.Addr != addr || n == 0 || n > seg || total+n > maxPayload {
			break
		}
		count++
		total += n
		if n < seg {
			break
		}
	}
	return count, total
}

func writeMsg(conn *net.UDPConn, m Message) error {
	n, err := conn.WriteToUDPAddrPort(m.Buf, m.Addr)
	if err != nil {
		return err
	}
	if n != len(m.Buf) {
		return io.ErrShortWrite
	}
	return nil
}
