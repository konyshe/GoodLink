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
	m_remote_stats int
)

// remoteWorker 是一个独立的 Worker 协程，循环处理多个会话
// 每个 Worker 拥有完全独立的资源，不与其他 Worker 共享状态
func remoteWorker(workerID int, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Printf("Worker %d 启动", workerID)

	for m_remote_stats == 1 {
		// 处理单个会话，所有资源在此函数内独立管理
		processOneSession(workerID)
	}

	log.Printf("Worker %d 停止", workerID)
}

// processOneSession 处理单个会话的完整生命周期
// 所有资源（udp_conn, tun_active, tun_passive, channels）都在此函数内声明，完全隔离
func processOneSession(workerID int) {
	// 独立的资源，每个会话完全隔离
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive
	var udp_conn *net.UDPConn

	redisJson := RedisJsonType{}
	conn_type := 0 //主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	var SessionID string

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

	// 阶段1: 轮询扫描待处理的会话，认领一个SessionID
	for m_remote_stats == 1 {
		// 扫描Hash中所有待处理的会话
		pendingSessions, err := RedisSessionScan()
		if err != nil {
			utils.Log().DebugF("Worker %d 扫描会话失败: %v", workerID, err)
			time.Sleep(3 * time.Second)
			continue
		}

		if len(pendingSessions) == 0 {
			time.Sleep(3 * time.Second)
			continue
		}

		// 尝试认领一个待处理的会话
		for _, session := range pendingSessions {
			if err := RedisSessionClaim(session.SessionID, &redisJson, 30*time.Second); err != nil {
				// 可能被其他 Worker 认领了，继续尝试下一个
				continue
			}

			SessionID = session.SessionID
			log.Printf("Worker %d 成功认领会话ID: %s", workerID, SessionID)
			break
		}

		if SessionID != "" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	if SessionID == "" || m_remote_stats != 1 {
		return
	}

	redisJson.RemoteVersion = GetVersion()

	// 阶段2: 处理会话状态为0，发送Remote端信息，并写入独立的session key
	utils.Log().DebugF("Worker %d 收到Local端请求: %v", workerID, redisJson)

	redisJson.State = 1
	redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
	redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

	if redisJson.LocalVersion != GetVersion() {
		utils.Log().DebugF("Worker %d 两端版本不兼容: Local: %v => Remote: %v", workerID, redisJson.LocalVersion, GetVersion())
		RedisSessionSet(SessionID, redisJson.SocketTimeOut*3, &redisJson)
		return
	}

	udp_conn, redisJson.RemoteAddr = GetUDPAddr()

	switch redisJson.LocalAddr.WanPort1 {
	case 0:
		conn_type = 0
		utils.Log().DebugF("Worker %d Local端未发来IP", workerID)

		tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, time.Duration(config.Arg_conn_active_send_time)*time.Millisecond, &m_upnp_bind)
		tun_active_chain = tun_active.GetChain()

		redisJson.SendPortCount = 0x100

	default:
		utils.Log().DebugF("Worker %d Local端有发来IP: %v", workerID, redisJson.LocalAddr)
		conn_type = 1

		tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, 0x100, time.Duration(config.Arg_conn_passive_send_time)*time.Millisecond, &m_upnp_bind)
		tun_passive.Start()

		tun_passive_chain = tun_passive.GetChain()
	}

	utils.Log().DebugF("Worker %d 发送Remote端地址: %v", workerID, redisJson.RemoteAddr)
	// 写入独立的session key，通知Local端会话已被认领
	RedisSessionSet(SessionID, redisJson.RedisTimeOut, &redisJson)

	last_state := redisJson.State

	// 阶段3: 使用独立的session key进行后续通信
	for m_remote_stats == 1 {
		time.Sleep(1 * time.Second)

		if RedisSessionGet(SessionID, &redisJson) != nil {
			log.Printf("Worker %d 会话超时: %s", workerID, SessionID)
			return
		}
		redisJson.RemoteVersion = GetVersion()

		utils.Log().SetDebugSate(redisJson.State)

		if !strings.EqualFold(redisJson.SessionID, SessionID) {
			utils.Log().DebugF("Worker %d 会话被重置", workerID)
			return
		}

		if redisJson.State < last_state {
			RedisSessionDel(SessionID)
			utils.Log().DebugF("Worker %d 状态异常: %d -> %d", workerID, last_state, redisJson.State)
			return
		}

		if redisJson.State != 3 && redisJson.State != 4 && redisJson.State-last_state > 1 {
			RedisSessionDel(SessionID)
			utils.Log().DebugF("Worker %d 状态异常: %d -> %d", workerID, last_state, redisJson.State)
			return
		}

		redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
		redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

		switch redisJson.State {
		case 2:
			switch conn_type {
			case 0:
				utils.Log().DebugF("Worker %d 收到Local端地址: %v", workerID, redisJson.LocalAddr)
				tun_active.Start()

			case 1:
				utils.Log().DebugF("Worker %d 收到Local端地址, 等待连接: %v", workerID, redisJson.LocalAddr)
			}

			select {
			case <-tun_active_chain:
				redisJson.State = 3
				utils.Log().DebugF("Worker %d Local端被动连接成功", workerID)
				RedisSessionSet(SessionID, redisJson.RedisTimeOut, &redisJson)
				if tun_active != nil && tun_active.TunQuicConn != nil {
					// 连接成功，启动代理和健康检查
					handleConnection(workerID, SessionID, tun_active.TunQuicConn, tun_active.TunHealthStream)
				}
				return

			case <-tun_passive_chain:
				redisJson.State = 3
				utils.Log().DebugF("Worker %d Local端主动连接成功", workerID)
				RedisSessionSet(SessionID, redisJson.RedisTimeOut, &redisJson)
				if tun_passive != nil && tun_passive.TunQuicConn != nil {
					// 连接成功，启动代理和健康检查
					handleConnection(workerID, SessionID, tun_passive.TunQuicConn, tun_passive.TunHealthStream)
				}
				return

			case <-time.After(time.Duration(config.Arg_p2p_timeout) * time.Second):
				redisJson.State = 4
				utils.Log().DebugF("Worker %d Local端连接超时", workerID)
				RedisSessionSet(SessionID, redisJson.RedisTimeOut, &redisJson)
				return
			}

		case 3, 4:
			return

		default:
			utils.Log().DebugF("Worker %d 等待Local端状态: Local: %v => Remote: %v", workerID, redisJson.LocalAddr, redisJson.RemoteAddr)
		}

		last_state = redisJson.State
	}
}

