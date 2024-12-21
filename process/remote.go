package process

import (
	"goodlink/proxy"
	"goodlink/stun2"
	"goodlink/tools"
	"goodlink/tun"
	_ "goodlink/tun"
	"log"
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

		if redisJson.State != 3 && redisJson.State != 4 && redisJson.State-last_state > 1 {
			log.Println("   状态异常")
			m_redis_db.Del(m_md5_tun_key)
			return nil, nil
		}

		redisJson.RedisTimeOut = time_out * 3

		switch redisJson.State {
		case 0:
			log.Printf("%d: 收到对端请求: %v\n", redisJson.State, redisJson)

			conn := tools.GetListenUDP()
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(conn)

			switch redisJson.ClientPort {
			case 0:
				conn_type = 0
				log.Print("   对端未发来IP")

				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				m_tun_active = tun.CreateTunActive(conn, time_out)
				tun_active_chain = m_tun_active.ProcessChain

				redisJson.State = 1
				redisJson.SendPortCount = 0x100
				log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				log.Print("   对端有发来IP")
				conn_type = 1

				if m_tun_passive != nil {
					m_tun_passive.Release()
				}
				m_tun_active = nil

				m_tun_passive = tun.CteateTunPassive(conn, redisJson.ClientIP, redisJson.ClientPort, 0x100)
				m_tun_passive.Start()

				tun_passive_chain = m_tun_passive.ProcessChain

				redisJson.State = 1
				log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 2:
			switch conn_type {
			case 0:
				log.Printf("%d: 收到对端地址: %v\n", redisJson.State, redisJson)
				m_tun_active.Start(redisJson.ClientIP, redisJson.ClientPort)

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
			tun.ProcessHealth(health)
			log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		}(remote_addr, conn)
	}
}
