package config

import (
	"flag"
	"fmt"
	"go2"
	"os"
)

// TunKeyByteLen 连接密钥固定字节长度（Local/Remote 必须一致）
const (
	TunKeyByteLen = 48
)

var (
	Arg_pprof_addr     string
	Arg_tun_local      *bool
	Arg_tun_remote     *bool
	Arg_redis_addr     string
	Arg_redis_tls_addr string
	Arg_redis_pass     string
	Arg_redis_id       int
	Arg_tun_key        string
	Arg_stun_test      *bool
	Arg_conn_type      int
	Arg_ui             *bool
	Arg_stun_svr_ip    string
	Arg_stun_svr_port  int
	Arg_transport      string
)

func Help() {
	v := flag.Bool("v", false, "查看版本信息")

	Arg_ui = flag.Bool("ui", false, "由UI启动, 从 goodlink.json 读取并监听转发配置")

	flag.StringVar(&Arg_pprof_addr, "pprof_addr", "", "如果CPU/内存/网络异常, 可监测运行, 例如: 0.0.0.0:6060")

	flag.StringVar(&Arg_stun_svr_ip, "stun_svr_ip", "", "STUN服务IP地址")
	flag.IntVar(&Arg_stun_svr_port, "stun_svr_port", 0, "STUN服务端口")
	flag.StringVar(&Arg_redis_addr, "redis_addr", "", "Redis服务地址, 例如: 1.2.3.4:6379")
	flag.StringVar(&Arg_redis_tls_addr, "redis_tls_addr", "", "Redis服务TLS地址, 例如: 1.2.3.4:16378")
	flag.StringVar(&Arg_redis_pass, "redis_pass", "", "Redis服务密码, 例如: 123456")
	flag.IntVar(&Arg_redis_id, "redis_id", 15, "Redis服务可使用的表ID")
	Arg_stun_test = flag.Bool("stun_test", false, "检测STUN列表是否可用")

	Arg_tun_local = flag.Bool("local", false, "启动Local端")
	Arg_tun_remote = flag.Bool("remote", false, "启动Remote端")
	flag.StringVar(&Arg_tun_key, "key", "", "自定义, 必须客户端和服务端一致。建议: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928")
	flag.StringVar(&Arg_transport, "transport", TransportQUIC, "传输协议: quic 或 kcp")

	/* 没有用到的参数 */
	flag.Bool("fork", false, "子进程")

	flag.Parse()

	Arg_transport = NormalizeTransport(Arg_transport)
	if Arg_transport != TransportQUIC && Arg_transport != TransportKCP {
		fmt.Printf("无效的传输协议: %s（仅支持 quic 或 kcp）\n", Arg_transport)
		os.Exit(1)
	}

	if *v {
		fmt.Printf("Version: %s\n", GetVersion())
		fmt.Print(go2.BuildVersion())
		os.Exit(0)
	}
}
