package main

import (
	"gogo"
	"goodlink/config"
	"goodlink/md5"
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

	"github.com/go-redis/redis"
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

	if m_cli_redis_addr == "" {
		config.Init()
		m_cli_redis_addr = config.GetAddr()
		m_cli_redis_pass = config.GetPasswd()
		m_cli_redis_id = config.GetID()
	}

	process.M_redis_db = redis.NewClient(&redis.Options{
		Addr:     m_cli_redis_addr,
		Password: m_cli_redis_pass,
		DB:       m_cli_redis_id,
	})
	if process.M_redis_db == nil {
		log.Println("Redis初始化失败")
		os.Exit(0)
	}
	defer process.M_redis_db.Close()

	process.M_tun_key = m_cli_tun_key
	process.M_md5_tun_key = md5.Encode(m_cli_tun_key)

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
