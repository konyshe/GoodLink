package pro

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"go2"
	go2aes "go2/aes"
	"goodlink/config"
	"goodlink/stun2"
	"goodlink/tun"
	"goodlink/upnp"
	goodlink_config "goodlink3/config"
	"log"
	"net"
	"time"

	"github.com/go-redis/redis"
)

var (
	m_redis_db    *redis.Client
	m_tun_key     string
	m_md5_tun_key string
	m_upnp_bind   upnp.Upnp
	m_svc         config.UIService
)

func Init(svc config.UIService) error {
	var redis_addr string
	var redis_pass string
	var redis_id int

	m_svc = svc
	m_tun_key = svc.TunKey
	m_md5_tun_key = go2.Md5Encode(svc.TunKey)

	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // 如果使用自签名证书，设置为true跳过证书验证（仅用于测试
		//Certificates:       []tls.Certificate{cert},
		ClientAuth: tls.NoClientCert, // 验证模式：请求并验证客户端证书
	}

	if config.Arg_redis_addr == "" && config.Arg_redis_tls_addr == "" {
		if rc, ok := config.LoadRuntimeConfFromEnv(); ok {
			redis_addr = rc.Redis.Addr
			redis_pass = rc.Redis.Passwd
			redis_id = rc.Redis.ID
			if rc.Redis.TlsAddr != "" {
				redis_addr = rc.Redis.TlsAddr
			} else {
				tlsConfig = nil
			}
		} else {
			redis_addr = goodlink_config.GetAddr()
			redis_pass = goodlink_config.GetPasswd()
			redis_id = goodlink_config.GetID()
			if goodlink_config.GetTlsAddr() != "" {
				redis_addr = goodlink_config.GetTlsAddr()
			} else {
				redis_addr = goodlink_config.GetAddr()
				tlsConfig = nil
			}
		}

	} else if config.Arg_redis_tls_addr != "" {
		redis_addr = config.Arg_redis_tls_addr
		redis_pass = config.Arg_redis_pass
		redis_id = config.Arg_redis_id

	} else {
		redis_addr = config.Arg_redis_addr
		redis_pass = config.Arg_redis_pass
		redis_id = config.Arg_redis_id
		tlsConfig = nil
	}

	m_redis_db = redis.NewClient(&redis.Options{
		Addr:         redis_addr,
		Password:     redis_pass,
		DB:           redis_id,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		TLSConfig:    tlsConfig,
	})
	if m_redis_db == nil {
		return errors.New("Redis失败, 请重启程序")
	}

	m_upnp_bind.Description = "goodlink-" + svc.ID
	m_upnp_bind.Init()
	m_upnp_bind.CleanMappings()

	return nil
}

func Release(tun_active *tun.TunActive, tun_passive *tun.TunPassive, udp_conn *net.UDPConn) {
	if tun_active != nil {
		tun_active.Release()
	}
	if tun_passive != nil {
		tun_passive.Release()
	}
	if udp_conn != nil {
		udp_conn.Close()
	}
}

type RedisJsonType struct {
	LocalVersion    string        `bson:"local_version" json:"local_version"`
	RemoteVersion   string        `bson:"remote_version" json:"remote_version"`
	SessionID       string        `bson:"session_id" json:"session_id"`
	State           int           `bson:"state" json:"state"`
	Transport       string        `bson:"transport" json:"transport"`
	SocketTimeOut   time.Duration `bson:"socket_time_out" json:"socket_time_out"`
	RedisTimeOut    time.Duration `bson:"redis_time_out" json:"redis_time_out"`
	SendPortCount   int           `bson:"send_port_count" json:"send_port_count"`
	ConnectCount    int           `bson:"connect_count" json:"connect_count"`
	RemoteAddr      tun.AddrType  `bson:"remote_addr" json:"remote_addr"`
	LocalAddr       tun.AddrType  `bson:"local_addr" json:"local_addr"`
	AllowLocalProxy *bool         `bson:"allow_local_proxy" json:"allow_local_proxy,omitempty"`
}

// LocalProxyAllowed 字段缺省（旧 Remote）时视为允许。
func (r RedisJsonType) LocalProxyAllowed() bool {
	return r.AllowLocalProxy == nil || *r.AllowLocalProxy
}

func sessionTransport(s string) string {
	t := config.NormalizeTransport(s)
	if t != config.TransportQUIC && t != config.TransportKCP {
		return config.TransportKCP
	}
	return t
}

// RedisSessionRegister Local端注册新SessionID到Hash
func RedisSessionRegister(timeout time.Duration, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	for m_redis_db.Exists(m_md5_tun_key).Val() > 0 {
		log.Printf("remote端上一个会话未完成，等待30秒后重试...")
		time.Sleep(30 * time.Second)
	}

	jsonByte, err := json.Marshal(*redisJson)
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %v", err)
	}

	encryptedData := go2aes.Encrypt7(jsonByte, m_tun_key)

	// 使用 HSET 将会话注册到 Hash 中
	if err := m_redis_db.Set(m_md5_tun_key, encryptedData, timeout).Err(); err != nil {
		return fmt.Errorf("注册会话失败: %v", err)
	}

	return nil
}

