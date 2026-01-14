package pro

import (
	"errors"
	"fmt"
	"go2"
	"goodlink/config"
	"goodlink/netstack"
	"goodlink/tun"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

var (
	m_local_state      = 0 //0: 停止, 1: 启动, 2: 连接成功
	m_tun_active       *tun.TunActive
	m_tun_passive      *tun.TunPassive
	g_netstack_started = false
)

// handleState0_RegisterSession 处理 State 0: 注册会话并等待 Remote 端认领
func handleState0_RegisterSession(sessionID string, redisJson *RedisJsonType, conn_type int, addr *tun.AddrType) (bool, error) {
	log.Printf("State 0: 注册会话 %s", sessionID)

	// 根据连接类型设置初始信息
	switch conn_type {
	case 0:
		log.Println("请求连接Remote端")
		log.Printf("%s%s", TagStatusPrefix, TagStatusConnecting)
	default:
		redisJson.LocalAddr = *addr
		log.Printf("发送Local端地址: %v", redisJson.LocalAddr)
		log.Printf("%s%s", TagStatusPrefix, TagStatusConnecting)
	}

	// 将SessionID注册到Hash中，等待Remote端认领
	if err := RedisSessionRegister(30*time.Second, redisJson); err != nil {
		log.Printf("注册会话失败: %v", err)
		return false, err
	}
	log.Printf("已注册会话到队列，等待Remote端认领: %s", sessionID)

	// 等待Remote端认领并写入独立的session key
	sessionClaimed := false
	for i := 0; i < 30 && m_local_state == 1; i++ {
		time.Sleep(1 * time.Second)

		// 尝试从独立的session key读取，如果能读到说明已被认领
		if RedisSessionGet(sessionID, redisJson) == nil {
			sessionClaimed = true
			log.Printf("State 0 -> State 1: 会话已被Remote端认领: %s", sessionID)
			break
		}
	}

	if !sessionClaimed {
		// 超时未被认领，从Hash中移除注册
		RedisSessionUnregister(sessionID)
		log.Println("等待Remote端认领超时")
		return false, nil
	}

	return true, nil
}

// handleState1_ProcessRemoteAddr 处理 State 1: 处理 Remote 端地址，创建 TUN 连接
func handleState1_ProcessRemoteAddr(sessionID string, redisJson *RedisJsonType, conn *net.UDPConn, addr *tun.AddrType, conn_type int, tun_active **tun.TunActive, tun_passive **tun.TunPassive) error {
	log.Printf("State 1: 收到Remote端地址: %v", redisJson.RemoteAddr)

	// 版本兼容性检查
	if redisJson.RemoteVersion != GetVersion() {
		log.Printf("两端版本不兼容: Local: %s => Remote: %s", GetVersion(), redisJson.RemoteVersion)
		log.Printf("%s%s", TagStatusPrefix, TagStatusVersionMismatch)
		RedisSessionDel(sessionID)
		return errors.New("两端版本不兼容")
	}

	if (addr.WanPort1 != addr.WanPort2) && (redisJson.RemoteAddr.WanPort1 != redisJson.RemoteAddr.WanPort2) {
		log.Println("*** local端和remote端都是NAT4 ***")
		log.Printf("%s%s", TagStatusPrefix, TagStatusConnectingNAT4)
	}

	// 根据连接类型创建 TUN 连接
	switch conn_type {
	case 0:
		// 被动连接：Local 端等待 Remote 端连接
		if *tun_passive != nil {
			(*tun_passive).Release()
		}
		*tun_active = nil

		redisJson.LocalAddr = *addr

		*tun_passive = tun.CreateTunPassive([]byte(redisJson.SessionID), conn, &redisJson.LocalAddr, &redisJson.RemoteAddr, redisJson.SendPortCount, time.Duration(config.Arg_conn_passive_send_time)*time.Millisecond, &m_upnp_bind)
		(*tun_passive).Start()

		redisJson.State = 2
		log.Printf("State 1 -> State 2: 发送Local端地址: %v", redisJson.LocalAddr)
		RedisSessionSet(sessionID, redisJson.RedisTimeOut, redisJson)

	default:
		// 主动连接：Local 端主动连接 Remote 端
		if *tun_active != nil {
			(*tun_active).Release()
		}
		*tun_passive = nil

		*tun_active = tun.CreateTunActive([]byte(redisJson.SessionID), conn, &redisJson.LocalAddr, &redisJson.RemoteAddr, time.Duration(config.Arg_conn_active_send_time)*time.Millisecond, &m_upnp_bind)
		(*tun_active).Start()

		redisJson.State = 2
		log.Printf("State 1 -> State 2: 发送Local端地址: %v", redisJson.LocalAddr)
		RedisSessionSet(sessionID, redisJson.RedisTimeOut, redisJson)
	}

	return nil
}

// handleLocalState3_ConnectionSuccess 处理 Local 端 State 3: 连接成功
func handleLocalState3_ConnectionSuccess(tun_active *tun.TunActive, tun_passive *tun.TunPassive) (*quic.Conn, *quic.Stream, bool) {
	log.Printf("State 3: 连接成功")

	if tun_passive != nil && tun_passive.TunQuicConn != nil {
		log.Printf("%s%s", TagStatusPrefix, TagStatusConnected)
		return tun_passive.TunQuicConn, tun_passive.TunHealthStream, true
	}
	if tun_active != nil && tun_active.TunQuicConn != nil {
		log.Printf("%s%s", TagStatusPrefix, TagStatusConnected)
		return tun_active.TunQuicConn, tun_active.TunHealthStream, true
	}

	log.Println("连接失败: TUN连接已建立但QUIC连接为空")
	return nil, nil, false
}

// handleLocalState4_ConnectionTimeout 处理 Local 端 State 4: 连接超时
func handleLocalState4_ConnectionTimeout() {
	log.Printf("State 4: 连接超时")
}

func GetLocalQuicConn(conn *net.UDPConn, addr *tun.AddrType, count int) (*tun.TunActive, *tun.TunPassive, *quic.Conn, *quic.Stream, error) {
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
		log.Printf("WanPort %d:%d, 当前是NAT1-NAT3, 主动连接", addr.WanPort1, addr.WanPort2)
		conn_type = 1 // 主动连接
	} else {
		log.Printf("WanPort %d:%d, 当前是NAT4, 被动连接", addr.WanPort1, addr.WanPort2)
	}

	// 阶段1: 处理 State 0 - 注册会话并等待认领
	sessionClaimed, err := handleState0_RegisterSession(SessionID, &redisJson, conn_type, addr)
	if err != nil {
		return tun_active, tun_passive, nil, nil, err
	}
	if !sessionClaimed {
		return tun_active, tun_passive, nil, nil, nil
	}

	// 阶段2: 使用独立的session key进行后续通信
	for m_local_state == 1 {
		time.Sleep(1 * time.Second)

		// 读取会话状态
		if RedisSessionGet(SessionID, &redisJson) != nil {
			return tun_active, tun_passive, nil, nil, fmt.Errorf("会话超时")
		}

		// 根据状态进行处理
		switch redisJson.State {
		case -1: // Remote端检测到版本不一致
			log.Printf("%s%s", TagStatusPrefix, TagStatusVersionMismatch)
			RedisSessionDel(SessionID)
			return tun_active, tun_passive, nil, nil, fmt.Errorf("Remote端检测到版本不兼容: Local: %s => Remote: %s", GetVersion(), redisJson.RemoteVersion)

		case 1:
			if err := handleState1_ProcessRemoteAddr(SessionID, &redisJson, conn, addr, conn_type, &tun_active, &tun_passive); err != nil {
				return tun_active, tun_passive, nil, nil, err
			}

		case 2:
			log.Printf("State 2: 等待连接建立, Local: %v => Remote: %v", redisJson.LocalAddr, redisJson.RemoteAddr)

		case 3:
			quicConn, healthStream, success := handleLocalState3_ConnectionSuccess(tun_active, tun_passive)
			if success {
				return tun_active, tun_passive, quicConn, healthStream, nil
			}
			return tun_active, tun_passive, nil, nil, nil

		case 4:
			handleLocalState4_ConnectionTimeout()
			return tun_active, tun_passive, nil, nil, nil

		default:
			log.Printf("等待Remote端状态: State %d, Local: %v => Remote: %v", redisJson.State, redisJson.LocalAddr, redisJson.RemoteAddr)
		}
	}

	return tun_active, tun_passive, nil, nil, nil
}

