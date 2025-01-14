package config

import (
	"flag"
	"fmt"
	"gogo"
	"os"
)

var (
	Arg_pprof_addr      string
	Arg_tun_local_addr  string
	Arg_tun_remote_addr string
	Arg_redis_addr      string
	Arg_redis_pass      string
	Arg_redis_id        int
	Arg_tun_key         string
	Arg_stun_test       *bool
	Arg_p2p_timeout     int
	Arg_conn_type       int
	Arg_conn_n0         int
	Arg_conn_n1         int
	Arg_stun_svr_addr   string
)

func Help() {
	v := flag.Bool("v", false, "查看版本信息")

	Arg_stun_test = flag.Bool("stun_test", false, "检测STUN列表是否可用")
	flag.StringVar(&Arg_pprof_addr, "pprof_addr", "", "如果CPU/内存/网络异常, 可监测运行, 例如: 0.0.0.0:6060")

	flag.StringVar(&Arg_redis_addr, "redis_addr", "", "Redis服务地址端口, 例如: 1.2.3.4:6379")
	flag.StringVar(&Arg_redis_pass, "redis_pass", "", "Redis服务密码, 例如: 123456")
	flag.IntVar(&Arg_redis_id, "redis_id", 15, "Redis服务可使用的表ID")

	flag.StringVar(&Arg_stun_svr_addr, "stun", "", "自定义的STUN服务器地址, 例如: stun.easyvoip.com:3478")

	flag.StringVar(&Arg_tun_local_addr, "local", "", "客户端监听的地址端口, 例如: 0.0.0.0:9022")
	flag.StringVar(&Arg_tun_remote_addr, "remote", "", "服务端所处网络中, 需要被远程访问的主机地址端口, 例如: 127.0.0.1:22")
	flag.StringVar(&Arg_tun_key, "key", "", "自定义, 必须客户端和服务端一致。建议: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928")
	flag.IntVar(&Arg_p2p_timeout, "time_out", 30, "最大连接超时, 单位: 秒")
	flag.IntVar(&Arg_conn_type, "conn", 0, "若超过10分钟无法连接, 可尝试更换连接方式: 0: 主动; 1: 被动")

	flag.IntVar(&Arg_conn_n0, "n0", 256, "dev n0")
	flag.IntVar(&Arg_conn_n1, "n1", 4, "dev n1")

	/* 没有用到的参数 */
	flag.Bool("fork", false, "子进程")

	flag.Parse()

	if *v {
		fmt.Print(gogo.BuildVersion())
		os.Exit(0)
	}
}
