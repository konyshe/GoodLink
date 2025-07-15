package pool2

import (
	"sync"
)

var (
	bufPool32k sync.Pool
	bufPool16k sync.Pool
	bufPool8k  sync.Pool
	bufPool4k  sync.Pool
	bufPool2k  sync.Pool
	bufPool1k  sync.Pool
	bufPool512 sync.Pool
	bufPool256 sync.Pool
	bufPool    sync.Pool
)

func Malloc(size int) []byte {
	var x interface{}
	if size >= 32*1024 {
		x = bufPool32k.Get()
		if x == nil {
			return make([]byte, size)
		}
	} else if size >= 16*1024 {
		x = bufPool16k.Get()
	} else if size >= 8*1024 {
		x = bufPool8k.Get()
	} else if size >= 4*1024 {
		x = bufPool4k.Get()
	} else if size >= 2*1024 {
		x = bufPool2k.Get()
	} else if size >= 1*1024 {
		x = bufPool1k.Get()
	} else if size >= 512 {
		x = bufPool512.Get()
	} else if size >= 256 {
		x = bufPool256.Get()
	} else {
		x = bufPool.Get()
	}
	if x == nil {
		return make([]byte, size)
	}
	buf := x.([]byte)
	if cap(buf) < size {
		return make([]byte, size)
	}
	return buf[:size]
}

func Free(buf []byte) {
	size := cap(buf)
	if size >= 32*1024 {
		bufPool32k.Put(buf)
	} else if size >= 16*1024 {
		bufPool16k.Put(buf)
	} else if size >= 8*1024 {
		bufPool8k.Put(buf)
	} else if size >= 4*1024 {
		bufPool4k.Put(buf)
	} else if size >= 2*1024 {
		bufPool2k.Put(buf)
	} else if size >= 1*1024 {
		bufPool1k.Put(buf)
	} else if size >= 512 {
		bufPool512.Put(buf)
	} else if size >= 256 {
		bufPool256.Put(buf)
	} else {
		bufPool.Put(buf)
	}
}
