package pro

import (
	"errors"
	"go2"
	"goodlink/config"
	"goodlink/proxy"
	"goodlink/tun"
	"log"
	"net"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_remote_stats        int
	m_processing_sessions sync.Map // 记录正在处理的 SessionID，避免同一进程内重复处理
)

// handleState1_SendRemoteAddr 处理 State 1: 发送 Remote 端地址，创建 TUN 连接
func handleState1_SendRemoteAddr(sessionID string, redisJson *RedisJsonType, tun_active **tun.TunActive, tun_passive **tun.TunPassive, udp_conn **net.UDPConn, conn_type *int, tun_active_chain *chan quic.Connection, tun_passive_chain *chan quic.Connection) error {
	log.Printf("会话 %s State 1: 发送Remote端地址", sessionID)

	redisJson.RemoteVersion = GetVersion()
	redisJson.State = 1
	redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
	redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

	// 版本兼容性检查
	if redisJson.LocalVersion != GetVersion() {
		log.Printf("会话 %s 两端版本不兼容: Local: %s => Remote: %s", sessionID, redisJson.LocalVersion, GetVersion())
		RedisSessionSet(sessionID, redisJson.SocketTimeOut*3, redisJson)
		return errors.New("两端版本不兼容")
	}

	// 获取 UDP 地址
	*udp_conn, redisJson.RemoteAddr = GetUDPAddr()
	if redisJson.RemoteAddr.WanPort1 == redisJson.RemoteAddr.WanPort2 {
		log.Printf("WanPort %d:%d, 当前是NAT1-NAT3", redisJson.RemoteAddr.WanPort1, redisJson.RemoteAddr.WanPort2)
	} else {
		log.Printf("WanPort %d:%d, 当前是NAT4", redisJson.RemoteAddr.WanPort1, redisJson.RemoteAddr.WanPort2)
	}

	// 根据 Local 端是否发送地址决定连接类型
	switch redisJson.LocalAddr.WanPort1 {
	case 0:
		*conn_type = 0
		log.Printf("会话 %s Local端未发来IP，使用主动连接", sessionID)

		*tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), *udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, time.Duration(config.Arg_conn_active_send_time)*time.Millisecond, &m_upnp_bind)
		*tun_active_chain = (*tun_active).GetChain()

		redisJson.SendPortCount = 0x100

	default:
		log.Printf("会话 %s Local端有发来IP: %v，使用被动连接", sessionID, redisJson.LocalAddr)
		*conn_type = 1

		*tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), *udp_conn, &redisJson.RemoteAddr, &redisJson.LocalAddr, 0x100, time.Duration(config.Arg_conn_passive_send_time)*time.Millisecond, &m_upnp_bind)
		(*tun_passive).Start()

		*tun_passive_chain = (*tun_passive).GetChain()
	}

	log.Printf("会话 %s 发送Remote端地址: %v", sessionID, redisJson.RemoteAddr)
	// 写入独立的session key，通知Local端会话已被认领
	RedisSessionSet(sessionID, redisJson.RedisTimeOut, redisJson)

	return nil
}

// handleState2_WaitConnection 处理 State 2: 等待连接建立
func handleState2_WaitConnection(sessionID string, redisJson *RedisJsonType, conn_type int, tun_active *tun.TunActive, tun_passive *tun.TunPassive, tun_active_chain chan quic.Connection, tun_passive_chain chan quic.Connection) (bool, error) {
	log.Printf("会话 %s State 2: 等待连接建立", sessionID)

	switch conn_type {
	case 0:
		log.Printf("会话 %s 收到Local端地址: %v，启动主动连接", sessionID, redisJson.LocalAddr)
		if tun_active != nil {
			tun_active.Start()
		}

	case 1:
		log.Printf("会话 %s 收到Local端地址, 等待被动连接: %v", sessionID, redisJson.LocalAddr)
	}

	// 等待连接建立或超时
	select {
	case <-tun_active_chain:
		redisJson.State = 3
		log.Printf("会话 %s State 2 -> State 3: Local端被动连接成功", sessionID)
		RedisSessionSet(sessionID, redisJson.RedisTimeOut, redisJson)
		if tun_active != nil && tun_active.TunQuicConn != nil {
			return true, nil
		}
		return false, nil

	case <-tun_passive_chain:
		redisJson.State = 3
		log.Printf("会话 %s State 2 -> State 3: Local端主动连接成功", sessionID)
		RedisSessionSet(sessionID, redisJson.RedisTimeOut, redisJson)
		if tun_passive != nil && tun_passive.TunQuicConn != nil {
			return true, nil
		}
		return false, nil

	case <-time.After(time.Duration(config.Arg_p2p_timeout) * time.Second):
		redisJson.State = 4
		log.Printf("会话 %s State 2 -> State 4: Local端连接超时", sessionID)
		RedisSessionSet(sessionID, redisJson.RedisTimeOut, redisJson)
		return false, nil
	}
}

