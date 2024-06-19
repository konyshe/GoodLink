package main

import (
	"gogo"
	"goodlink/proxy"
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

	if m_cli_tun_remote != "" && m_cli_admin_remote_addr == "" && m_cli_admin_local_addr == "" {
		var tunnelServer tunnel.TunnelServer
		go tunnelServer.ProcessServerParent(m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key)

	} else {
		go func() {
			if m_cli_admin_remote_addr != "" && m_cli_admin_local_addr != "" && m_cli_tun_remote != "" {
				var tunnelServer tunnel.TunnelServer
				proxy.ProcessProxyServer(m_cli_tun_remote, tunnelServer.ProcessServerChild(m_cli_admin_local_addr, m_cli_admin_remote_addr))
				os.Exit(0)
			} else if m_cli_tun_local != "" {
				var tunnelClient tunnel.TunnelClient
				proxy.ProcessProxyClient(m_cli_tun_local, tunnelClient.ProcessClient(m_cli_redis_addr, m_cli_redis_pass, m_cli_redis_id, m_cli_tun_key))
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
