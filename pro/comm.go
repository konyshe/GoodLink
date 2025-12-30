package pro

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	go2aes "go2/aes"
	"goodlink/config"
	"goodlink/stun2"
	"goodlink/upnp"
	"goodlink/utils"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"net"
	"time"

	"github.com/go-redis/redis"
)

var (
	m_redis_db    *redis.Client
	m_tun_key     string
	m_md5_tun_key string
	m_version     string
	m_upnp_bind   upnp.Upnp
)

func SetVersion(v string) {
	m_version = v
}

func GetVersion() string {
	return m_version
}

func Init() error {
	utils.Log().Debug("初始化配置中")
	for {
		if err := config.Init(); err != nil {
			utils.Log().Debug(err.Error())
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	utils.Log().Debug("初始化配置完成")

	var redis_addr string
	var redis_pass string
	var redis_id int
	tlsConfig := &tls.Config{
		InsecureSkipVerify: true, // 如果使用自签名证书，设置为true跳过证书验证（仅用于测试
		//Certificates:       []tls.Certificate{cert},
		ClientAuth: tls.NoClientCert, // 验证模式：请求并验证客户端证书
	}

	if config.Arg_redis_addr == "" && config.Arg_redis_tls_addr == "" {
		redis_addr = config.GetAddr()
		redis_pass = config.GetPasswd()
		redis_id = config.GetID()

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

	if err := m_upnp_bind.CleanMappings(0); err != nil {
		utils.Log().ErrorF("UPnP清理失败: %v", err)
	}

	return nil
}

func Release(tun_active *tun.TunActive, tun_passive *tun.TunPassive) {
	utils.Log().SetDebugSate(0)

	if tun_active != nil {
		tun_active.Release()
	}
	if tun_passive != nil {
		tun_passive.Release()
	}
}

type RedisJsonType struct {
	LocalVersion  string        `bson:"local_version" json:"local_version"`
	RemoteVersion string        `bson:"remote_version" json:"remote_version"`
	SessionID     string        `bson:"session_id" json:"session_id"`
	State         int           `bson:"state" json:"state"`
	SocketTimeOut time.Duration `bson:"socket_time_out" json:"socket_time_out"`
	RedisTimeOut  time.Duration `bson:"redis_time_out" json:"redis_time_out"`
	SendPortCount int           `bson:"send_port_count" json:"send_port_count"`
	ConnectCount  int           `bson:"connect_count" json:"connect_count"`
	RemoteAddr    tun.AddrType  `bson:"remote_addr" json:"remote_addr"`
	LocalAddr     tun.AddrType  `bson:"local_addr" json:"local_addr"`
}

func RedisSet(time_out time.Duration, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis为初始化")
	}
	if jsonByte, err := json.Marshal(*redisJson); err == nil {
		m_redis_db.Set(m_md5_tun_key, go2aes.Encrypt7(jsonByte, m_tun_key), time_out)
	}
	return nil
}

func RedisGet(redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis为初始化")
	}

	aes_res, err := m_redis_db.Get(m_md5_tun_key).Bytes()
	if err != nil || aes_res == nil || len(aes_res) == 0 {
		return fmt.Errorf("获取信令数据失败: %v", err)
	}

	if err = json.Unmarshal(go2aes.Decrypt7(aes_res, m_tun_key), redisJson); err != nil {
		return fmt.Errorf("解析信令数据失败: %v", err)
	}

	return nil
}

func RedisDel() {
	m_redis_db.Del(m_md5_tun_key)
}

// 获取会话注册表的 Redis key
func getSessionsKey() string {
	return m_md5_tun_key + ":sessions"
}

// 获取单个会话的 Redis key
func getSessionKey(sessionID string) string {
	return m_md5_tun_key + ":" + sessionID
}

// RedisSessionRegister Local端注册新SessionID到Hash
// 返回注册的SessionID
func RedisSessionRegister(timeout time.Duration, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	jsonByte, err := json.Marshal(*redisJson)
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %v", err)
	}

	encryptedData := go2aes.Encrypt7(jsonByte, m_tun_key)

	// 使用 HSET 将会话注册到 Hash 中
	if err := m_redis_db.HSet(getSessionsKey(), redisJson.SessionID, encryptedData).Err(); err != nil {
		return fmt.Errorf("注册会话失败: %v", err)
	}

	// 设置 Hash 的过期时间
	m_redis_db.Expire(getSessionsKey(), timeout)

	return nil
}