func GetLocalStats() int {
	return m_local_state
}

func StopLocal() error {
	m_local_state = 0
	Release(m_tun_active, m_tun_passive, nil)
	return nil
}

func RunLocal() error {
	m_local_state = 1

	count := 0

	var udp_conn *net.UDPConn
	var addr tun.AddrType

	for m_local_state == 1 {

		if udp_conn != nil {
			udp_conn.Close()
		}
		udp_conn, addr = GetUDPAddr()

		log.Printf("Local端地址: %v", addr)
		log.Printf("%s%s", TagStatusPrefix, TagStatusConnecting)

		if !g_netstack_started {
			if err := netstack.Start(); err != nil {
				return err
			}
			g_netstack_started = true
		}

		count++

		tun_active, tun_passive, quic_conn, health, err := GetLocalQuicConn(udp_conn, &addr, count)
		if err != nil {
			Release(tun_active, tun_passive, nil)
			return err
		}
		if quic_conn == nil {
			Release(tun_active, tun_passive, nil)
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
			log.Printf("%s%s", TagStatusPrefix, TagStatusConnecting)
		}
		log.Printf("释放连接: %v", quic_conn.LocalAddr())
		Release(tun_active, tun_passive, nil)

		netstack.SetForWarder(nil)
		count = 0
	}

	return nil
}
