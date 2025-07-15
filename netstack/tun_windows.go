//go:build windows

// Package netstack 提供了基于 WireGuard TUN 设备的网络栈实现
// 该包实现了虚拟网络接口，允许在用户空间处理网络数据包
// 主要用于实现 VPN 和网络代理功能
package netstack

import (
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

// 常量定义
const (
	offset     = 0 // 数据包偏移量，用于处理TUN_PI头，0表示不使用TUN_PI
	defaultMTU = 0 // 默认MTU值，0表示使用系统自动配置的MTU
)

// TUN 结构体实现了 TUN 设备的核心功能
// 它封装了 WireGuard 的 TUN 接口和基本的 I/O 操作
// 该结构体实现了 Device 接口，提供了与网络栈交互的能力
type TUN struct {
	*Endpoint // 嵌入Endpoint接口，提供网络栈接口功能，实现数据包的收发

	nt     *tun.NativeTun // 原生的 TUN 设备接口，用于与系统TUN设备交互
	mtu    uint32         // 最大传输单元，限制单个数据包的最大大小
	name   string         // TUN 设备名称，用于系统识别
	offset int            // 数据包偏移量，用于处理TUN_PI头

	// 使用读写锁替代互斥锁，允许并发读取
	rMutex sync.RWMutex // 读取读写锁，优化并发读取性能
	wMutex sync.Mutex   // 写入互斥锁，保护并发写入操作
	
	// 为每个goroutine分配独立的缓冲区池避免竞争
	rBuffPool sync.Pool // 读取缓冲区池
	wBuffPool sync.Pool // 写入缓冲区池
}

// Open 创建一个新的 TUN 设备
// 该函数负责初始化TUN设备，设置MTU，并创建必要的端点
// 参数:
//   - name: TUN 设备名称，用于系统识别
//   - mtu: 最大传输单元大小，如果为 0 则使用系统默认值
//
// 返回:
//   - Device: 实现了Device接口的TUN设备
//   - error: 创建过程中的错误信息
func Open(name string, mtu uint32) (_ Device, err error) {
	InitWintunDll()

	// 使用defer和recover处理可能的panic，确保错误被正确捕获和包装
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("open tun: %v", r)
		}
	}()

	// 初始化 TUN 结构体，设置基本参数
	t := &TUN{
		name:   name,              // 设置设备名称
		mtu:    mtu,               // 设置MTU值
		offset: offset,            // 设置数据包偏移量
	}
	
	// 初始化缓冲区池
	t.rBuffPool.New = func() interface{} {
		return &struct {
			buffs [][]byte
			sizes []int
		}{
			buffs: make([][]byte, 1),
			sizes: make([]int, 1),
		}
	}
	
	t.wBuffPool.New = func() interface{} {
		return make([][]byte, 1)
	}

	// 设置 MTU，如果指定了MTU则使用指定值，否则使用系统默认值
	forcedMTU := defaultMTU
	if t.mtu > 0 {
		forcedMTU = int(t.mtu)
	}

	// 创建 TUN 设备，使用WireGuard的tun包创建原生TUN接口
	nt, err := tun.CreateTUN(t.name, forcedMTU)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	t.nt = nt.(*tun.NativeTun) // 类型断言，确保使用NativeTun实现

	// 获取实际的 MTU 值，从系统获取TUN设备的实际MTU
	tunMTU, err := nt.MTU()
	if err != nil {
		return nil, fmt.Errorf("get mtu: %w", err)
	}
	t.mtu = uint32(tunMTU) // 更新为系统实际的MTU值

	// 创建 I/O 端点，用于处理数据包的收发
	ep, err := NewEndpoint(t, t.mtu, offset)
	if err != nil {
		return nil, fmt.Errorf("create endpoint: %w", err)
	}
	t.Endpoint = ep // 设置端点，使TUN设备能够与网络栈交互

	return t, nil
}

// Read 从 TUN 设备读取数据包
// 该方法实现了io.Reader接口，用于从TUN设备读取网络数据包
// 参数:
//   - packet: 用于存储读取到的数据包的缓冲区
//
// 返回:
//   - int: 读取的字节数
//   - error: 读取过程中的错误信息
func (t *TUN) Read(packet []byte) (int, error) {
	// 从池中获取缓冲区结构
	bufStruct := t.rBuffPool.Get().(*struct {
		buffs [][]byte
		sizes []int
	})
	defer t.rBuffPool.Put(bufStruct)
	
	bufStruct.buffs[0] = packet                                   // 设置接收缓冲区
	_, err := t.nt.Read(bufStruct.buffs, bufStruct.sizes, t.offset) // 从TUN设备读取数据
	return bufStruct.sizes[0], err                                // 返回读取的字节数和错误
}

// Name 返回 TUN 设备的名称
// 该方法实现了Device接口的Name方法
// 返回:
//   - string: TUN设备的名称
func (t *TUN) Name() string {
	name, _ := t.nt.Name() // 忽略错误，因为设备名称获取失败不影响主要功能
	return name
}

// Close 关闭 TUN 设备
// 该方法负责清理资源，确保TUN设备和相关端点被正确关闭
func (t *TUN) Close() {
	defer t.Endpoint.Close() // 确保端点被正确关闭，使用defer确保即使后续操作失败也能关闭端点
	t.nt.Close()             // 关闭TUN设备
}

// Write 向 TUN 设备写入数据包
// 该方法实现了io.Writer接口，用于向TUN设备写入网络数据包
// 参数:
//   - packet: 要发送的数据包
//
// 返回:
//   - int: 写入的字节数
//   - error: 写入过程中的错误信息
func (t *TUN) Write(packet []byte) (int, error) {
	// 从池中获取写入缓冲区
	buffs := t.wBuffPool.Get().([][]byte)
	defer t.wBuffPool.Put(buffs)
	
	buffs[0] = packet                 // 设置发送缓冲区
	return t.nt.Write(buffs, t.offset) // 向TUN设备写入数据
}

// Type 返回设备类型，用于标识这是一个TUN设备
// 该方法实现了Device接口的Type方法
// 返回:
//   - string: 设备类型标识符
func (t *TUN) Type() string {
	return "tun" // 返回固定的设备类型标识符
}

func (t *TUN) GetNt() *tun.NativeTun {
	return t.nt
}