// RedisSessionScan Remote端扫描待处理的SessionID列表
// 返回所有state=0的待处理会话
func RedisSessionScan() ([]RedisJsonType, error) {
	if m_redis_db == nil {
		return nil, errors.New("Redis未初始化")
	}

	// 获取Hash中所有会话
	result, err := m_redis_db.HGetAll(getSessionsKey()).Result()
	if err != nil {
		return nil, fmt.Errorf("扫描会话失败: %v", err)
	}

	var pendingSessions []RedisJsonType
	for sessionID, encryptedData := range result {
		var redisJson RedisJsonType
		decryptedData := go2aes.Decrypt7([]byte(encryptedData), m_tun_key)
		if err := json.Unmarshal(decryptedData, &redisJson); err != nil {
			utils.Log().DebugF("解析会话数据失败: %v", err)
			continue
		}

		// 确保SessionID一致
		if redisJson.SessionID != sessionID {
			redisJson.SessionID = sessionID
		}

		// 只返回state=0的待处理会话
		if redisJson.State == 0 {
			pendingSessions = append(pendingSessions, redisJson)
		}
	}

	return pendingSessions, nil
}

// RedisSessionClaim Remote端原子认领一个SessionID
// 使用 Lua 脚本保证原子性：获取数据并删除，确保只有一个 Worker 能认领成功
func RedisSessionClaim(sessionID string, redisJson *RedisJsonType, timeout time.Duration) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	// 使用 Lua 脚本原子地获取并删除会话
	// 只有成功获取到数据的 Worker 才能认领该会话
	luaScript := `
		local data = redis.call('HGET', KEYS[1], ARGV[1])
		if data then
			redis.call('HDEL', KEYS[1], ARGV[1])
			return data
		end
		return nil
	`

	result, err := m_redis_db.Eval(luaScript, []string{getSessionsKey()}, sessionID).Result()
	if err != nil {
		return fmt.Errorf("认领会话失败: %v", err)
	}

	if result == nil {
		return errors.New("会话不存在或已被其他Worker认领")
	}

	encryptedData, ok := result.(string)
	if !ok {
		return errors.New("会话数据格式错误")
	}

	decryptedData := go2aes.Decrypt7([]byte(encryptedData), m_tun_key)
	if err := json.Unmarshal(decryptedData, redisJson); err != nil {
		return fmt.Errorf("解析会话数据失败: %v", err)
	}

	// 检查是否是待处理状态
	if redisJson.State != 0 {
		return errors.New("会话状态异常")
	}

	return nil
}

// RedisSessionSet 基于SessionID的会话数据写入
func RedisSessionSet(sessionID string, timeout time.Duration, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	jsonByte, err := json.Marshal(*redisJson)
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %v", err)
	}

	encryptedData := go2aes.Encrypt7(jsonByte, m_tun_key)
	sessionKey := getSessionKey(sessionID)

	if err := m_redis_db.Set(sessionKey, encryptedData, timeout).Err(); err != nil {
		return fmt.Errorf("写入会话数据失败: %v", err)
	}

	return nil
}

// RedisSessionGet 基于SessionID的会话数据读取
func RedisSessionGet(sessionID string, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis未初始化")
	}

	sessionKey := getSessionKey(sessionID)
	encryptedData, err := m_redis_db.Get(sessionKey).Bytes()
	if err != nil || encryptedData == nil || len(encryptedData) == 0 {
		return fmt.Errorf("获取会话数据失败: %v", err)
	}

	decryptedData := go2aes.Decrypt7(encryptedData, m_tun_key)
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
	m_redis_db.Del(getSessionKey(sessionID))
}

// RedisSessionUnregister 从Hash中移除会话注册
func RedisSessionUnregister(sessionID string) {
	if m_redis_db == nil {
		return
	}
	m_redis_db.HDel(getSessionsKey(), sessionID)
}

func GetUDPLocalIPPort(level string) (string, int) {
	conn, err := net.Dial(level, "ifconfig.co:80")
	if err != nil {
		return "", 0
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.String(), conn.LocalAddr().(*net.UDPAddr).Port
}

func GetUDPAddr() (conn *net.UDPConn, addr tun.AddrType) {
	addr.LocalIPv4, _ = GetUDPLocalIPPort("udp4")
	addr.IPv6, _ = GetUDPLocalIPPort("udp6")

	var err error

	for {
		conn, err = net.ListenUDP("udp4", nil) // 只监听IPv4
		if err != nil {
			utils.Log().Debug(err.Error())
			time.Sleep(1 * time.Second)
			continue
		}

		addr.LocalPort = conn.LocalAddr().(*net.UDPAddr).Port
		addr.WanIPv4, addr.WanPort1, addr.WanPort2, addr.WanPort3 = stun2.GetStunIpPort(conn)
		conn.Close()

		conn, err = net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv6zero, Port: addr.LocalPort})
		if err != nil {
			utils.Log().Debug(err.Error())
			time.Sleep(1 * time.Second)
			continue
		}

		break
	}

	return
}
