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
)

type RedisInfo struct {
	Addr   string `bson:"addr" json:"addr"`
	Passwd string `bson:"passwd" json:"passwd"`
	Id     int    `bson:"id" json:"id"`
}

type ConfigInfo struct {
	Redis RedisInfo `bson:"redis" json:"redis"`
}

func help() {
	v := flag.Bool("v", false, "查看版本信息")

	/* 没有用到的参数 */
	var temp_value int64
	flag.Int64Var(&temp_value, "gogo-restart-delay", 1000, "自动重启的延迟时间, 单位: 毫秒")
	flag.Bool("gogo-background", false, "后台执行")

	var configInfo ConfigInfo
	gogo.UtilsHttpClient().Get("https://gitee.com/konyshe/goodlink_conf/raw/master/config.json").SetSuccessResult(&configInfo).Do()

	flag.StringVar(&m_cli_pprof_addr, "pprof_addr", "", "性能检测服务监听的地址端口, 例如: 0.0.0.0:6060")
	flag.StringVar(&m_cli_redis_addr, "redis_addr", configInfo.Redis.Addr, "Redis服务地址端口")
	flag.StringVar(&m_cli_redis_pass, "redis_pass", configInfo.Redis.Passwd, "Redis服务密码")
	flag.IntVar(&m_cli_redis_id, "redis_id", configInfo.Redis.Id, "Redis服务可使用的表ID")
	flag.StringVar(&m_cli_tun_local_addr, "local", "", "客户端监听的地址端口")
	flag.StringVar(&m_cli_tun_remote_addr, "remote", "", "服务端所处网络中, 需要被远程访问的主机地址端口, 例如: 192.168.3.2:9999")
	flag.StringVar(&m_cli_tun_key, "key", "", "自定义, 客户端和服务端必须一致。16-24个字节长度: {name}_{YYYYMMDDHHMM}, 例如: kony_202412140928")

	flag.Parse()

	if *v {
		fmt.Print(gogo.BuildVersion())
		os.Exit(0)
	}
}
