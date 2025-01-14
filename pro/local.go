package pro

import (
	"fmt"
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/stun2"
	"goodlink/utils"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"net"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_local_state = 0 //0: 停止, 1: 启动, 2: 连接成功
)

func GetLocalQuicConn(conn_type int, count int) (quic.Connection, quic.Stream, error) {
	redisJson := RedisJsonType{
		ConnectCount: count,
	}

	conn := utils.GetListenUDP()
	redisJson.LocalPort0 = conn.LocalAddr().(*net.UDPAddr).Port
	LocalIP, LocalPort1, LocalPort2 := stun2.GetWanIpPort2(conn)
	if LocalPort1 == LocalPort2 {
		conn_type = 0
	}

	SessionID := string(utils.RandomBytes(24))
	redisJson.SessionID = SessionID
	utils.Log().DebugF("会话ID: %s", SessionID)

	switch conn_type {
	case 0:
		utils.Log().Debug("请求连接对端")
		RedisSet(15*time.Second, &redisJson)

	default:
		redisJson.LocalIP, redisJson.LocalPort1, redisJson.LocalPort2 = LocalIP, LocalPort1, LocalPort2
		redisJson.State = 0
		utils.Log().DebugF("发送本端地址: %v", redisJson)
		RedisSet(15*time.Second, &redisJson)
	}

	for m_local_state == 1 {
		time.Sleep(1 * time.Second)

		if RedisGet(&redisJson) != nil {
			utils.Log().Debug("会话超时")
			return nil, nil, nil
		}

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			utils.Log().Debug("会话被重置")
			return nil, nil, nil
		}

		switch redisJson.State {
		case 1:
			utils.Log().DebugF("收到对端地址: %v", redisJson)

			switch conn_type {
			case 0:
				if m_tun_passive != nil {
					m_tun_passive.Release()
				}
				m_tun_active = nil

				redisJson.LocalIP, redisJson.LocalPort1, redisJson.LocalPort2 = LocalIP, LocalPort1, LocalPort2
				if redisJson.LocalIP == redisJson.RemoteIP {
					RedisDel()
					return nil, nil, fmt.Errorf("已经和对端处在同一个公网下")
				}

				m_tun_passive = tun.CteateTunPassive([]byte(redisJson.SessionID), conn, redisJson.RemoteIP, redisJson.RemotePort1, redisJson.RemotePort2, redisJson.SendPortCount)
				m_tun_passive.Start()

				redisJson.State = 2
				utils.Log().DebugF("发送本端地址: %v", redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

				//go m_tun_passive.Start()

			default:
				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				if redisJson.LocalIP == redisJson.RemoteIP {
					RedisDel()
					return nil, nil, fmt.Errorf("已经和对端处在同一个公网下")
				}

				m_tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), conn, 15*time.Second)
				m_tun_active.Start(redisJson.LocalPort1, redisJson.LocalPort2, redisJson.RemoteIP, redisJson.RemotePort1, redisJson.RemotePort2, redisJson.SocketTimeOut)
				redisJson.State = 2
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 3:
			if m_tun_passive != nil {
				if m_tun_passive.TunQuicConn != nil {
					utils.Log().DebugF("连接成功")
					return m_tun_passive.TunQuicConn, m_tun_passive.TunHealthStream, nil
				}
			}
			if m_tun_active != nil {
				if m_tun_active.TunQuicConn != nil {
					utils.Log().DebugF("连接成功")
					return m_tun_active.TunQuicConn, m_tun_active.TunHealthStream, nil
				}
			}
			utils.Log().Debug("连接失败")
			return nil, nil, nil

		case 4:
			utils.Log().Debug("连接超时")
			return nil, nil, nil

		default:
			utils.Log().Debug("等待对端状态")
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
		utils.Log().DebugF("释放连接: %v", conn.LocalAddr())
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
