package main

import (
	"gogo"
	"goodlink/config"
	"goodlink/stun2"
	"goodlink/tunnel"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
)

func main2() {
	if m_cli_pprof_addr != "" {
		go log.Println(http.ListenAndServe(m_cli_pprof_addr, nil))
	}

	if m_cli_stun_svr_addr != "" {
		stun2.StartSvr(m_cli_stun_svr_addr, m_cli_stun_svr_port)
		os.Exit(0)
	}

	if m_cli_redis_addr == "" {
		config.Init()
		m_cli_redis_addr = config.GetAddr()
		m_cli_redis_pass = config.GetPasswd()
		m_cli_redis_id = config.GetID()
	}

	if m_cli_tun_local_addr != "" {
		go func() {
			if err := tunnel.ProcessClient(m_cli_tun_local_addr,
				m_cli_redis_addr,
				m_cli_redis_pass,
				m_cli_redis_id,
				m_cli_tun_key,
				true); err != nil {

				log.Println(err)
				os.Exit(0)
			}
		}()
	} else {
		go tunnel.ProcessServer(m_cli_tun_remote_addr,
			m_cli_redis_addr,
			m_cli_redis_pass,
			m_cli_redis_id,
			m_cli_tun_key)
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
