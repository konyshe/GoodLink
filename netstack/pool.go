package netstack

import (
	"sync"
)

const (
	// 头部缓冲区大小：1字节协议 + 4字节IP + 2字节端口 = 7字节
	headerBufferSize = 7
	// I/O缓冲区大小：用于读写操作
	ioBufferSize = 32 * 1024
)

var (
	// 头部缓冲池：用于TCP/UDP转发的协议头
	headerPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, headerBufferSize)
			return &buf
		},
	}

	// I/O缓冲池：用于数据读写操作
	ioBufferPool = sync.Pool{
		New: func() interface{} {
			buf := make([]byte, ioBufferSize)
			return &buf
		},
	}
)

// getHeaderBuffer 从池中获取头部缓冲区
func getHeaderBuffer() *[]byte {
	return headerPool.Get().(*[]byte)
}

// putHeaderBuffer 将头部缓冲区归还到池中
func putHeaderBuffer(buf *[]byte) {
	if buf != nil {
		headerPool.Put(buf)
	}
}

// getIOBuffer 从池中获取I/O缓冲区
func getIOBuffer() *[]byte {
	return ioBufferPool.Get().(*[]byte)
}

// putIOBuffer 将I/O缓冲区归还到池中
func putIOBuffer(buf *[]byte) {
	if buf != nil {
		ioBufferPool.Put(buf)
	}
}
