package pro

import (
	"errors"
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/stun2"
	"goodlink/utils"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_remote_stats int
)

func GetRemoteQuicConn(time_out time.Duration) (quic.Connection, quic.Stream) {
	redisJson := RedisJsonType{}
	last_state := redisJson.State
	conn_type := 0 //主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	for RedisGet(&redisJson) != nil && m_remote_stats == 1 {
		time.Sleep(5 * time.Second)
	}

	SessionID := redisJson.SessionID
	utils.Log().DebugF("会话ID: %s", SessionID)

	for m_remote_stats == 1 {
		time.Sleep(1 * time.Second)

		if RedisGet(&redisJson) != nil {
			utils.Log().Debug("连接超时")
			return nil, nil
		}

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			utils.Log().Debug("连接被重置")
			return nil, nil
		}

		if redisJson.State < last_state {
			m_redis_db.Del(m_md5_tun_key)
			utils.Log().DebugF("状态异常: %d -> %d", last_state, redisJson.State)
			return nil, nil
		}

		if redisJson.State != 3 && redisJson.State != 4 && redisJson.State-last_state > 1 {
			m_redis_db.Del(m_md5_tun_key)
			utils.Log().DebugF("状态异常: %d -> %d", last_state, redisJson.State)
			return nil, nil
		}

		redisJson.SocketTimeOut = time_out
		redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

		switch redisJson.State {
		case 0:
			utils.Log().DebugF("收到对端请求: %v", redisJson)

			conn := utils.GetListenUDP()
			redisJson.ServerIP, redisJson.ServerPort1, redisJson.ServerPort2 = stun2.GetWanIpPort2(conn)

			switch redisJson.ClientPort1 {
			case 0:
				conn_type = 0
				utils.Log().Debug("对端未发来IP")

				if m_tun_active != nil {
					m_tun_active.Release()
				}
				m_tun_passive = nil

				m_tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), conn, time_out)
				tun_active_chain = m_tun_active.GetChain()

				redisJson.State = 1
				redisJson.SendPortCount = 0x100
				utils.Log().DebugF("发送本端地址: %v", redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				utils.Log().Debug("对端有发来IP")
				conn_type = 1

				if m_tun_passive != nil {
					m_tun_passive.Release()
				}
				m_tun_active = nil

				m_tun_passive = tun.CteateTunPassive([]byte(redisJson.SessionID), conn, redisJson.ClientIP, redisJson.ClientPort1, redisJson.ClientPort2, 0x100)
				m_tun_passive.Start()

				tun_passive_chain = m_tun_passive.GetChain()

				redisJson.State = 1
				utils.Log().DebugF("发送本端地址: %v", redisJson)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

				go m_tun_passive.Start()
			}

		case 2:
			switch conn_type {
			case 0:
				utils.Log().DebugF("收到对端地址: %v", redisJson)
				m_tun_active.Start(redisJson.ServerPort1, redisJson.ServerPort2, redisJson.ClientIP, redisJson.ClientPort1, redisJson.ClientPort2, redisJson.SocketTimeOut)

			case 1:
				utils.Log().DebugF("收到对端地址, 等待连接: %v", redisJson)
			}

			select {
			case <-tun_active_chain:
				redisJson.State = 3
				utils.Log().Debug("通知对端, 连接成功")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				if m_tun_active != nil {
					return m_tun_active.TunQuicConn, m_tun_active.TunHealthStream
				}
				return nil, nil

			case <-tun_passive_chain:
				redisJson.State = 3
				utils.Log().Debug("通知对端, 连接成功")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				if m_tun_passive != nil {
					return m_tun_passive.TunQuicConn, m_tun_passive.TunHealthStream
				}
				return nil, nil

			case <-time.After(time_out):
				redisJson.State = 4
				utils.Log().Debug("通知对端, 连接超时")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return nil, nil
			}

		case 3, 4:

		default:
			utils.Log().Debug("等待对端状态")
		}

		last_state = redisJson.State
	}

	return nil, nil
}

func StopRemote() error {
	m_remote_stats = 0
	Release()
	proxy.StopSocks5()
	return nil
}

func RunRemote(remote_addr string, tun_key string, time_out time.Duration) error {
	var wg sync.WaitGroup

	if remote_addr == "" {
		utils.Log().Debug("开启本地代理")
		remote_addr = utils.GetFreeLocalAddr()
		if remote_addr == "" {
			return errors.New("获取本地端口失败")
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			proxy.ListenSocks5(remote_addr)
		}()
	}

	utils.Log().DebugF("转发地址: %s", remote_addr)

	m_remote_stats = 1

	m_tun_key = tun_key
	m_md5_tun_key = md5.Encode(tun_key)

	for m_remote_stats == 1 {
		conn, health := GetRemoteQuicConn(time_out)
		if conn == nil {
			Release()
			continue
		}

		wg.Add(1)
		go func(remote string, conn quic.Connection) {
			defer func() {
				Release()
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				proxy.ProcessProxyServer(remote, conn)
			}()

			tun.ProcessHealth(health)
			utils.Log().DebugF("释放连接: %v", conn.LocalAddr())
		}(remote_addr, conn)
	}

	wg.Wait()
	return nil
}