// handleRemoteState3_ConnectionSuccess 处理 Remote 端 State 3: 连接成功
func handleRemoteState3_ConnectionSuccess(sessionID string, tun_active *tun.TunActive, tun_passive *tun.TunPassive) {
	log.Printf("会话 %s State 3: 连接成功", sessionID)

	if tun_active != nil && tun_active.TunQuicConn != nil {
		// 连接成功，启动代理和健康检查
		handleConnection(sessionID, tun_active.TunQuicConn, tun_active.TunHealthStream)
	} else if tun_passive != nil && tun_passive.TunQuicConn != nil {
		// 连接成功，启动代理和健康检查
		handleConnection(sessionID, tun_passive.TunQuicConn, tun_passive.TunHealthStream)
	}
}

// handleRemoteState4_ConnectionTimeout 处理 Remote 端 State 4: 连接超时
func handleRemoteState4_ConnectionTimeout(sessionID string) {
	log.Printf("会话 %s State 4: 连接超时", sessionID)
}

// processSession 处理单个会话的完整生命周期
// 由主循环认领会话后启动，接收已认领的 SessionID 和 redisJson
func processSession(redisJson *RedisJsonType) {
	// 独立的资源，每个会话完全隔离
	var tun_active *tun.TunActive
	var tun_passive *tun.TunPassive
	var udp_conn *net.UDPConn

	conn_type := 0 // 主动连接

	var tun_active_chain chan quic.Connection
	var tun_passive_chain chan quic.Connection

	log.Printf("收到Local端请求: %v", redisJson)

	// 阶段1: 处理 State 0 -> State 1 - 认领会话并发送 Remote 端地址
	if err := handleState1_SendRemoteAddr(redisJson.SessionID, redisJson, &tun_active, &tun_passive, &udp_conn, &conn_type, &tun_active_chain, &tun_passive_chain); err != nil {
		log.Printf("会话 %s 处理 State 1 失败: %v", redisJson.SessionID, err)
		return
	}

	// 阶段2: 使用独立的session key进行后续通信
	for m_remote_stats == 1 {
		time.Sleep(1 * time.Second)

		// 读取会话状态
		if RedisSessionGet(redisJson.SessionID, redisJson) != nil {
			log.Printf("会话 %s 超时", redisJson.SessionID)
			break
		}

		redisJson.RemoteVersion = GetVersion()
		redisJson.SocketTimeOut = time.Duration(config.Arg_p2p_timeout) * time.Second
		redisJson.RedisTimeOut = redisJson.SocketTimeOut * 3

		// 根据状态进行处理
		switch redisJson.State {
		case 1:
			log.Printf("会话 %s State 1: 等待Local端状态, Local: %v => Remote: %v", redisJson.SessionID, redisJson.LocalAddr, redisJson.RemoteAddr)

		case 2:
			success, err := handleState2_WaitConnection(redisJson.SessionID, redisJson, conn_type, tun_active, tun_passive, tun_active_chain, tun_passive_chain)
			if err != nil {
				log.Printf("会话 %s 处理 State 2 失败: %v", redisJson.SessionID, err)
				break
			}
			if success {
				// 连接成功，进入 State 3
				go func() {
					handleRemoteState3_ConnectionSuccess(redisJson.SessionID, tun_active, tun_passive)
					Release(tun_active, tun_passive, udp_conn)
				}()
				return
			} else if redisJson.State == 4 {
				// 连接超时，进入 State 4
				handleRemoteState4_ConnectionTimeout(redisJson.SessionID)
				break
			}

		case 3:
			go func() {
				handleRemoteState3_ConnectionSuccess(redisJson.SessionID, tun_active, tun_passive)
				Release(tun_active, tun_passive, udp_conn)
			}()
			return

		case 4:
			handleRemoteState4_ConnectionTimeout(redisJson.SessionID)
			break

		default:
			log.Printf("会话 %s 等待Local端状态: State %d, Local: %v => Remote: %v", redisJson.SessionID, redisJson.State, redisJson.LocalAddr, redisJson.RemoteAddr)
		}
	}

	Release(tun_active, tun_passive, udp_conn)
}

// handleConnection 处理已建立的连接
func handleConnection(sessionID string, quicConn quic.Connection, healthStream quic.Stream) {
	log.Printf("开始处理连接: %s", sessionID)

	// 启动代理服务
	go proxy.ProcessProxyServer(quicConn)

	// 阻塞等待健康检查结束
	tun.ProcessHealth(healthStream)

	m_upnp_bind.RemoveKeepPort(quicConn.LocalAddr().(*net.UDPAddr).Port)

	log.Printf("释放连接: %v, SessionID: %s", quicConn.LocalAddr(), sessionID)
}

func StopRemote() error {
	m_remote_stats = 0
	// 清空正在处理的会话记录
	m_processing_sessions.Range(func(key, value any) bool {
		m_processing_sessions.Delete(key)
		return true
	})
	// 重置活跃连接数
	log.Println("[连接统计] Remote端已停止，所有连接已清理")
	return nil
}

func RunRemote(tun_key string) error {
	m_remote_stats = 1

	m_tun_key = tun_key
	m_md5_tun_key = go2.Md5Encode(tun_key)

	log.Printf("Remote端启动，等待Local端连接...")
	log.Printf("%s%s", TagStatusPrefix, TagStatusRunning)

	// 主循环扫描待处理的会话
	for m_remote_stats == 1 {
		// 每次只认领一个待处理的会话
		redisJson, err := RedisSessionClaim()
		if err != nil {
			time.Sleep(3 * time.Second)
			continue
		}
		processSession(redisJson)
	}

	log.Println("Remote端已停止")
	return nil
}
