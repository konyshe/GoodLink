package main

import (
	"gogo"
	"goodlink/tunnel"
	"log"
	"os"
	"os/signal"
	"syscall"
	//_ "net/http/pprof"
)

func main2() {
	/*go func() {
		log.Println(http.ListenAndServe("localhost:8080", nil))
	}()*/

	if m_cli_tun_remote != "" {
		go tunnel.ProcessServer(m_cli_tun_remote, m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key)

	} else if m_cli_tun_local != "" {
		go tunnel.ProcessClient(m_cli_tun_local, m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key)
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
