package tunnel

import (
	"time"

	"github.com/quic-go/quic-go"
)

func process_health(health_stream quic.Stream) {
	go func() {
		for health_stream != nil {
			health_stream.SetWriteDeadline(time.Now().Add(1 * time.Second))
			health_stream.Write(m_send_data)
			time.Sleep(1 * time.Second)
		}
	}()

	for health_stream != nil {
		health_stream.SetReadDeadline(time.Now().Add(3 * time.Second))
		if n, err := health_stream.Read(m_recv_data); err != nil || n == 0 {
			break
		}
	}
}
