package tunnel

import (
	"log"
	"os"
	"time"

	"github.com/quic-go/quic-go"
)

func process_health(health_stream quic.Stream, send_data, recv_data []byte) {
	go func() {
		for {
			health_stream.SetWriteDeadline(time.Now().Add(1 * time.Second))
			health_stream.Write(send_data)
			time.Sleep(1 * time.Second)
		}
	}()

	for {
		health_stream.SetReadDeadline(time.Now().Add(6 * time.Second))
		if n, err := health_stream.Read(recv_data); err != nil || n == 0 {
			log.Printf("process_health exit: %v\n", os.Args)
			break
		}
	}
}
