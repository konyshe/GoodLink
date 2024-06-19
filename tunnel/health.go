package main

import (
	"gogo"
	"log"
	"os"
	"time"

	"github.com/quic-go/quic-go"
)

func process_health(health_stream quic.Stream) {
	go func() {
		for {
			health_stream.SetWriteDeadline(time.Now().Add(1 * time.Second))
			health_stream.Write(m_send_data)
			gogo.Utils().TimeSleepSecond(1)
		}
	}()

	go func() {
		for {
			health_stream.SetReadDeadline(time.Now().Add(6 * time.Second))
			if n, err := health_stream.Read(m_recv_data); err != nil || n == 0 {
				log.Printf("process_health exit: %v\n", os.Args)
				os.Exit(0)
			}
		}
	}()
}
