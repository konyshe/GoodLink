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

	"gogo"

	"github.com/quic-go/quic-go"
)

var (
	m_local_state = 0 //0: 停止, 1: 启动, 2: 连接成功
)

func GetLocalQuicConn(conn_type int, count int) (quic.Connection, quic.Stream, error) {
	redisJson := RedisJsonType{
		ConnectCount: count,
	}

	conn := tools.GetListenUDP()

	ClientIP, ClientPort1, ClientPort2 := stun2.GetWanIpPort2(conn)
	if ClientPort1 == ClientPort2 {
		conn_type = 0
	}

	switch conn_type {
	case 0:
		gogo.Log().Debug("0: 请求连接对端")
		RedisSet(15*time.Second, &redisJson)

	default:
		redisJson.ClientIP, redisJson.ClientPort1, redisJson.ClientPort2 = ClientIP, ClientPort1, ClientPort2
		redisJson.State = 0
		gogo.Log().DebugF("%d: 发送本端地址: %v", redisJson.State, redisJson)
		RedisSet(15*time.Second, &redisJson)
	}

	for m_local_state == 1 {
		time.Sleep(1 * time.Second)

		if RedisGet(&redisJson) != nil {
			continue
		}

		switch redisJson.State {
		case 1:
			gogo.Log().DebugF("%d: 收到对端地址: %v", redisJson.State, redisJson)

			switch conn_type {
			case 0:
				if m_tun_passive != nil {
					m_tun_passive.Release()
				}
				m_tun_active = nil

				redisJson.ClientIP, redisJson.ClientPort1, redisJson.ClientPort2 = ClientIP, ClientPort1, ClientPort2
				if redisJson.ClientIP == redisJson.ServerIP {
					RedisDel()
					return nil, nil, fmt.Errorf("已经和对端处在同一个公网下")
				}

				m_tun_passive = tun.CteateTunPassive(conn, redisJson.ServerIP, redisJson.ServerPort1, redisJson.ServerPort2, redisJson.SendPortCount)
				m_tun_passive.Start()

				redisJson.State = 2
				gogo.Log().DebugF("%d: 发送本端地址: %v", redisJson.State, redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				if redisJson.ClientIP == redisJson.ServerIP {
					RedisDel()
					return nil, nil, fmt.Errorf("已经和对端处在同一个公网下")
				}

				m_tun_active = tun.CreateTunActive(conn, 15*time.Second)
				m_tun_active.Start(redisJson.ClientPort1, redisJson.ClientPort2, redisJson.ServerIP, redisJson.ServerPort1, redisJson.ServerPort2, redisJson.SocketTimeOut)
				redisJson.State = 2
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 3:
			if m_tun_passive != nil {
				if m_tun_passive.TunQuicConn != nil {
					gogo.Log().DebugF("%d: 连接成功", redisJson.State)
					return m_tun_passive.TunQuicConn, m_tun_passive.TunHealthStream, nil
				}
			}
			if m_tun_active != nil {
				if m_tun_active.TunQuicConn != nil {
					gogo.Log().DebugF("%d: 连接成功", redisJson.State)
					return m_tun_active.TunQuicConn, m_tun_active.TunHealthStream, nil
				}
			}
			gogo.Log().Debug("   连接失败")
			return nil, nil, nil

		case 4:
			gogo.Log().DebugF("%d: 连接超时", redisJson.State)
			return nil, nil, nil

		default:
			gogo.Log().DebugF("%d: 等待对端状态", redisJson.State)
		}
	}

	return nil, nil, nil
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

	log.Printf("   绑定端口: %v", tun_local_addr)

	chain := make(chan int, 1)

	listener, err := net.Listen("tcp", tun_local_addr)
	if listener == nil || err != nil {
		return fmt.Errorf("绑定端口失败: %v", tun_local_addr)
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

		conn, health, err := GetLocalQuicConn(conn_type, count)
		if err != nil {
			Release()
			return err
		}
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
		gogo.Log().DebugF("   释放连接: %v", conn.LocalAddr())
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
