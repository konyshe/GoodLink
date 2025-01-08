package tun

import (
	"log"
	"time"

	"github.com/quic-go/quic-go"
)

func ProcessHealth(health_stream quic.Stream) {
	go func() {
		for health_stream != nil {
			for {
				health_stream.SetWriteDeadline(time.Now().Add(time.Millisecond * 100))
				if _, err := health_stream.Write(m_send_data); err == nil {
					break
				}
			}
			health_stream.SetWriteDeadline(time.Time{})
			time.Sleep(500 * time.Millisecond)
		}
	}()

	time.Sleep(3 * time.Second)

	for health_stream != nil {
		health_stream.SetReadDeadline(time.Now().Add(time.Second * 3))
		if _, err := health_stream.Read(m_recv_data); err != nil {
			log.Printf("   直连异常: %v", err)
			break
		}
	}
}
