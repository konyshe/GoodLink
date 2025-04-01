//go:build windows

package netstack

import (
	"context"
	"errors"
	"io"
	"sync"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	// Queue length for outbound packet, arriving for read. Overflow
	// causes packet drops.
	defaultOutQueueLen = 1 << 10
)

// Endpoint implements the interface of stack.LinkEndpoint from io.ReadWriter.
type Endpoint struct {
	*channel.Endpoint

	// rw is the io.ReadWriter for reading and writing packets.
	rw io.ReadWriter

	// mtu (maximum transmission unit) is the maximum size of a packet.
	mtu uint32

	// offset can be useful when perform TUN device I/O with TUN_PI enabled.
	offset int

	// once is used to perform the init action once when attaching.
	once sync.Once

	// wg keeps track of running goroutines.
	wg sync.WaitGroup
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
			defaultOutQueueLen, // 发包队列长度（1024）
			mtu,                // 最大传输单元
			"",                 // 链路层名称（保留空）
		),
		rw:     rw,     // 底层IO读写接口
		mtu:    mtu,    // 保存MTU配置
		offset: offset, // TUN设备头偏移量
	}, nil
}

// Attach启动从io读取数据包的例程。读者和 //通过提供的分派器分派它们。
// 调用Attach后，Endpoint将开始从底层IO接口读取数据包并
func (e *Endpoint) Attach(dispatcher stack.NetworkDispatcher) {
	// 调用基类方法附加网络调度器
	e.Endpoint.Attach(dispatcher)

	// 使用sync.Once确保初始化逻辑只执行一次
	e.once.Do(func() {
		// 创建带取消功能的上下文（用于goroutine退出控制）
		ctx, cancel := context.WithCancel(context.Background())

		// 设置等待组计数器（两个后台goroutine）
		e.wg.Add(2)

		// 启动发包处理协程
		go func() {
			e.outboundLoop(ctx) // 处理出站数据包
			e.wg.Done()         // 协程结束计数器减1
		}()

		// 启动收包处理协程
		go func() {
			e.dispatchLoop(cancel) // 处理入站数据包（传递cancel函数）
			e.wg.Done()            // 协程结束计数器减1
		}()
	})
}

func (e *Endpoint) Wait() {
	e.wg.Wait()
}

// dispatchLoop dispatches packets to upper layer.
func (e *Endpoint) dispatchLoop(cancel context.CancelFunc) {
	// 确保退出时取消上下文，通知outboundLoop终止
	defer cancel()

	// 获取配置参数：数据偏移量和MTU值
	offset, mtu := e.offset, int(e.mtu)

	// 数据包接收主循环
	for {
		// 创建带偏移量的接收缓冲区（用于TUN设备头）
		data := make([]byte, offset+mtu)

		// 从IO接口读取原始数据
		n, err := e.rw.Read(data)
		if err != nil {
			break // 发生致命错误时退出循环
		}

		// 过滤无效数据包：空包或超过MTU大小的包
		if n == 0 || n > mtu {
			continue
		}

		// 检查端点是否已附加到协议栈
		if !e.IsAttached() {
			continue /* 未附加时丢弃数据包 */
		}

		// 创建协议栈数据包缓冲区
		// 从偏移量开始截取有效载荷（跳过TUN头）
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload: buffer.MakeWithData(data[offset : offset+n]),
		})

		// 根据IP版本分发数据包
		switch header.IPVersion(data[offset:]) {
		case header.IPv4Version:
			e.InjectInbound(header.IPv4ProtocolNumber, pkt) // 注入IPv4协议栈
		case header.IPv6Version:
			//e.InjectInbound(header.IPv6ProtocolNumber, pkt) // 注入IPv6协议栈
		}
		pkt.DecRef() // 释放数据包引用计数
	}
}

// outboundLoop reads outbound packets from channel, and then it calls
// writePacket to send those packets back to lower layer.
func (e *Endpoint) outboundLoop(ctx context.Context) {
	// 出站数据包处理主循环
	for {
		// 从通道读取待发送数据包（支持上下文取消）
		pkt := e.ReadContext(ctx)

		// 读取到nil表示通道已关闭或上下文取消
		if pkt == nil {
			break
		}

		// 将数据包写入底层IO接口
		e.writePacket(pkt)
	}
}

// writePacket writes outbound packets to the io.Writer.
func (e *Endpoint) writePacket(pkt *stack.PacketBuffer) tcpip.Error {
	// 确保数据包引用计数最终释放
	defer pkt.DecRef()

	// 转换数据包为可写缓冲区
	buf := pkt.ToBuffer()
	// 确保缓冲区资源最终释放
	defer buf.Release()

	// 添加TUN设备头偏移（当offset非0时）
	if e.offset != 0 {
		// 创建空白头部缓冲区（长度等于offset）
		v := buffer.NewViewWithData(make([]byte, e.offset))
		// 将头部预置到数据缓冲区前
		_ = buf.Prepend(v)
	}

	// 将数据写入底层设备（如TUN接口）
	if _, err := e.rw.Write(buf.Flatten()); err != nil {
		// 返回端点状态错误（写入失败时）
		return &tcpip.ErrInvalidEndpointState{}
	}
	return nil
}
