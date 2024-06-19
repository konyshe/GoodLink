package main

import (
	"gogo"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	//_ "net/http/pprof"
)

var (
	m_recv_data        []byte
	m_send_data        []byte
	m_process_time_out = 15 * time.Second
)

func main2() {
	/*go func() {
		log.Println(http.ListenAndServe("localhost:8080", nil))
	}()*/
	var tunnelClient TunnelClient
	var tunnelServer TunnelServer
	m_recv_data = make([]byte, 1600)
	m_send_data = []byte(randomString(9))

	if m_cli_tun_remote != "" && m_cli_admin_remote_addr == "" && m_cli_admin_local_addr == "" {
		go tunnelServer.process_server_parent()

	} else {
		go func() {
			if m_cli_admin_remote_addr != "" && m_cli_admin_local_addr != "" && m_cli_tun_remote != "" {
				process_proxy_server(m_cli_tun_remote, tunnelServer.process_server_child())
				go func() {
					time.Sleep(m_process_time_out)
					if tunnelServer.GetQuicConn() == nil {
						log.Printf("main2 exit: %v\n", os.Args)
						os.Exit(0)
					}
				}()
			} else if m_cli_tun_local != "" {
				process_proxy_client(m_cli_tun_local, tunnelClient.process_client())
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
