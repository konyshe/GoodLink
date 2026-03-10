//go:build darwin

package netstack

import (
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/tun"
)

const (
	// macOS utun 需要 4 字节 AF 头
	offset     = 4
	defaultMTU = 0
)

type TUN struct {
	*Endpoint

	nt     *tun.NativeTun
	mtu    uint32
	name   string
	offset int

	rMutex sync.RWMutex
	wMutex sync.Mutex

	rBuffPool sync.Pool
	wBuffPool sync.Pool
}

func Open(name string, mtu uint32) (_ Device, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("open tun: %v", r)
		}
	}()

	t := &TUN{
		name:   name,
		mtu:    mtu,
		offset: offset,
	}

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

	forcedMTU := defaultMTU
	if t.mtu > 0 {
		forcedMTU = int(t.mtu)
	}

	// macOS utun 设备名称由系统分配（utun0, utun1, ...）
	nt, err := tun.CreateTUN("utun", forcedMTU)
	if err != nil {
		return nil, fmt.Errorf("create tun: %w", err)
	}
	t.nt = nt.(*tun.NativeTun)

	actualName, err := t.nt.Name()
	if err != nil {
		t.nt.Close()
		return nil, fmt.Errorf("get tun name: %w", err)
	}
	t.name = actualName

	tunMTU, err := nt.MTU()
	if err != nil {
		t.nt.Close()
		return nil, fmt.Errorf("get mtu: %w", err)
	}
	t.mtu = uint32(tunMTU)

	ep, err := NewEndpoint(t, t.mtu, offset)
	if err != nil {
		t.nt.Close()
		return nil, fmt.Errorf("create endpoint: %w", err)
	}
	t.Endpoint = ep

	return t, nil
}

func (t *TUN) Read(packet []byte) (int, error) {
	bufStruct := t.rBuffPool.Get().(*struct {
		buffs [][]byte
		sizes []int
	})
	defer t.rBuffPool.Put(bufStruct)

	bufStruct.buffs[0] = packet
	_, err := t.nt.Read(bufStruct.buffs, bufStruct.sizes, t.offset)
	return bufStruct.sizes[0], err
}

func (t *TUN) Write(packet []byte) (int, error) {
	buffs := t.wBuffPool.Get().([][]byte)
	defer t.wBuffPool.Put(buffs)

	buffs[0] = packet
	return t.nt.Write(buffs, t.offset)
}

func (t *TUN) Name() string {
	name, _ := t.nt.Name()
	return name
}

func (t *TUN) Close() {
	defer t.Endpoint.Close()
	t.nt.Close()
}

func (t *TUN) Type() string {
	return "tun"
}
