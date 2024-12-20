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

	for {
		time.Sleep(1 * time.Second)

		for RedisGet(&redisJson) != nil {
			time.Sleep(3 * time.Second)
		}

		if redisJson.State < last_state {
			log.Println("   对端已重置连接")
			return nil, nil
		}

		if redisJson.State-last_state > 1 {
			log.Println("   状态异常")
			M_redis_db.Del(M_md5_tun_key)
			return nil, nil
		}

		switch redisJson.State {
		case 0:
			m_tun_active = &tunnel.TunActive{
				RedisTimeOut:    time_out * 3,
				SocketTimeOut:   time_out,
				TunQuicConn:     nil,
				TunHealthStream: nil,
				Conn:            nil,
				ConnList:        make([]*net.UDPConn, 0),
				ProcessChain:    make(chan quic.Connection, 1),
			}

			log.Printf("%d: 收到对端请求\n", redisJson.State)
			m_tun_active.Conn = tools.GetListenUDP()
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(m_tun_active.Conn)

			redisJson.RedisTimeOut = m_tun_active.RedisTimeOut
			redisJson.State = 1
			redisJson.SendPortCount = 0x100 //0x400
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(redisJson.RedisTimeOut, &redisJson)

		case 2:
			log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)

			m_tun_active.ProcessServerChild(redisJson.ClientIP, redisJson.ClientPort)

			select {
			case <-m_tun_active.ProcessChain:
				redisJson.State = 3
				log.Printf("%d: 通知对端, 连接成功\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return m_tun_active.TunQuicConn, m_tun_active.TunHealthStream

			case <-time.After(m_tun_active.SocketTimeOut):
				redisJson.State = 4
				log.Printf("%d: 通知对端, 连接超时\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return nil, nil
			}

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
			tunnel.ProcessHealth(health)
			log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		}(remote_addr, conn)
	}
}
