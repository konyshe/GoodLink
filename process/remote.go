package process

import (
	"goodlink/proxy"
	"goodlink/stun2"
	"goodlink/tools"
	"goodlink/tunnel"
	_ "goodlink/tunnel"
	"log"
	"net"
	"os"
	"time"

	"github.com/quic-go/quic-go"
)

func GetRemoteQuicConn(time_out time.Duration) (quic.Connection, quic.Stream) {
	redisJson := RedisJsonType{}
	last_state := redisJson.State
	conn_type := 0 //主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	for {
		time.Sleep(1 * time.Second)

		for RedisGet(&redisJson) != nil {
			time.Sleep(3 * time.Second)
		}

		if redisJson.State < last_state {
			log.Println("   对端已重置连接")
			return nil, nil
		}

		switch redisJson.State {
		case 0:
			log.Printf("%d: 收到对端请求\n", redisJson.State)
			switch redisJson.ClientPort {
			case 0:
				log.Print("   对端未发来IP, 开始主动连接")
				conn_type = 0

				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				m_tun_active = &tunnel.TunActive{
					RedisTimeOut:    time_out * 3,
					TunQuicConn:     nil,
					TunHealthStream: nil,
					Conn:            nil,
					ConnList:        make([]*net.UDPConn, 0),
					ProcessChain:    make(chan quic.Connection, 1),
				}
				tun_active_chain = m_tun_active.ProcessChain

				m_tun_active.Conn = tools.GetListenUDP()
				redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(m_tun_active.Conn)

				log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
				redisJson.State = 1
				redisJson.SendPortCount = 0x100
				redisJson.RedisTimeOut = m_tun_active.RedisTimeOut
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				log.Print("   对端有发来IP, 开始被动连接")
				conn_type = 1

				if m_tun_passive != nil {
					m_tun_passive.Release()
				}
				m_tun_active = nil

				m_tun_passive = &tunnel.TunPassive{
					TunQuicConn:     nil,
					TunHealthStream: nil,
					TunState:        1,
					ConnList:        make([]*net.UDPConn, 0),
					ProcessChain:    make(chan quic.Connection, 1),
				}
				tun_passive_chain = m_tun_passive.ProcessChain

				redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort()
				m_tun_passive.Process(redisJson.ClientIP, redisJson.ClientPort, 0x100)
				m_tun_passive.Send()
				go func(d *tunnel.TunPassive) {
					for {
						time.Sleep(3 * time.Second)
						if d.Send() < 0 {
							return
						}
					}
				}(m_tun_passive)

				log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
				redisJson.State = 1
				redisJson.RedisTimeOut = time_out * 3
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 2:
			switch conn_type {
			case 0:
				log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)
				m_tun_active.ProcessServerChild(redisJson.ClientIP, redisJson.ClientPort)

			case 1:
				log.Printf("%d: 收到对端地址, 等待连接: %v\n", redisJson.State, redisJson)
			}

			select {
			case <-tun_active_chain:
				redisJson.State = 3
				log.Printf("%d: 通知对端, 连接成功\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return m_tun_active.TunQuicConn, m_tun_active.TunHealthStream

			case <-tun_passive_chain:
				redisJson.State = 3
				log.Printf("%d: 通知对端, 连接成功\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return m_tun_passive.TunQuicConn, m_tun_passive.TunHealthStream

			case <-time.After(time_out):
				redisJson.State = 4
				log.Printf("%d: 通知对端, 连接超时\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return nil, nil
			}

		case 3, 4:

		default:
			if redisJson.State-last_state > 1 {
				log.Println("   状态异常")
				m_redis_db.Del(m_md5_tun_key)
				return nil, nil
			}
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}

		last_state = redisJson.State
	}
}

func RunRemote(remote_addr string, tun_key string, time_out time.Duration) error {
	if remote_addr == "" {
		remote_addr = tools.GetFreeLocalAddr()
		if remote_addr == "" {
			log.Panic("   获取本地端口失败")
			os.Exit(0)
		}
		go proxy.ListenSocks5(remote_addr)
	}

	for {
		conn, health := GetRemoteQuicConn(time_out)
		if conn == nil {
			Release()
			continue
		}

		go func(remote string, conn quic.Connection) {
			defer Release()
			go proxy.ProcessProxyServer(remote, conn)
			tunnel.ProcessHealth(health)
			log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		}(remote_addr, conn)
	}
}
