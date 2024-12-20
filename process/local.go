package process

import (
	"fmt"
	"goodlink/proxy"
	"goodlink/stun2"
	"goodlink/tools"
	"goodlink/tunnel"
	_ "goodlink/tunnel"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

func GetLocalQuicConn(conn_type int, count int) (quic.Connection, quic.Stream) {
	var err error

	redisJson := RedisJsonType{
		ConnectCount: count,
	}

	wan_ip_chain := make(chan string, 1)
	wan_port_chain := make(chan int, 1)

	go func() {
		ClientIP, ClientPort := stun2.GetWanIpPort()
		wan_ip_chain <- ClientIP
		wan_port_chain <- ClientPort
	}()

	switch conn_type {
	case 0:
		log.Println("0: 请求连接对端")
		RedisSet(30*time.Second, &redisJson)

	default:
		redisJson.ClientIP, redisJson.ClientPort = <-wan_ip_chain, <-wan_port_chain
		redisJson.State = 0
		log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
		RedisSet(30*time.Second, &redisJson)
	}

	for {
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

				m_tun_passive = &tunnel.TunPassive{
					TunQuicConn:     nil,
					TunHealthStream: nil,
					TunState:        1,
					ConnList:        make([]*net.UDPConn, 0),
				}

				redisJson.ClientIP, redisJson.ClientPort = <-wan_ip_chain, <-wan_port_chain
				m_tun_passive.Process(redisJson.ServerIP, redisJson.ServerPort, redisJson.SendPortCount)
				m_tun_passive.Send()
				go func(d *tunnel.TunPassive) {
					for {
						time.Sleep(3 * time.Second)
						if d.Send() < 0 {
							return
						}
					}
				}(m_tun_passive)

				redisJson.State = 2
				log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)
				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				m_tun_active = &tunnel.TunActive{
					RedisTimeOut:    redisJson.RedisTimeOut,
					TunQuicConn:     nil,
					TunHealthStream: nil,
					Conn:            nil,
					ConnList:        make([]*net.UDPConn, 0),
					ProcessChain:    make(chan quic.Connection, 1),
				}
				m_tun_active.Conn = tools.GetListenUDP()
				m_tun_active.ProcessServerChild(redisJson.ServerIP, redisJson.ServerPort)
				redisJson.State = 2
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 3:
			if m_tun_passive.TunQuicConn == nil {
				log.Println("   连接失败")
				return nil, nil
			}
			log.Printf("%d: 连接成功\n", redisJson.State)
			return m_tun_passive.TunQuicConn, m_tun_passive.TunHealthStream

		case 4:
			log.Printf("%d: 连接超时\n", redisJson.State)
			return nil, nil

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}
	}
}

func RunLocal(conn_type int, tun_local_addr string, tun_key string, retry bool) error {
	listener, err := net.Listen("tcp", tun_local_addr)
	if listener == nil || err != nil {
		return fmt.Errorf("地址监听失败: %v", tun_local_addr)
	}
	defer listener.Close()

	count := 0

	for {

		count++

		conn, health := GetLocalQuicConn(conn_type, count)
		if conn == nil {
			Release()
			continue
		}

		chain := make(chan int, 1)
		go func() {
			proxy.ProcessProxyClient(listener, conn)
			chain <- 1
		}()

		tunnel.ProcessHealth(health)
		log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		Release()

		if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
			conn.Write([]byte("hello"))
			conn.Close() // 关闭连接
		}

		<-chain
		count = 0

		if !retry {
			return fmt.Errorf("   连接已断开")
		}
	}
}
