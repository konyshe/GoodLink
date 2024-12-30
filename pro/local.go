package pro

import (
	"fmt"
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/stun2"
	"goodlink/tools"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_local_state = 0 //0: 停止, 1: 启动, 2: 连接成功
)

func GetLocalQuicConn(conn_type int, count int) (quic.Connection, quic.Stream) {
	var err error

	redisJson := RedisJsonType{
		ConnectCount: count,
	}

	wan_ip_chain := make(chan string, 1)
	wan_port_chain := make(chan int, 1)
	defer func() {
		close(wan_ip_chain)
		close(wan_port_chain)
	}()

	conn := tools.GetListenUDP()

	go func() {
		ClientIP, ClientPort := stun2.GetWanIpPort2(conn)
		wan_ip_chain <- ClientIP
		wan_port_chain <- ClientPort
	}()

	switch conn_type {
	case 0:
		log.Println("0: 请求连接对端")
		RedisSet(15*time.Second, &redisJson)

	default:
		redisJson.ClientIP, redisJson.ClientPort = <-wan_ip_chain, <-wan_port_chain
		redisJson.State = 0
		log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
		RedisSet(15*time.Second, &redisJson)
	}

	for m_local_state == 1 {
		time.Sleep(1 * time.Second)

		err = RedisGet(&redisJson)
		if err != nil {
			log.Println(err)
			return nil, nil
		}

		switch redisJson.State {
		case 1:
			switch conn_type {
			case 0:
				log.Printf("%d: 收到对端地址: %v\n", redisJson.State, redisJson)

				if m_tun_passive != nil {
					m_tun_passive.Release()
				}
				m_tun_active = nil

				redisJson.ClientIP, redisJson.ClientPort = <-wan_ip_chain, <-wan_port_chain

				m_tun_passive = tun.CteateTunPassive(conn, redisJson.ServerIP, redisJson.ServerPort, redisJson.SendPortCount)
				m_tun_passive.Start()

				redisJson.State = 2
				log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				log.Printf("%d: 收到对端地址: %v\n", redisJson.State, redisJson)
				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				m_tun_active = tun.CreateTunActive(conn, 15*time.Second)
				m_tun_active.Start(redisJson.ServerIP, redisJson.ServerPort, redisJson.SocketTimeOut)
				redisJson.State = 2
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 3:
			if m_tun_passive != nil {
				if m_tun_passive.TunQuicConn != nil {
					log.Printf("%d: 连接成功\n", redisJson.State)
					return m_tun_passive.TunQuicConn, m_tun_passive.TunHealthStream
				}
			}
			if m_tun_active != nil {
				if m_tun_active.TunQuicConn != nil {
					log.Printf("%d: 连接成功\n", redisJson.State)
					return m_tun_active.TunQuicConn, m_tun_active.TunHealthStream
				}
			}
			log.Println("   连接失败")
			return nil, nil

		case 4:
			log.Printf("%d: 连接超时\n", redisJson.State)
			return nil, nil

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}
	}

	return nil, nil
}

func GetLocalStats() int {
	return m_local_state
}

func StopLocal() error {
	m_local_state = 0
	Release()
	return nil
}

func RunLocal(conn_type int, tun_local_addr string, tun_key string) error {
	m_local_state = 1

	log.Printf("   本地监听地址: %v\n", tun_local_addr)

	chain := make(chan int, 1)

	listener, err := net.Listen("tcp", tun_local_addr)
	if listener == nil || err != nil {
		return fmt.Errorf("地址监听失败: %v", tun_local_addr)
	}
	defer func() {
		listener.Close()
		close(chain)
	}()

	m_tun_key = tun_key
	m_md5_tun_key = md5.Encode(tun_key)

	count := 0

	for m_local_state == 1 {

		count++

		conn, health := GetLocalQuicConn(conn_type, count)
		if conn == nil {
			Release()
			continue
		}

		go func() {
			proxy.ProcessProxyClient(listener, conn)
			chain <- 1
		}()

		m_local_state = 2
		tun.ProcessHealth(health)
		if m_local_state != 0 {
			m_local_state = 1
		}
		log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		Release()

		if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
			conn.Write([]byte("hello"))
			conn.Close() // 关闭连接
		}

		<-chain
		count = 0
	}

	return nil
}
