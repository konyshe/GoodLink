package pro

import (
	"fmt"
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/utils"
	"goodlink2/tun"
	"log"
	"net"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_local_state = 0 //0: 停止, 1: 启动, 2: 连接成功
)

func GetLocalQuicConn(conn *net.UDPConn, addr *tun.AddrType, conn_type int, count int) (*tun.TunActive, *tun.TunPassive, quic.Connection, quic.Stream, error) {
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive

	SessionID := string(utils.RandomBytes(24))
	utils.Log().DebugF("会话ID: %s", SessionID)

	redisJson := RedisJsonType{
		State:        0,
		SessionID:    SessionID,
		ConnectCount: count,
	}

	switch conn_type {
	case 0:
		utils.Log().Debug("请求连接对端")

	default:
		redisJson.LocalAddr = *addr
		utils.Log().DebugF("发送本端地址: %v", redisJson.LocalAddr)
	}

	RedisSet(15*time.Second, &redisJson)

	for m_local_state == 1 {
		time.Sleep(1 * time.Second)

		if RedisGet(&redisJson) != nil {
			log.Println("会话超时")
			return tun_active, tun_passive, nil, nil, nil
		}

		//log.Printf("状态消息: %v", redisJson)

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			utils.Log().Debug("会话被重置")
			return tun_active, tun_passive, nil, nil, nil
		}

		switch redisJson.State {
		case 1:
			utils.Log().DebugF("收到对端地址: %v", redisJson.RemoteAddr)

			switch conn_type {
			case 0:
				if tun_passive != nil {
					tun_passive.Release()
				}
				tun_active = nil

				redisJson.LocalAddr = *addr

				tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), conn, &redisJson.LocalAddr, &redisJson.RemoteAddr, redisJson.SendPortCount)
				tun_passive.Start()

				redisJson.State = 2
				utils.Log().DebugF("发送本端地址: %v", redisJson.LocalAddr)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				if tun_active != nil {
					tun_active.Release()
				}
				tun_passive = nil

				tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), conn, &redisJson.LocalAddr, &redisJson.RemoteAddr)
				tun_active.Start()

				redisJson.State = 2
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 3:
			if tun_passive != nil {
				if tun_passive.TunQuicConn != nil {
					utils.Log().DebugF("连接成功")
					return tun_active, tun_passive, tun_passive.TunQuicConn, tun_passive.TunHealthStream, nil
				}
			}
			if tun_active != nil {
				if tun_active.TunQuicConn != nil {
					utils.Log().DebugF("连接成功")
					return tun_active, tun_passive, tun_active.TunQuicConn, tun_active.TunHealthStream, nil
				}
			}
			utils.Log().Debug("连接失败")
			return tun_active, tun_passive, nil, nil, nil

		case 4:
			utils.Log().Debug("连接超时")
			return tun_active, tun_passive, nil, nil, nil

		default:
			utils.Log().DebugF("等待对端状态: Local: %v => Remote: %v", redisJson.LocalAddr, redisJson.RemoteAddr)
		}
	}

	return tun_active, tun_passive, nil, nil, nil
}

func GetLocalStats() int {
	return m_local_state
}

var (
	m_tun_active  *tun.TunActive
	m_tun_passive *tun.TunPassive
)

func StopLocal() error {
	m_local_state = 0
	Release(m_tun_active, m_tun_passive)
	return nil
}

func RunLocal(conn_type int, tun_local_addr string, tun_key string) error {
	m_local_state = 1

	utils.Log().DebugF("绑定端口: %v", tun_local_addr)

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

	conn, addr := GetUDPAddr()

	for m_local_state == 1 {

		conn.Close()
		conn, _ = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6zero, Port: addr.LocalPort})
		if conn == nil {
			continue
		}
		log.Printf("本端地址: %v", addr)

		count++

		tun_active, tun_passive, conn, health, err := GetLocalQuicConn(conn, &addr, conn_type, count)
		if err != nil {
			Release(tun_active, tun_passive)
			return err
		}
		if conn == nil {
			Release(tun_active, tun_passive)
			continue
		}

		m_tun_active = tun_active
		m_tun_passive = tun_passive

		go func() {
			proxy.ProcessProxyClient(listener, conn)
			chain <- 1
		}()

		m_local_state = 2
		tun.ProcessHealth(health)
		if m_local_state != 0 {
			m_local_state = 1
		}
		utils.Log().DebugF("释放连接: %v", conn.LocalAddr())
		Release(tun_active, tun_passive)

		if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
			conn.Write([]byte("hello"))
			conn.Close() // 关闭连接
		}

		<-chain
		count = 0
	}

	return nil
}
