package main

import (
	"flag"
	"fmt"
	"gogo"
	"os"
)

var (
	m_cli_pprof_addr      string
	m_cli_tun_local_addr  string
	m_cli_tun_remote_addr string
	m_cli_redis_addr      string
	m_cli_redis_pass      string
	m_cli_redis_id        int
	m_cli_tun_key         string
	m_cli_stun_svr_addr   string
	m_cli_stun_svr_port   int
	m_cli_stun_test       *bool
	m_cli_stun_timeout    int
)

func help() {
	v := flag.Bool("v", false, "查看版本信息")

	/* 没有用到的参数 */
	var temp_value int64
	flag.Int64Var(&temp_value, "gogo-restart-delay", 1000, "自动重启的延迟时间, 单位: 毫秒")
	flag.Bool("gogo-background", false, "后台执行")

	flag.StringVar(&m_cli_stun_svr_addr, "stun_svr", "", "stun svr listen addr")
	flag.IntVar(&m_cli_stun_svr_port, "stun_port", 3478, "stun svr listen port")
	m_cli_stun_test = flag.Bool("stun_test", false, "后台执行")

	flag.StringVar(&m_cli_pprof_addr, "pprof_addr", "", "性能检测服务监听的地址端口, 例如: 0.0.0.0:6060")
	flag.StringVar(&m_cli_redis_addr, "redis_addr", "", "Redis服务地址端口, 例如: 1.2.3.4:6379")
	flag.StringVar(&m_cli_redis_pass, "redis_pass", "", "Redis服务密码, 例如: 123456")
	flag.IntVar(&m_cli_redis_id, "redis_id", 15, "Redis服务可使用的表ID")

	flag.StringVar(&m_cli_tun_local_addr, "local", "", "客户端监听的地址端口, 例如: 0.0.0.0:9022")
	flag.StringVar(&m_cli_tun_remote_addr, "remote", "", "服务端所处网络中, 需要被远程访问的主机地址端口, 例如: 127.0.0.1:22")
	flag.StringVar(&m_cli_tun_key, "key", "", "自定义, 必须客户端和服务端一致。建议: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928")
	flag.IntVar(&m_cli_stun_timeout, "time_out", 15, "最大连接超时, 单位: 秒")

	flag.Parse()

	if *v {
		fmt.Print(gogo.BuildVersion())
		os.Exit(0)
	}
}
