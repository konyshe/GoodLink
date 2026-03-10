package proxy

import (
	"context"
	"encoding/binary"
	go2pool "go2/pool"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// Local端
func ProcessProxyClient(listener net.Listener, stun_quic_conn *quic.Conn) {
	for {
		new_tcp_conn, err := listener.Accept()
		if err != nil {
			log.Println("接受连接失败:", err)
			break
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		new_quic_stream, err := stun_quic_conn.OpenStreamSync(ctx)
		cancel()
		if err != nil {
			log.Println("打开流失败:", err)
			new_tcp_conn.Close()
			break
		}

		// 批量构建头部数据：协议类型(1字节) + IP地址(4字节) + 端口(2字节)
		// 使用缓冲池获取头部缓冲区
		ioBuf := go2pool.Malloc(HEAD_LEN)
		defer go2pool.Free(ioBuf)

		ioBuf[0] = 0x00 // TCP协议标识

		// 写入IPv4地址
		ipv4Bytes := new_tcp_conn.LocalAddr().(*net.TCPAddr).IP.To4()
		copy(ioBuf[1:5], ipv4Bytes[:])

		// 写入端口（大端序）
		binary.BigEndian.PutUint16(ioBuf[5:HEAD_LEN], uint16(PROXY_PORT))

		// 一次性写入所有头部数据
		if _, err := new_quic_stream.Write(ioBuf[:HEAD_LEN]); err != nil {
			log.Println("写入头部失败", err)
			new_tcp_conn.Close()
			new_quic_stream.CancelRead(0)
			new_quic_stream.Close()
			return
		}

		go ForwardT2Q(new_tcp_conn, new_quic_stream)
		go ForwardQ2T(new_quic_stream, new_tcp_conn)
	}
}
