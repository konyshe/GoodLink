//go:build windows || darwin

package netstack

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	defaultOutQueueLen = 1 << 14

	dispatchRetryDelay    = 100 * time.Millisecond
	dispatchMaxRetryDelay = 5 * time.Second

	// Wintun/utun 支持一次读写多个包；32 在拷贝次数和延迟之间折中。
	tunBatchSize = 32
)

type batchIO interface {
	ReadBatch(bufs [][]byte, sizes []int, offset int) (int, error)
	WriteBatch(bufs [][]byte, offset int) (int, error)
}

type Endpoint struct {
	*channel.Endpoint

	rw     io.ReadWriter
	batch  batchIO
	mtu    uint32
	offset int

	once   sync.Once
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

func NewEndpoint(rw io.ReadWriter, mtu uint32, offset int) (*Endpoint, error) {
	if mtu == 0 {
		return nil, errors.New("MTU size is zero")
	}
	if rw == nil {
		return nil, errors.New("RW interface is nil")
	}
	if offset < 0 {
		return nil, errors.New("offset must be non-negative")
	}

	ep := &Endpoint{
		Endpoint: channel.New(
			defaultOutQueueLen,
			mtu,
			"",
		),
		rw:     rw,
		mtu:    mtu,
		offset: offset,
	}
	if b, ok := rw.(batchIO); ok {
		ep.batch = b
	}
	return ep, nil
}

func (e *Endpoint) Attach(dispatcher stack.NetworkDispatcher) {
	e.Endpoint.Attach(dispatcher)

	e.once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		e.cancel = cancel

		e.wg.Add(2)

		go func() {
			e.outboundLoop(ctx)
			e.wg.Done()
		}()

		go func() {
			e.dispatchLoop(cancel)
			e.wg.Done()
		}()
	})
}

func (e *Endpoint) Wait() {
	e.wg.Wait()
}

func isTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return true
		}
	}
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == syscall.EAGAIN || errno == syscall.EINTR || errno == syscall.EWOULDBLOCK
	}
	return false
}

func (e *Endpoint) injectIPv4(payload []byte) {
	if len(payload) == 0 || !e.IsAttached() {
		return
	}
	if header.IPVersion(payload) != header.IPv4Version {
		return
	}
	// 拷进 PacketBuffer 自有 View，读缓冲才能复用。直接 MakeWithData(切片)
	// 会让栈持有读缓冲，只能每包 make。
	view := buffer.NewView(len(payload))
	if _, err := view.Write(payload); err != nil {
		view.Release()
		return
	}
	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
		Payload: buffer.MakeWithView(view),
	})
	e.InjectInbound(header.IPv4ProtocolNumber, pkt)
	pkt.DecRef()
}

func (e *Endpoint) dispatchLoop(cancel context.CancelFunc) {
	defer cancel()

	if e.batch != nil {
		e.dispatchBatch()
		return
	}

	offset, mtu := e.offset, int(e.mtu)
	retryDelay := dispatchRetryDelay
	data := make([]byte, offset+mtu)

	for {
		n, err := e.rw.Read(data)
		if err != nil {
			if !isTemporaryError(err) {
				log.Printf("[netstack] dispatchLoop 致命错误，退出: %v", err)
				return
			}
			log.Printf("[netstack] dispatchLoop 读取临时错误，%v 后重试: %v", retryDelay, err)
			time.Sleep(retryDelay)
			retryDelay = min(retryDelay*2, dispatchMaxRetryDelay)
			continue
		}
		retryDelay = dispatchRetryDelay

		if n == 0 || n > mtu {
			continue
		}
		e.injectIPv4(data[offset : offset+n])
	}
}

func (e *Endpoint) dispatchBatch() {
	offset, mtu := e.offset, int(e.mtu)
	retryDelay := dispatchRetryDelay

	bufs := make([][]byte, tunBatchSize)
	sizes := make([]int, tunBatchSize)
	for i := range bufs {
		bufs[i] = make([]byte, offset+mtu)
	}

	for {
		n, err := e.batch.ReadBatch(bufs, sizes, offset)
		if err != nil && n == 0 {
			if !isTemporaryError(err) {
				log.Printf("[netstack] dispatchLoop 致命错误，退出: %v", err)
				return
			}
			log.Printf("[netstack] dispatchLoop 读取临时错误，%v 后重试: %v", retryDelay, err)
			time.Sleep(retryDelay)
			retryDelay = min(retryDelay*2, dispatchMaxRetryDelay)
			continue
		}
		retryDelay = dispatchRetryDelay

		for i := 0; i < n; i++ {
			pktLen := sizes[i]
			if pktLen == 0 || pktLen > mtu {
				continue
			}
			e.injectIPv4(bufs[i][offset : offset+pktLen])
		}
	}
}

func copyPktTo(dst []byte, pkt *stack.PacketBuffer) int {
	vl, hdrOff := pkt.AsViewList()
	skipped := 0
	n := 0
	for v := vl.Front(); v != nil; v = v.Next() {
		s := v.AsSlice()
		if skipped < hdrOff {
			if skipped+len(s) <= hdrOff {
				skipped += len(s)
				continue
			}
			s = s[hdrOff-skipped:]
			skipped = hdrOff
		}
		if n >= len(dst) {
			break
		}
		n += copy(dst[n:], s)
	}
	return n
}

func (e *Endpoint) packPacket(pkt *stack.PacketBuffer, dest []byte) []byte {
	need := e.offset + pkt.Size()
	if cap(dest) < need {
		dest = make([]byte, need)
	} else {
		dest = dest[:need]
	}
	_ = copyPktTo(dest[e.offset:], pkt)
	return dest
}

func (e *Endpoint) outboundLoop(ctx context.Context) {
	scratch := make([][]byte, tunBatchSize)
	pkts := make([]*stack.PacketBuffer, 0, tunBatchSize)
	bufs := make([][]byte, 0, tunBatchSize)
	for i := range scratch {
		scratch[i] = make([]byte, e.offset+int(e.mtu))
	}

	for {
		pkt := e.ReadContext(ctx)
		if pkt == nil {
			break
		}

		pkts = pkts[:0]
		bufs = bufs[:0]
		pkts = append(pkts, pkt)
		bufs = append(bufs, e.packPacket(pkt, scratch[0]))

		for i := 1; i < tunBatchSize; i++ {
			p := e.Read()
			if p == nil {
				break
			}
			pkts = append(pkts, p)
			bufs = append(bufs, e.packPacket(p, scratch[i]))
		}

		if e.batch != nil {
			if _, err := e.batch.WriteBatch(bufs, e.offset); err != nil {
				log.Printf("[netstack] writePacket 写入TUN失败: %v", err)
			}
		} else {
			for _, buf := range bufs {
				if _, err := e.rw.Write(buf); err != nil {
					log.Printf("[netstack] writePacket 写入TUN失败: %v", err)
					break
				}
			}
		}

		for _, p := range pkts {
			p.DecRef()
		}
	}
}

func (e *Endpoint) Close() {
	if e.cancel != nil {
		e.cancel()
	}
	e.Endpoint.Close()
}
