//go:build cmd

package main

import (
	"gogo"
	"goodlink/pro"
	_ "goodlink/pro"
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
	if *m_cli_stun_test { // 测试stun节点，开发使用选项
		stun2.TestStun()
		os.Exit(0)
	}

	go func() {
		if m_cli_pprof_addr != "" { // 性能监测，开发使用选项
			log.Println(http.ListenAndServe(m_cli_pprof_addr, nil))
		}
	}()

	if m_cli_stun_svr_addr != "" { // 自建stun服务，开发使用选项
		stun2.StartSvr(m_cli_stun_svr_addr, m_cli_stun_svr_port)
		os.Exit(0)
	}

	// 第三方集成, 关注以下代码即可
	go func() {
		pro.Init(m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id)

		switch len(m_cli_tun_local_addr) {
		case 0:
			pro.RunRemote(m_cli_tun_remote_addr,
				m_cli_tun_key,
				time.Duration(m_cli_stun_timeout)*time.Second)

		default:
			if err := pro.RunLocal(m_cli_conn_type,
				m_cli_tun_local_addr,
				m_cli_tun_key); err != nil {

				log.Println(err)
				os.Exit(0)
			}
		}
	}()

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
