package proxy

import (
	"io"
	"net"
	"sync"

	"github.com/quic-go/quic-go"
)

var (
	bufPool sync.Pool
)

func GetBuf(size int) []byte {
	x := bufPool.Get()
	if x == nil {
		return make([]byte, size)
	}
	buf := x.([]byte)
	if cap(buf) < size {
		return make([]byte, size)
	}
	return buf[:size]
}

func PutBuf(buf []byte) {
	bufPool.Put(buf)
}

func stunT2QProcess1(tc net.Conn, qc quic.Stream, stun_quic_conn quic.Connection) {
	buf := GetBuf(1500)
	defer func() {
		PutBuf(buf)
		qc.Close()
		tc.Close()
	}()

	for {
		if _, err := io.CopyBuffer(tc, qc, buf); err != nil {
			tc.Close()
			break
		}
	}
}

func stunQ2TProcess1(qc quic.Stream, tc net.Conn, stun_quic_conn quic.Connection) {
	buf := GetBuf(1500)
	defer func() {
		PutBuf(buf)
		qc.Close()
		tc.Close()
	}()

	for {
		if _, err := io.CopyBuffer(qc, tc, buf); err != nil {
			qc.Close()
			break
		}
	}
}
