package pro

import (
	"errors"
	"go2"
	"goodlink/config"
	"goodlink/netstack"
	"goodlink2/tun"
	"log"
	"net"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_local_state      = 0 //0: 停止, 1: 启动, 2: 连接成功
	m_tun_active       *tun.TunActive
	m_tun_passive      *tun.TunPassive
	g_netstack_started = false
)

func GetLocalQuicConn(conn *net.UDPConn, addr *tun.AddrType, count int) (*tun.TunActive, *tun.TunPassive, quic.Connection, quic.Stream, error) {
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive

	SessionID := string(go2.RandomBytes(24))
	log.Printf("会话ID: %s", SessionID)

	redisJson := RedisJsonType{
		LocalVersion: GetVersion(),
		State:        0,
		SessionID:    SessionID,
		ConnectCount: count,
	}

	conn_type := 0 // 被动连接
	if addr.WanPort1 == addr.WanPort2 {
		log.Printf("WanPort %d:%d, 主动连接", addr.WanPort1, addr.WanPort2)
		conn_type = 1 // 主动连接
	} else {
		log.Printf("WanPort %d:%d, 被动连接", addr.WanPort1, addr.WanPort2)
	}

	switch conn_type {
	case 0:
		log.Println("请求连接Remote端")
		log.Println("[GOODLINK_STATUS]connecting")

	default:
		redisJson.LocalAddr = *addr
		log.Printf("发送Local端地址: %v", redisJson.LocalAddr)
		log.Println("[GOODLINK_STATUS]connecting")
	}

	// 阶段1: 将SessionID注册到Hash中，等待Remote端认领
	if err := RedisSessionRegister(30*time.Second, &redisJson); err != nil {
		log.Printf("注册会话失败: %v", err)
		return tun_active, tun_passive, nil, nil, err
	}
	log.Printf("已注册会话到队列，等待Remote端认领: %s", SessionID)

	// 等待Remote端认领并写入独立的session key
	sessionClaimed := false
	for i := 0; i < 30 && m_local_state == 1; i++ {
		time.Sleep(1 * time.Second)

		// 尝试从独立的session key读取，如果能读到说明已被认领
		if RedisSessionGet(SessionID, &redisJson) == nil {
			sessionClaimed = true
			log.Printf("会话已被Remote端认领: %s", SessionID)
			break
		}
	}

	if !sessionClaimed {
		// 超时未被认领，从Hash中移除注册
		RedisSessionUnregister(SessionID)
		log.Println("等待Remote端认领超时")
		return tun_active, tun_passive, nil, nil, nil
	}

	// 阶段2: 使用独立的session key进行后续通信
	for m_local_state == 1 {
		time.Sleep(1 * time.Second)

		if RedisSessionGet(SessionID, &redisJson) != nil {
			log.Println("会话超时")
			return tun_active, tun_passive, nil, nil, nil
		}

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			log.Println("会话被重置")
			return tun_active, tun_passive, nil, nil, nil
		}

		switch redisJson.State {
		case 1:
			if redisJson.RemoteVersion != GetVersion() {
				log.Printf("两端版本不兼容: %v", redisJson)
				RedisSessionDel(SessionID)
				return tun_active, tun_passive, nil, nil, errors.New("两端版本不兼容")
			}

			log.Printf("收到Remote端地址: %v", redisJson.RemoteAddr)

			switch conn_type {
			case 0:
				if tun_passive != nil {
					tun_passive.Release()
				}
				tun_active = nil

				redisJson.LocalAddr = *addr

				tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), conn, &redisJson.LocalAddr, &redisJson.RemoteAddr, redisJson.SendPortCount, time.Duration(config.Arg_conn_passive_send_time)*time.Millisecond, &m_upnp_bind)
				tun_passive.Start()

				redisJson.State = 2
				log.Printf("发送Local端地址: %v", redisJson.LocalAddr)
				RedisSessionSet(SessionID, redisJson.RedisTimeOut, &redisJson)

			default:
				if tun_active != nil {
					tun_active.Release()
				}
				tun_passive = nil

				tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), conn, &redisJson.LocalAddr, &redisJson.RemoteAddr, time.Duration(config.Arg_conn_active_send_time)*time.Millisecond, &m_upnp_bind)
				tun_active.Start()

				redisJson.State = 2
				RedisSessionSet(SessionID, redisJson.RedisTimeOut, &redisJson)
			}

		case 3:
			if tun_passive != nil {
				if tun_passive.TunQuicConn != nil {
					log.Printf("连接成功")
					log.Println("[GOODLINK_STATUS]connected")
					return tun_active, tun_passive, tun_passive.TunQuicConn, tun_passive.TunHealthStream, nil
				}
			}
			if tun_active != nil {
				if tun_active.TunQuicConn != nil {
					log.Printf("连接成功")
					log.Println("[GOODLINK_STATUS]connected")
					return tun_active, tun_passive, tun_active.TunQuicConn, tun_active.TunHealthStream, nil
				}
			}
			log.Println("连接失败")
			return tun_active, tun_passive, nil, nil, nil

		case 4:
			log.Println("连接超时")
			return tun_active, tun_passive, nil, nil, nil

		default:
			log.Printf("等待Remote端状态: Local: %v => Remote: %v", redisJson.LocalAddr, redisJson.RemoteAddr)
		}
	}

	return tun_active, tun_passive, nil, nil, nil
}

func GetLocalStats() int {
	return m_local_state
}

func StopLocal() error {
	m_local_state = 0
	Release(m_tun_active, m_tun_passive)
	return nil
}

func RunLocal(tun_key string) error {
	m_local_state = 1

	m_tun_key = tun_key
	m_md5_tun_key = go2.Md5Encode(tun_key)

	count := 0

	var udp_conn *net.UDPConn
	var addr tun.AddrType

	for m_local_state == 1 {

		if udp_conn != nil {
			udp_conn.Close()
		}
		udp_conn, addr = GetUDPAddr()

		log.Printf("Local端地址: %v", addr)
		log.Println("[GOODLINK_STATUS]connecting")

		if !g_netstack_started {
			if err := netstack.Start(); err != nil {
				return err
			}
			g_netstack_started = true
		}

		count++

		tun_active, tun_passive, quic_conn, health, err := GetLocalQuicConn(udp_conn, &addr, count)
		if err != nil {
			Release(tun_active, tun_passive)
			return err
		}
		if quic_conn == nil {
			Release(tun_active, tun_passive)
			continue
		}

		m_tun_active = tun_active
		m_tun_passive = tun_passive

		netstack.SetForWarder(quic_conn)
		log.Printf("Remote端IP: %s", netstack.GetRemoteIP())

		m_local_state = 2
		tun.ProcessHealth(health)
		if m_local_state != 0 {
			m_local_state = 1
			log.Println("[GOODLINK_STATUS]connecting")
		}
		log.Printf("释放连接: %v", quic_conn.LocalAddr())
		Release(tun_active, tun_passive)

		netstack.SetForWarder(nil)
		count = 0
	}

	return nil
}