// handleConnection 处理已建立的连接
func handleConnection(workerID int, sessionID string, quicConn quic.Connection, healthStream quic.Stream) {
	utils.Log().DebugF("Worker %d 开始处理连接: %s", workerID, sessionID)

	// 启动代理服务
	go proxy.ProcessProxyServer(quicConn)

	// 阻塞等待健康检查结束
	tun.ProcessHealth(healthStream)

	utils.Log().DebugF("Worker %d 释放连接: %v, SessionID: %s", workerID, quicConn.LocalAddr(), sessionID)
}

// Worker 数量，可根据需要调整
const remoteWorkerCount = 10

func StopRemote() error {
	m_remote_stats = 0
	return nil
}

func RunRemote(tun_key string) error {
	var wg sync.WaitGroup

	m_remote_stats = 1

	m_tun_key = tun_key
	m_md5_tun_key = go2.Md5Encode(tun_key)

	log.Printf("启动 %d 个 Worker 协程", remoteWorkerCount)

	// 启动多个 Worker 协程并行处理会话
	for i := 0; i < remoteWorkerCount; i++ {
		wg.Add(1)
		go remoteWorker(i, &wg)
	}

	// 等待所有 Worker 协程结束
	wg.Wait()

	log.Println("所有 Worker 协程已停止")
	return nil
}