// RedisSessionScan Remote端扫描待处理的SessionID列表
func RedisSessionClaim() (*RedisJsonType, error) {
	if m_redis_db == nil {
		return nil, errors.New("Redis未初始化")
	}

	// 获取Hash中所有会话
	encryptedData, err := m_redis_db.Get(m_md5_tun_key).Result()
	if err != nil {
		return nil, fmt.Errorf("扫描会话失败: %v", err)
	}

	var redisJson RedisJsonType
	decryptedData := go2aes.Decrypt7([]byte(encryptedData), m_tun_key)
	if err := json.Unmarshal(decryptedData, &redisJson); err != nil {
		return nil, fmt.Errorf("解析会话数据失败: %v", err)
	}

	// 认领后从redis中删除，防止重复认领
	m_redis_db.Del(m_md5_tun_key)

	return &redisJson, nil
}

// RedisSessionSet 基于SessionID的会话数据写入
// 使用SessionID作为密钥加密，Remote端认领后使用此函数
func RedisSessionSet(sessionID string, timeout time.Duration, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	jsonByte, err := json.Marshal(*redisJson)
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %v", err)
	}

	// 使用SessionID作为密钥加密（Remote端认领后，后续交互都使用SessionID作为密钥）
	encryptedData := go2aes.Encrypt7(jsonByte, sessionID)

	if err := m_redis_db.Set(go2.Md5Encode(sessionID), encryptedData, timeout).Err(); err != nil {
		return fmt.Errorf("写入会话数据失败: %v", err)
	}

	return nil
}

// RedisSessionGet 基于SessionID的会话数据读取
// 使用SessionID作为密钥解密，Remote端认领后使用此函数
func RedisSessionGet(sessionID string, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	encryptedData, err := m_redis_db.Get(go2.Md5Encode(sessionID)).Bytes()
	if err != nil || encryptedData == nil || len(encryptedData) == 0 {
		return fmt.Errorf("获取会话数据失败: %v", err)
	}

	// 使用SessionID作为密钥解密
	decryptedData := go2aes.Decrypt7(encryptedData, sessionID)
	if err := json.Unmarshal(decryptedData, redisJson); err != nil {
		return fmt.Errorf("解析会话数据失败: %v", err)
	}

	return nil
}

// RedisSessionDel 基于SessionID的会话数据删除
func RedisSessionDel(sessionID string) {
	if m_redis_db == nil {
		return
	}
	m_redis_db.Del(go2.Md5Encode(sessionID))
}

// RedisSessionUnregister 移除 Local 端尚未被认领的注册键(m_md5_tun_key)。
// sessionID 仅用于调用方语义对齐; 注册键与认领后的 session 键不同。
func RedisSessionUnregister(sessionID string) {
	if m_redis_db == nil {
		return
	}
	_ = sessionID
	m_redis_db.Del(m_md5_tun_key)
}

// externalIPProbeTimeout 探测本机出口地址时的连接超时:
// net.Dial 本身没有超时, 外部服务不可达时会阻塞到系统级超时
const externalIPProbeTimeout = 3 * time.Second

// maxGetUDPAddrAttempts 是获取本地 UDP 地址的重试次数上限。
// 原来是无限重试, 一旦环境异常就会永久卡住, 且无法被停止流程打断。
const maxGetUDPAddrAttempts = 5

func GetUDPLocalIPPort(level string) (string, int) {
	dialer := &net.Dialer{Timeout: externalIPProbeTimeout}
	conn, err := dialer.Dial(level, "ifconfig.co:80")
	if err != nil {
		return "", 0
	}
	defer conn.Close()

	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", 0
	}
	return udpAddr.IP.String(), udpAddr.Port
}

func GetUDPAddr() (conn *net.UDPConn, addr tun.AddrType, err error) {
	addr.LocalIPv4, _ = GetUDPLocalIPPort("udp4")
	addr.IPv6, _ = GetUDPLocalIPPort("udp6")

	for attempt := 1; attempt <= maxGetUDPAddrAttempts; attempt++ {
		conn, err = net.ListenUDP("udp4", nil) // 只监听IPv4
		if err != nil {
			log.Println(err.Error())
			conn = nil
			if attempt < maxGetUDPAddrAttempts {
				time.Sleep(1 * time.Second)
			}
			continue
		}

		addr.LocalPort = conn.LocalAddr().(*net.UDPAddr).Port
		addr.WanIPv4, addr.WanPort1, addr.WanPort2, addr.WanPort3 = stun2.GetStunIpPort(conn)

		// STUN 失败时端口为 0, 此时得到的地址信息不能用于连接:
		// WanPort1==WanPort2 会被判定为 NAT1-3 并走注定失败的主动连接
		if addr.WanIPv4 == "" || addr.WanPort1 == 0 || addr.WanPort2 == 0 {
			log.Println("STUN 探测失败, 重新获取本地UDP地址后重试")
			conn.Close()
			conn = nil
			addr.LocalPort = 0 // 下一轮重新使用随机本地端口, 避免绑定上一轮残留端口
			if attempt < maxGetUDPAddrAttempts {
				time.Sleep(1 * time.Second)
			}
			continue
		}
		conn.Close()

		conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6zero, Port: addr.LocalPort})
		if err != nil {
			log.Println(err.Error())
			conn = nil
			if attempt < maxGetUDPAddrAttempts {
				time.Sleep(1 * time.Second)
			}
			continue
		}

		return conn, addr, nil
	}

	return nil, tun.AddrType{}, fmt.Errorf("获取本地UDP地址失败, 已重试%d次: %v", maxGetUDPAddrAttempts, err)
}
