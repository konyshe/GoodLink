package main

import (
	"gogo"
	"goodlink/tunnel"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	//_ "net/http/pprof"
)

func main2() {
	if *mp_cli_pprof {
		go log.Println(http.ListenAndServe("0.0.0.0:6060", nil))
	}

	if m_cli_tun_remote_addr != "" {
		go tunnel.ProcessServer(m_cli_tun_remote_addr, m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key)

	} else if m_cli_tun_local_addr != "" {
		go func() {
			if err := tunnel.ProcessClient(m_cli_tun_local_addr, m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key, true); err != nil {
				log.Println(err)
				os.Exit(0)
			}
		}()
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	log.Println("main2 end")
}

func main() {
	help()

	gogo.GuardStart(main2, func(err error) {
		// if 0: err==nil; -1: err==255; -2: err==254; err==1: 1; err==2
		if err != nil {
			gogo.Log().ErrorF("发现导致重启的错误: %v", err)
		}
	})
}
