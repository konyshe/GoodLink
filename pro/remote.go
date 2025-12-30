package pro

import (
	"go2"
	"goodlink/config"
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
	m_remote_stats        int
	m_processing_sessions sync.Map // 记录正在处理的 SessionID，避免同一进程内重复处理
)

// processSession 处理单个会话的完整生命周期
// 由主循环认领会话后启动，接收已认领的 SessionID 和 redisJson
func processSession(sessionID string, redisJson RedisJsonType, wg *sync.WaitGroup) {
	defer wg.Done()
	defer m_processing_sessions.Delete(sessionID) // 处理完成后从本地 map 移除

	// 独立的资源，每个会话完全隔离
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive
	var udp_conn *net.UDPConn

	conn_type := 0 //主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	// 清理函数，确保资源正确释放
	defer func() {
		if tun_active != nil {
			tun_active.Release()
		}
		if tun_passive != nil {
			tun_passive.Release()
		}
		if udp_conn != nil {
			udp_conn.Close()
		}
	}()

	redisJson.RemoteVersion = GetVersion()

	// 阶段2: 处理会话状态为0，发送Remote端信息，并写入独立的session key
	utils.Log().DebugF("会话 %s 收到Local端请求: %v", sessionID, redisJson)

	redisJson.State = 1
	redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
	redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

	if redisJson.LocalVersion != GetVersion() {
		utils.Log().DebugF("会话 %s 两端版本不兼容: Local: %v => Remote: %v", sessionID, redisJson.LocalVersion, GetVersion())
		RedisSessionSet(sessionID, redisJson.SocketTimeOut*3, &redisJson)
		return
	}

	udp_conn, redisJson.RemoteAddr = GetUDPAddr()

	switch redisJson.LocalAddr.WanPort1 {
	case 0:
		conn_type = 0
		utils.Log().DebugF("会话 %s Local端未发来IP", sessionID)

		tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, time.Duration(config.Arg_conn_active_send_time)*time.Millisecond, &m_upnp_bind)
		tun_active_chain = tun_active.GetChain()

		redisJson.SendPortCount = 0x100

	default:
		utils.Log().DebugF("会话 %s Local端有发来IP: %v", sessionID, redisJson.LocalAddr)
		conn_type = 1

		tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, 0x100, time.Duration(config.Arg_conn_passive_send_time)*time.Millisecond, &m_upnp_bind)
		tun_passive.Start()

		tun_passive_chain = tun_passive.GetChain()
	}

	utils.Log().DebugF("会话 %s 发送Remote端地址: %v", sessionID, redisJson.RemoteAddr)
	// 写入独立的session key，通知Local端会话已被认领
	RedisSessionSet(sessionID, redisJson.RedisTimeOut, &redisJson)

	last_state := redisJson.State

	// 阶段3: 使用独立的session key进行后续通信
	for m_remote_stats == 1 {
		time.Sleep(1 * time.Second)

		if RedisSessionGet(sessionID, &redisJson) != nil {
			log.Printf("会话 %s 超时", sessionID)
			return
		}
		redisJson.RemoteVersion = GetVersion()

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, sessionID) {
			utils.Log().DebugF("会话 %s 被重置", sessionID)
			return
		}

		if redisJson.State < last_state {
			RedisSessionDel(sessionID)
			utils.Log().DebugF("会话 %s 状态异常: %d -> %d", sessionID, last_state, redisJson.State)
			return
		}

		if redisJson.State != 3 && redisJson.State != 4 && redisJson.State-last_state > 1 {
			RedisSessionDel(sessionID)
			utils.Log().DebugF("会话 %s 状态异常: %d -> %d", sessionID, last_state, redisJson.State)
			return
		}

		redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
		redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

		switch redisJson.State {
		case 2:
			switch conn_type {
			case 0:
				utils.Log().DebugF("会话 %s 收到Local端地址: %v", sessionID, redisJson.LocalAddr)
				tun_active.Start()

			case 1:
				utils.Log().DebugF("会话 %s 收到Local端地址, 等待连接: %v", sessionID, redisJson.LocalAddr)
			}

			select {
			case <-tun_active_chain:
				redisJson.State = 3
				utils.Log().DebugF("会话 %s Local端被动连接成功", sessionID)
				RedisSessionSet(sessionID, redisJson.RedisTimeOut, &redisJson)
				if tun_active != nil && tun_active.TunQuicConn != nil {
					// 连接成功，启动代理和健康检查
					handleConnection(sessionID, tun_active.TunQuicConn, tun_active.TunHealthStream)
				}
				return

			case <-tun_passive_chain:
				redisJson.State = 3
				utils.Log().DebugF("会话 %s Local端主动连接成功", sessionID)
				RedisSessionSet(sessionID, redisJson.RedisTimeOut, &redisJson)
				if tun_passive != nil && tun_passive.TunQuicConn != nil {
					// 连接成功，启动代理和健康检查
					handleConnection(sessionID, tun_passive.TunQuicConn, tun_passive.TunHealthStream)
				}
				return

			case <-time.After(time.Duration(config.Arg_p2p_timeout) * time.Second):
				redisJson.State = 4
				utils.Log().DebugF("会话 %s Local端连接超时", sessionID)
				RedisSessionSet(sessionID, redisJson.RedisTimeOut, &redisJson)
				return
			}

		case 3, 4:
			return

		default:
			utils.Log().DebugF("会话 %s 等待Local端状态: Local: %v => Remote: %v", sessionID, redisJson.LocalAddr, redisJson.RemoteAddr)
		}

		last_state = redisJson.State
	}
}

// handleConnection 处理已建立的连接
func handleConnection(sessionID string, quicConn quic.Connection, healthStream quic.Stream) {
	utils.Log().DebugF("开始处理连接: %s", sessionID)

	// 启动代理服务
	go proxy.ProcessProxyServer(quicConn)

	// 阻塞等待健康检查结束
	tun.ProcessHealth(healthStream)

	utils.Log().DebugF("释放连接: %v, SessionID: %s", quicConn.LocalAddr(), sessionID)
}

func StopRemote() error {
	m_remote_stats = 0
	// 清空正在处理的会话记录
	m_processing_sessions.Range(func(key, value any) bool {
		m_processing_sessions.Delete(key)
		return true
	})
	return nil
}

func RunRemote(tun_key string) error {
	var wg sync.WaitGroup

	m_remote_stats = 1

	m_tun_key = tun_key
	m_md5_tun_key = go2.Md5Encode(tun_key)

	log.Println("Remote端启动，等待Local端连接...")

	// 主循环扫描待处理的会话
	for m_remote_stats == 1 {
		pendingSessions, err := RedisSessionScan()
		if err != nil {
			utils.Log().DebugF("扫描会话失败: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}

		if len(pendingSessions) == 0 {
			time.Sleep(3 * time.Second)
			continue
		}

		// 尝试认领待处理的会话
		for _, session := range pendingSessions {
			// 检查本地是否已在处理
			if _, exists := m_processing_sessions.Load(session.SessionID); exists {
				continue
			}

			redisJson := RedisJsonType{}
			if err := RedisSessionClaim(session.SessionID, &redisJson, 30*time.Second); err != nil {
				// 可能被其他节点认领了，继续尝试下一个
				continue
			}

			// 标记为正在处理
			m_processing_sessions.Store(session.SessionID, true)

			log.Printf("认领会话: %s", session.SessionID)
			wg.Add(1)
			go processSession(session.SessionID, redisJson, &wg)
		}
	}

	// 等待所有 Worker 协程结束
	wg.Wait()

	log.Println("Remote端已停止")
	return nil
}
