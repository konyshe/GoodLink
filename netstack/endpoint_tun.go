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
	"time"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	defaultOutQueueLen = 1 << 14 // 16384, 避免出站队列溢出丢包

	dispatchRetryDelay    = 100 * time.Millisecond
	dispatchMaxRetryDelay = 5 * time.Second
)

// Endpoint implements the interface of stack.LinkEndpoint from io.ReadWriter.
type Endpoint struct {
	*channel.Endpoint

	rw     io.ReadWriter
	mtu    uint32
	offset int

	once   sync.Once
	wg     sync.WaitGroup
	cancel context.CancelFunc
}

// New returns stack.LinkEndpoint(.*Endpoint) and error.
func NewEndpoint(rw io.ReadWriter, mtu uint32, offset int) (*Endpoint, error) {
	// 校验MTU（最大传输单元）有效性
	if mtu == 0 {
		return nil, errors.New("MTU size is zero")
	}

	// 确保IO接口有效
	if rw == nil {
		return nil, errors.New("RW interface is nil")
	}

	// 校验偏移量合法性（用于TUN设备头）
	if offset < 0 {
		return nil, errors.New("offset must be non-negative")
	}

	// 创建并初始化端点实例
	return &Endpoint{
		Endpoint: channel.New(
			defaultOutQueueLen,
			mtu,                // 最大传输单元
			"",                 // 链路层名称（保留空）
		),
		rw:     rw,     // 底层IO读写接口
		mtu:    mtu,    // 保存MTU配置
		offset: offset, // TUN设备头偏移量
	}, nil
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
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return true
}

func (e *Endpoint) dispatchLoop(cancel context.CancelFunc) {
	defer cancel()

	offset, mtu := e.offset, int(e.mtu)
	retryDelay := dispatchRetryDelay

	for {
		// 每次读取分配独立缓冲区，避免 InjectInbound 异步引用被覆盖
		data := make([]byte, offset+mtu)

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

		if !e.IsAttached() {
			continue
		}

		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(data[offset : offset+n]),
		})

		switch header.IPVersion(data[offset:]) {
		case header.IPv4Version:
			e.InjectInbound(header.IPv4ProtocolNumber, pkt)
		}
		pkt.DecRef()
	}
}

func (e *Endpoint) outboundLoop(ctx context.Context) {
	for {
		pkt := e.ReadContext(ctx)
		if pkt == nil {
			break
		}
		e.writePacket(pkt)
	}
}

func (e *Endpoint) writePacket(pkt *stack.PacketBuffer) tcpip.Error {
	defer pkt.DecRef()

	buf := pkt.ToBuffer()
	defer buf.Release()

	if e.offset != 0 {
		v := buffer.NewViewWithData(make([]byte, e.offset))
		_ = buf.Prepend(v)
	}

	if _, err := e.rw.Write(buf.Flatten()); err != nil {
		log.Printf("[netstack] writePacket 写入TUN失败: %v", err)
		return &tcpip.ErrInvalidEndpointState{}
	}
	return nil
}

func (e *Endpoint) Close() {
	if e.cancel != nil {
		e.cancel()
	}
	e.Endpoint.Close()
}
