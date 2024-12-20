package main

import (
	"gogo"
	"goodlink/process"
	_ "goodlink/process"
	"goodlink/stun2"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main2() {
	if *m_cli_stun_test {
		stun2.TestStun()
		os.Exit(0)
	}

	if m_cli_pprof_addr != "" {
		go log.Println(http.ListenAndServe(m_cli_pprof_addr, nil))
	}

	if m_cli_stun_svr_addr != "" {
		stun2.StartSvr(m_cli_stun_svr_addr, m_cli_stun_svr_port)
		os.Exit(0)
	}

	process.Init(m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key)

	if m_cli_tun_local_addr != "" {
		go func() {
			if err := process.RunLocal(m_cli_tun_local_addr, m_cli_tun_key, true); err != nil {
				log.Println(err)
				os.Exit(0)
			}
		}()
	} else {
		go process.RunRemote(m_cli_tun_remote_addr, m_cli_tun_key, time.Duration(m_cli_stun_timeout)*time.Second)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	log.Println("   main2 end")
}

func main() {
	help()

	gogo.GuardStart(main2, func(err error) {
		// if 0: err==nil; -1: err==255; -2: err==254; err==1: 1; err==2
		if err != nil {
			log.Printf("   发现导致重启的错误: %v", err)
		}
	})
}
