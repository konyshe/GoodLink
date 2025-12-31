//go:build cmd

package main

import (
	"flag"
	go2log "go2/log"
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
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
)

func main2() {
	log.Println("官方网址: https://gitee.com/konyshe/goodlink")

	go func() {
		if config.Arg_pprof_addr != "" { // 性能监测，开发使用选项
			log.Println(http.ListenAndServe(config.Arg_pprof_addr, nil))
		}
	}()

	// 第三方集成, 关注以下代码即可
	go func() {
		if err := pro.Init(); err != nil {
			log.Println(err)
			return
		}

		if *config.Arg_tun_local {
			if err := pro.RunLocal(config.Arg_tun_key); err != nil {
				log.Println(err)
				os.Exit(0)
			}
		} else if *config.Arg_tun_remote {
			pro.RunRemote(config.Arg_tun_key)
		} else {
			log.Println("参数错误")
			os.Exit(0)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	pro.StopLocal()
	pro.StopRemote()
}

func main() {
	// 新增系统级调优
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetGCPercent(10) // 降低GC频率
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
			log.Println(string(debug.Stack()))
		}
	}()

	pro.SetVersion(GetVersion())

	config.Help(GetVersion())

	if *config.Arg_stun_test { // 测试stun节点，开发使用选项
		stun2.TestStun()
		os.Exit(0)
	}

	if config.Arg_tun_key == "" {
		flag.Usage()
		os.Exit(0)
	}

	utils.GuardStart(main2, 500*time.Millisecond, func(err error) {
		// if 0: err==nil; -1: err==255; -2: err==254; err==1: 1; err==2
		if err != nil {
			log.Printf("   异常退出: %v", err)
			go2log.Dingf("error: %v", err)
		}
	})
}
