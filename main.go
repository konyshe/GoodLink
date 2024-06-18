package main

import (
	"fmt"
	"gogo"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	//_ "net/http/pprof"

	"github.com/quic-go/quic-go"
)

var (
	m_stun_quic_conn   quic.Connection
	m_recv_data        []byte
	m_send_data        []byte
	m_process_stop     = false
	m_process_lock     sync.Mutex
	m_process_time_out = 15 * time.Second
)

func main2() {
	/*go func() {
		log.Println(http.ListenAndServe("localhost:8080", nil))
	}()*/

	m_stun_quic_conn = nil
	m_recv_data = make([]byte, 1600)
	m_send_data = []byte(randomString(9))

	if m_cli_tun_remote != "" {
		if m_cli_admin_remote_addr != "" && m_cli_admin_local_addr != "" {
			if process_server_child() != nil {
				go process_proxy_remote(m_cli_tun_remote)
			}
		} else {
			process_server_parent()
		}
	} else if m_cli_tun_local != "" {
		if process_client() != nil {
			go process_proxy_local(m_cli_tun_local)
		}
	}

	time.Sleep(m_process_time_out)
	if m_stun_quic_conn == nil {
		fmt.Printf("main exit: %v\n", os.Args)
		os.Exit(0)
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
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
