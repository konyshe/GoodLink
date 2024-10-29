package main

import (
	"flag"
	"fmt"
	"gogo"
	"os"
)

var (
	m_cli_admin_remote_addr string
	m_cli_admin_local_addr  string

	m_cli_tun_local_addr  string
	m_cli_tun_remote_addr string
	m_cli_redis_addr      string
	m_cli_redis_pass      string
	m_cli_redis_id        int
	m_cli_tun_key         string

	mp_cli_pprof *bool
)

func help() {
	v := flag.Bool("v", false, "show version info")

	/* 没有用到的参数 */
	var temp_value int64
	flag.Int64Var(&temp_value, "gogo-restart-delay", 100, "gogo-restart-delay")
	flag.Bool("gogo-background", false, "gogo-background")

	mp_cli_pprof = flag.Bool("pprof", false, "If present, exposes pprof information on 0.0.0.0:6060")

	flag.StringVar(&m_cli_admin_remote_addr, "admin_remote_addr", "", "隧道对端地址,内部子进程使用,用户忽略")
	flag.StringVar(&m_cli_admin_local_addr, "admin_local_addr", "", "隧道本地地址,内部子进程使用,用户忽略")
	flag.StringVar(&m_cli_redis_addr, "redis_addr", "", "redis访问地址, 例如: 1.1.2.2:6379")
	flag.StringVar(&m_cli_redis_pass, "redis_pass", "", "redis访问密码, 例如: 12345678")
	flag.IntVar(&m_cli_redis_id, "redis_id", 0, "redis可用的表ID")
	flag.StringVar(&m_cli_tun_local_addr, "local", "", "客户端提供穿透服务的监听地址, 例如: 127.0.0.1:9022")
	flag.StringVar(&m_cli_tun_remote_addr, "remote", "", "服务端连接目标服务的地址, 例如: 192.168.3.2:22")
	flag.StringVar(&m_cli_tun_key, "key", "", "隧道Key, 请保证客户端和服务端一致")

	flag.Parse()

	if *v {
		fmt.Print(gogo.BuildVersion())
		os.Exit(0)
	}
}
