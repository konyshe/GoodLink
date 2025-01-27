package pro2

import (
	"fmt"
	"goodlink/pro"
	"goodlink/utils"
	"goodlink2/tun2"
	_ "goodlink2/tun2"
	"net"
	"os"

	"github.com/go-redis/redis"
)

var (
	m_redis_db    *redis.Client
	m_tun_key     string
	m_md5_tun_key string
)

func Release(tun_active *tun2.TunActive, tun_passive *tun2.TunPassive) {
	utils.Log().SetDebugSate(0)

	if tun_active != nil {
		tun_active.Release()
	}
	if tun_passive != nil {
		tun_passive.Release()
	}
}

func GetTCPAddr() (*net.Listener, *net.TCPAddr, pro.AddrType) {
	addr := pro.AddrType{}
	addr.IPv6, _ = utils.GetUDPLocalIPPort("udp6")
	addr.LocalIPv4, addr.LocalPort = utils.GetUDPLocalIPPort("udp4")

	// 设置 SO_REUSEADDR 和 SO_REUSEPORT 选项
	//tcp_addr := &net.TCPAddr{IP: net.ParseIP("0.0.0.0"), Port: 12345}
	l, err := net.Listen("tcp4", "0.0.0.0")
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		os.Exit(1)
	}
	//defer l.Close() // 注意这里我们延迟关闭监听者，但不关闭端口

	tcp_addr := l.Addr().(*net.TCPAddr)

	// 使用 Dialer 来建立连接
	dialer := &net.Dialer{
		LocalAddr: tcp_addr, // 使用相同的本地地址和端口
	}
	/*
		conn, err := dialer.Dial("tcp", "ifconfig.co:80")
		if err != nil {
			fmt.Println("Error dialing:", err.Error())
			os.Exit(1)
		}
		defer conn.Close()
		log.Printf("本地监听: %v:%v\n", conn.LocalAddr().(*net.TCPAddr).IP, conn.LocalAddr().(*net.TCPAddr).Port)
	*/
	addr.WanIPv4, addr.WanPort1 = utils.GetTCPWanIPv4Port(dialer)
	addr.WanPort2 = 0

	return &l, tcp_addr, addr
}
