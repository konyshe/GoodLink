package netstack

import (
	"bytes"
	go2pool "go2/pool"
	"log"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/waiter"
)

func handleConnection(ep tcpip.Endpoint, wq *waiter.Queue) {
	// 确保连接最终关闭
	defer ep.Close()

	// 创建等待队列用于读取事件通知
	// var wq waiter.Queue

	// 创建可读事件监听器（waiter.Entry）
	// EventIn: 当端点有数据可读时触发
	waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventIn)
	wq.EventRegister(&waitEntry)
	// 退出时自动取消事件注册
	defer wq.EventUnregister(&waitEntry)

	// 使用缓冲池获取I/O缓冲区，减少内存分配
	ioBuf := go2pool.Malloc(32 * 1024)
	defer go2pool.Free(ioBuf)

	buf := bytes.NewBuffer(ioBuf)

	// 连接处理主循环
	for {
		buf.Reset()
		// 尝试读取数据（非阻塞模式）
		_, err := ep.Read(buf, tcpip.ReadOptions{})
		if err != nil {
			switch err.(type) {
			case *tcpip.ErrWouldBlock: // 处理暂时无数据的情况
				<-notifyCh // 等待可读事件通知
				continue
			case *tcpip.ErrNotConnected: // 处理连接关闭的情况
				log.Println("Connection closed") // 打印连接关闭信息
				return                           // 结束连接处理
			}
			log.Println(err) // 打印错误信息
			return           // 其他错误直接返回
		}

		// 回显数据：将接收到的数据原样写回
		var r bytes.Reader
		r.Reset(buf.Bytes()) // 将缓冲区数据载入读取器
		// 写入数据到TCP连接
		_, err = ep.Write(&r, tcpip.WriteOptions{})
		if err != nil {
			return // 写入失败时结束连接
		}
	}
}

func startTCPServer(s *stack.Stack) {
	// 创建事件等待队列（用于异步I/O通知）
	var wq waiter.Queue

	// 创建TCP端点（相当于socket）
	// 参数说明：
	// - tcp.ProtocolNumber: TCP协议号(6)
	// - ipv4.ProtocolNumber: IPv4协议号(0x0800)
	// - &wq: 关联的等待队列
	ep, err := s.NewEndpoint(tcp.ProtocolNumber, ipv4.ProtocolNumber, &wq)
	if err != nil {
		panic(err)
	}

	// 绑定到指定网络接口（NIC 1）和端口
	// NIC: 网络接口控制器编号（虚拟网卡）
	if err := ep.Bind(tcpip.FullAddress{
		NIC: 1,
		// Addr: tcpip.AddrFromSlice(net.ParseIP("192.168.3.3").To4()),
		Addr: tcpip.Address{}, // 零值表示所有IP地址
		Port: 80,
	}); err != nil {
		panic(err)
	}

	// 启动监听，设置最大等待连接队列长度
	if err := ep.Listen(10); err != nil {
		panic(err)
	}

	// 持续接受新连接的循环
	for {
		var addr tcpip.FullAddress
		// 非阻塞式接受连接
		newEP, waitq, err := ep.Accept(&addr)
		if err != nil {
			switch err.(type) {
			case *tcpip.ErrWouldBlock: // 处理暂时无连接的情况
				// 注册读事件监听
				waitEntry, notifyCh := waiter.NewChannelEntry(waiter.EventIn)
				wq.EventRegister(&waitEntry)
				// 阻塞等待直到新连接到达
				<-notifyCh
				wq.EventUnregister(&waitEntry)
				continue
			case *tcpip.ErrInvalidEndpointState: // 处理端点状态错误
				continue
			default:
				log.Println(err)
				continue
			}
		}

		log.Printf("Accepted %s:%d\n", addr.Addr, addr.Port)

		// 获取新连接的端点
		go handleConnection(newEP, waitq) // 传递正确的连接端点
	}
}
