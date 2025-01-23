package pro

import (
	"encoding/json"
	"errors"
	"fmt"
	"goodlink/aes"
	"goodlink/config"
	"goodlink/utils"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"time"

	"github.com/go-redis/redis"
)

var (
	m_redis_db    *redis.Client
	m_tun_key     string
	m_md5_tun_key string
)

func Init(m_cli_redis_addr, m_cli_redis_pass string, m_cli_redis_id int) error {
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

	if m_cli_redis_addr == "" {
		m_cli_redis_addr = config.GetAddr()
		m_cli_redis_pass = config.GetPasswd()
		m_cli_redis_id = config.GetID()
	}

	m_redis_db = redis.NewClient(&redis.Options{
		Addr:     m_cli_redis_addr,
		Password: m_cli_redis_pass,
		DB:       m_cli_redis_id,
		//MaxRetries:   99,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	})
	if m_redis_db == nil {
		return errors.New("Redis失败, 请重启程序")
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
	SessionID       string        `bson:"session_id" json:"session_id"`
	State           int           `bson:"state" json:"state"`
	SocketTimeOut   time.Duration `bson:"SocketTimeOut" json:"SocketTimeOut"`
	RedisTimeOut    time.Duration `bson:"RedisTimeOut" json:"RedisTimeOut"`
	SendPortCount   int           `bson:"send_port_count" json:"send_port_count"`
	ConnectCount    int           `bson:"connect_count" json:"connect_count"`
	RemoteWanIPv4   string        `bson:"remote_ip" json:"remote_wan_ip_v4"`     // Remote端, 外网IPv4地址
	RemoteLocalIPv4 string        `bson:"remote_ip" json:"remote_local_ip_v4"`   // Remote端, 本地IPv4地址
	RemoteIPv6      string        `bson:"remote_ip" json:"remote_ip_v6"`         // Remote端, IPv6地址
	RemoteLocalPort int           `bson:"remote_port0" json:"remote_local_port"` // Remote端, 本地端口
	RemoteWanPort1  int           `bson:"remote_port1" json:"remote_wan_port1"`  // Remote端, 外网端口1
	RemoteWanPort2  int           `bson:"remote_port2" json:"remote_wan_port2"`  // Remote端, 外网端口2
	LocalWanIPv4    string        `bson:"local_ip" json:"local_wan_ip_v4"`       // Remote端, 外网IPv4地址
	LocalLocalIPv4  string        `bson:"local_ip" json:"local_local_ip_v4"`     // Local端, 本地IPv4地址
	LocalIPv6       string        `bson:"local_ip" json:"local_ip_v6"`           // Local端, IPv6地址
	LocalLocalPort  int           `bson:"local_port0" json:"local_local_port"`   // Local端, 本地端口
	LocalWanPort1   int           `bson:"local_port1" json:"local_wan_port1"`    // Local端, 外网端口1
	LocalWanPort2   int           `bson:"local_port2" json:"local_wan_port2"`    // Local端, 外网端口2
}

func RedisSet(time_out time.Duration, redisJson *RedisJsonType) error {
	if m_redis_db == nil {
		return errors.New("Redis为初始化")
	}
	if jsonByte, err := json.Marshal(*redisJson); err == nil {
		m_redis_db.Set(m_md5_tun_key, aes.Encrypt(jsonByte, m_tun_key), time_out)
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

	if err = json.Unmarshal(aes.Decrypt(aes_res, m_tun_key), redisJson); err != nil {
		return fmt.Errorf("解析信令数据失败: %v", err)
	}

	return nil
}

func RedisDel() {
	m_redis_db.Del(m_md5_tun_key)
}
