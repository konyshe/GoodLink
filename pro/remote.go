package pro

import (
	"goodlink/config"
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/utils"
	"goodlink2/tun"
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

func GetRemoteQuicConn() (*net.UDPConn, *tun.TunActive, *tun.TunPassive, quic.Connection, quic.Stream) {
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive
	var udp_conn *net.UDPConn

	redisJson := RedisJsonType{}
	last_state := redisJson.State
	conn_type := 0 //主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	for RedisGet(&redisJson) != nil && m_remote_stats == 1 {
		time.Sleep(5 * time.Second)
	}

	redisJson.RemoteVersion = m_version

	SessionID := redisJson.SessionID
	log.Printf("会话ID: %s", SessionID)

	for m_remote_stats == 1 {
		time.Sleep(1 * time.Second)

		if RedisGet(&redisJson) != nil {
			log.Println("会话超时")
			return udp_conn, tun_active, tun_passive, nil, nil
		}
		redisJson.RemoteVersion = m_version

		//log.Printf("状态消息: %v", redisJson)

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			utils.Log().Debug("会话被重置")
			return udp_conn, tun_active, tun_passive, nil, nil
		}

		if redisJson.State < last_state {
			m_redis_db.Del(m_md5_tun_key)
			utils.Log().DebugF("状态异常: %d -> %d", last_state, redisJson.State)
			return udp_conn, tun_active, tun_passive, nil, nil
		}

		if redisJson.State != 3 && redisJson.State != 4 && redisJson.State-last_state > 1 {
			m_redis_db.Del(m_md5_tun_key)
			utils.Log().DebugF("状态异常: %d -> %d", last_state, redisJson.State)
			return udp_conn, tun_active, tun_passive, nil, nil
		}

		redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
		redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

		switch redisJson.State {
		case 0:
			utils.Log().DebugF("收到对端请求: %v", redisJson)

			redisJson.State = 1

			if redisJson.LocalVersion != m_version {
				utils.Log().DebugF("两端版本不兼容: %v", redisJson)
				RedisSet(redisJson.SocketTimeOut*3, &redisJson)
				return udp_conn, tun_active, tun_passive, nil, nil
			}

			udp_conn, redisJson.RemoteAddr = GetUDPAddr()

			switch redisJson.LocalAddr.WanPort1 {
			case 0:
				conn_type = 0
				utils.Log().Debug("对端未发来IP")

				if tun_active != nil {
					tun_active.Release()
				}
				tun_passive = nil

				tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, time.Duration(config.Arg_conn_active_send_time)*time.Millisecond)
				tun_active_chain = tun_active.GetChain()

				redisJson.SendPortCount = 0x100

			default:
				utils.Log().DebugF("对端有发来IP: %v", redisJson.LocalAddr)
				conn_type = 1

				if tun_passive != nil {
					tun_passive.Release()
				}
				tun_active = nil

				tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, 0x100, time.Duration(config.Arg_conn_passive_send_time)*time.Millisecond)
				tun_passive.Start()

				tun_passive_chain = tun_passive.GetChain()
			}

			utils.Log().DebugF("发送本端地址: %v", redisJson.RemoteAddr)
			RedisSet(redisJson.RedisTimeOut, &redisJson)

		case 2:
			switch conn_type {
			case 0:
				utils.Log().DebugF("收到对端地址: %v", redisJson.LocalAddr)
				tun_active.Start()

			case 1:
				utils.Log().DebugF("收到对端地址, 等待连接: %v", redisJson.LocalAddr)
			}

			select {
			case <-tun_active_chain:
				redisJson.State = 3
				utils.Log().Debug("对端被动连接成功")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				if tun_active != nil {
					return udp_conn, tun_active, tun_passive, tun_active.TunQuicConn, tun_active.TunHealthStream
				}
				return udp_conn, tun_active, tun_passive, nil, nil

			case <-tun_passive_chain:
				redisJson.State = 3
				utils.Log().Debug("对端主动连接成功")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				if tun_passive != nil {
					return udp_conn, tun_active, tun_passive, tun_passive.TunQuicConn, tun_passive.TunHealthStream
				}
				return udp_conn, tun_active, tun_passive, nil, nil

			case <-time.After(time.Duration(config.Arg_p2p_timeout) * time.Second):
				redisJson.State = 4
				utils.Log().Debug("对端连接超时")
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return udp_conn, tun_active, tun_passive, nil, nil
			}

		case 3, 4:

		default:
			utils.Log().DebugF("等待对端状态: Local: %v => Remote: %v", redisJson.LocalAddr, redisJson.RemoteAddr)
		}

		last_state = redisJson.State
	}

	return udp_conn, tun_active, tun_passive, nil, nil
}

var (
	lock_remote      sync.Mutex
	tun_active_list  []*tun.TunActive
	tun_passive_list []*tun.TunPassive
	udp_conn_list    []*net.UDPConn
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

	if udp_conn_list != nil {
		for _, udp_conn := range udp_conn_list {
			udp_conn.Close()
		}
		udp_conn_list = nil
	}

	return nil
}

func RunRemote(tun_key string) error {
	var wg sync.WaitGroup

	tun_active_list = make([]*tun.TunActive, 0)
	tun_passive_list = make([]*tun.TunPassive, 0)
	udp_conn_list = make([]*net.UDPConn, 0)

	m_remote_stats = 1

	m_tun_key = tun_key
	m_md5_tun_key = md5.Encode(tun_key)

	for m_remote_stats == 1 {

		udp_conn, tun_active, tun_passive, quic_conn, health := GetRemoteQuicConn()
		if quic_conn == nil {
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
		if udp_conn != nil {
			udp_conn_list = append(udp_conn_list, udp_conn)
		}
		lock_remote.Unlock()

		wg.Add(1)
		go func(tun_active2 *tun.TunActive, tun_passive2 *tun.TunPassive, quic_conn2 quic.Connection) {
			defer func() {
				Release(tun_active2, tun_passive2)
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				defer wg.Done()
				proxy.ProcessProxyServer(quic_conn2)
			}()

			tun.ProcessHealth(health)
			utils.Log().DebugF("释放连接: %v", quic_conn2.LocalAddr())
		}(tun_active, tun_passive, quic_conn)
	}

	wg.Wait()
	return nil
}
