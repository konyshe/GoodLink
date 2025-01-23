package pro

import (
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/utils"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_remote_stats int
)

func GetRemoteQuicConn(conn *net.UDPConn, addr *AddrType, time_out time.Duration) (*tun.TunActive, *tun.TunPassive, quic.Connection, quic.Stream) {
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive

	redisJson := RedisJsonType{}
	last_state := redisJson.State
	conn_type := 0 //主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	for RedisGet(&redisJson) != nil && m_remote_stats == 1 {
		time.Sleep(5 * time.Second)
	}

	SessionID := redisJson.SessionID
	log.Printf("会话ID: %s", SessionID)

	for m_remote_stats == 1 {
		time.Sleep(1 * time.Second)

		if RedisGet(&redisJson) != nil {
			log.Println("会话超时")
			return tun_active, tun_passive, nil, nil
		}

		//log.Printf("状态消息: %v", redisJson)

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			utils.Log().Debug("会话被重置")
			return tun_active, tun_passive, nil, nil
		}

		if redisJson.State < last_state {
			m_redis_db.Del(m_md5_tun_key)
			utils.Log().DebugF("状态异常: %d -> %d", last_state, redisJson.State)
			return tun_active, tun_passive, nil, nil
		}

		if redisJson.State != 3 && redisJson.State != 4 && redisJson.State-last_state > 1 {
			m_redis_db.Del(m_md5_tun_key)
			utils.Log().DebugF("状态异常: %d -> %d", last_state, redisJson.State)
			return tun_active, tun_passive, nil, nil
		}

		redisJson.SocketTimeOut = time_out
		redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

		switch redisJson.State {
		case 0:
			utils.Log().DebugF("收到对端请求: %v", redisJson)

			redisJson.RemoteAddr = *addr

			switch redisJson.LocalAddr.WanPort1 {
			case 0:
				conn_type = 0
				utils.Log().Debug("对端未发来IP")

				if tun_active != nil {
					tun_active.Release()
				}
				tun_passive = nil

				tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), conn, time_out)
				tun_active_chain = tun_active.GetChain()

				redisJson.State = 1
				redisJson.SendPortCount = 0x100
				utils.Log().DebugF("发送本端地址: %v", redisJson.RemoteAddr)
				RedisSet(redisJson.RedisTimeOut, &redisJson)

			default:
				utils.Log().DebugF("对端有发来IP: %v", redisJson.LocalAddr)
				conn_type = 1

				if tun_passive != nil {
					tun_passive.Release()
				}
				tun_active = nil

				tun_passive = tun.CteateTunPassive([]byte(redisJson.SessionID), conn, redisJson.LocalAddr.WanIPv4, redisJson.LocalAddr.WanPort1, redisJson.LocalAddr.WanPort2, 0x100)
				tun_passive.Start()

				tun_passive_chain = tun_passive.GetChain()

				redisJson.State = 1
				utils.Log().DebugF("发送本端地址: %v", redisJson.RemoteAddr)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
			}

		case 2:
			switch conn_type {
			case 0:
				utils.Log().DebugF("收到对端地址: %v", redisJson.LocalAddr)
				tun_active.Start(redisJson.RemoteAddr.WanPort1, redisJson.RemoteAddr.WanPort2, redisJson.LocalAddr.WanIPv4, redisJson.LocalAddr.WanPort1, redisJson.LocalAddr.WanPort2, redisJson.SocketTimeOut)

			case 1:
				utils.Log().DebugF("收到对端地址, 等待连接: %v", redisJson.LocalAddr)
			}

			select {
			case <-tun_active_chain:
				redisJson.State = 3
				utils.Log().Debug("对端被动连接成功")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				if tun_active != nil {
					return tun_active, tun_passive, tun_active.TunQuicConn, tun_active.TunHealthStream
				}
				return tun_active, tun_passive, nil, nil

			case <-tun_passive_chain:
				redisJson.State = 3
				utils.Log().Debug("对端主动连接成功")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				if tun_passive != nil {
					return tun_active, tun_passive, tun_passive.TunQuicConn, tun_passive.TunHealthStream
				}
				return tun_active, tun_passive, nil, nil

			case <-time.After(time_out):
				redisJson.State = 4
				utils.Log().Debug("对端连接超时")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return tun_active, tun_passive, nil, nil
			}

		case 3, 4:

		default:
			utils.Log().DebugF("等待对端状态: Local: %v => Remote: %v", redisJson.LocalAddr, redisJson.RemoteAddr)
		}

		last_state = redisJson.State
	}

	return tun_active, tun_passive, nil, nil
}

var (
	lock_remote      sync.Mutex
	tun_active_list  []*tun.TunActive
	tun_passive_list []*tun.TunPassive
)

func StopRemote() error {
	m_remote_stats = 0

	lock_remote.Lock()
	defer lock_remote.Unlock()

	if tun_active_list != nil {
		for _, tun_active := range tun_active_list {
			tun_active.Release()
		}
		tun_active_list = nil
	}

	if tun_passive_list != nil {
		for _, tun_passive := range tun_passive_list {
			tun_passive.Release()
		}
		tun_passive_list = nil
	}

	return nil
}

func RunRemote(remote_addr string, tun_key string, time_out time.Duration) error {
	var wg sync.WaitGroup

	tun_active_list = make([]*tun.TunActive, 0)
	tun_passive_list = make([]*tun.TunPassive, 0)

	m_remote_stats = 1

	m_tun_key = tun_key
	m_md5_tun_key = md5.Encode(tun_key)

	conn, addr := GetUDPAddr()

	for m_remote_stats == 1 {

		conn.Close()
		conn = utils.GetListenUDPPort("udp", addr.LocalPort) // 同时监听IPv4和IPv6
		if conn == nil {
			utils.Log().DebugF("绑定端口失败: %v", addr.LocalPort)
			time.Sleep(time.Second)
			continue
		}
		log.Printf("本端地址: %v", addr)

		conn.SetReadDeadline(time.Time{})
		conn.SetWriteDeadline(time.Time{})
		conn.SetDeadline(time.Time{})

		tun_active, tun_passive, conn, health := GetRemoteQuicConn(conn, &addr, time_out)
		if conn == nil {
			Release(tun_active, tun_passive)
			continue
		}

		lock_remote.Lock()
		if tun_active != nil {
			tun_active_list = append(tun_active_list, tun_active)
		}
		if tun_passive != nil {
			tun_passive_list = append(tun_passive_list, tun_passive)
		}
		lock_remote.Unlock()

		wg.Add(1)
		go func(tun_active2 *tun.TunActive, tun_passive2 *tun.TunPassive, remote_addr string, conn quic.Connection) {
			defer func() {
				Release(tun_active2, tun_passive2)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				proxy.ProcessProxyServer(remote_addr, conn)
			}()

			tun.ProcessHealth(health)
			utils.Log().DebugF("释放连接: %v", conn.LocalAddr())
		}(tun_active, tun_passive, remote_addr, conn)
	}

	wg.Wait()
	return nil
}
