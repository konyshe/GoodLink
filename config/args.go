package config

import (
	"flag"
	"fmt"
	"gogo"
	"os"
)

var (
	Arg_pprof_addr             string
	Arg_tun_local              *bool
	Arg_tun_remote             *bool
	Arg_redis_addr             string
	Arg_redis_tls_addr         string
	Arg_redis_pass             string
	Arg_redis_id               int
	Arg_tun_key                string
	Arg_stun_test              *bool
	Arg_p2p_timeout            int
	Arg_conn_type              int
	Arg_conn_n0                int
	Arg_conn_n1                int
	Arg_conn_active_send_time  int
	Arg_conn_passive_send_time int
	M_version                  = "1.6"
)

func Help() {
	v := flag.Bool("v", false, "查看版本信息")

	Arg_stun_test = flag.Bool("stun_test", false, "检测STUN列表是否可用")
	flag.StringVar(&Arg_pprof_addr, "pprof_addr", "", "如果CPU/内存/网络异常, 可监测运行, 例如: 0.0.0.0:6060")

	flag.StringVar(&Arg_redis_addr, "redis_addr", "", "Redis服务地址, 例如: 1.2.3.4:6379")
	flag.StringVar(&Arg_redis_tls_addr, "redis_tls_addr", "", "Redis服务TLS地址, 例如: 1.2.3.4:16378")
	flag.StringVar(&Arg_redis_pass, "redis_pass", "", "Redis服务密码, 例如: 123456")
	flag.IntVar(&Arg_redis_id, "redis_id", 15, "Redis服务可使用的表ID")

	Arg_tun_local = flag.Bool("local", false, "启动Local端")
	Arg_tun_remote = flag.Bool("remote", false, "启动Remote端")

	flag.StringVar(&Arg_tun_key, "key", "", "自定义, 必须客户端和服务端一致。建议: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928")
	flag.IntVar(&Arg_p2p_timeout, "time_out", 15, "最大连接超时, 单位: 秒")

	flag.IntVar(&Arg_conn_n0, "n0", 256, "dev n0")
	flag.IntVar(&Arg_conn_n1, "n1", 4, "dev n1")
	flag.IntVar(&Arg_conn_active_send_time, "n2", 7, "dev n0")
	flag.IntVar(&Arg_conn_passive_send_time, "n3", 2, "dev n1")

	/* 没有用到的参数 */
	flag.Bool("fork", false, "子进程")

	flag.Parse()

	if *v {
		fmt.Print(gogo.BuildVersion())
		os.Exit(0)
	}
}
