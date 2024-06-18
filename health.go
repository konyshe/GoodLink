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
			health_stream.Write(m_send_data)
			gogo.Utils().TimeSleepSecond(1)
		}
	}()

	last_health_time := time.Now()

	go func() {
		for {
			if n, err := health_stream.Read(m_recv_data); err == nil && n > 0 {
				last_health_time = time.Now()
			}
		}
	}()

	go func() {
		for {
			if time.Since(last_health_time) >= 6*time.Second {
				log.Printf("process_health exit: %v\n", os.Args)
				os.Exit(0)
			}
			gogo.Utils().TimeSleepSecond(1)
		}
	}()
}
