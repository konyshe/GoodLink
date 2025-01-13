//go:build cmd

package main

import (
	"goodlink/config"
	"goodlink/pro"
	_ "goodlink/pro"
	"goodlink/stun2"
	"goodlink/utils"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main2() {
	if *config.Arg_stun_test { // 测试stun节点，开发使用选项
		stun2.TestStun()
		os.Exit(0)
	}

	go func() {
		if config.Arg_pprof_addr != "" { // 性能监测，开发使用选项
			log.Println(http.ListenAndServe(config.Arg_pprof_addr, nil))
		}
	}()

	// 第三方集成, 关注以下代码即可
	go func() {
		if err := pro.Init(config.Arg_redis_addr, config.Arg_redis_pass, config.Arg_redis_id); err != nil {
			log.Println(err)
			return
		}

		switch len(config.Arg_tun_local_addr) {
		case 0:
			pro.RunRemote(config.Arg_tun_remote_addr,
				config.Arg_tun_key,
				time.Duration(config.Arg_p2p_timeout)*time.Second)

		default:
			if err := pro.RunLocal(config.Arg_conn_type,
				config.Arg_tun_local_addr,
				config.Arg_tun_key); err != nil {

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
	config.Help()

	utils.GuardStart(main2, 500*time.Millisecond, func(err error) {
		// if 0: err==nil; -1: err==255; -2: err==254; err==1: 1; err==2
		if err != nil {
			log.Printf("   异常退出: %v", err)
			utils.DingF("error: %v", err)
		}
	})
}
