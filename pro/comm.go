package pro

import (
	"encoding/json"
	"fmt"
	"goodlink/aes"
	"goodlink/config"
	"goodlink/md5"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"log"
	"os"
	"time"

	"github.com/go-redis/redis"
)

var (
	m_redis_db    *redis.Client
	m_tun_key     string
	m_md5_tun_key string
	m_tun_active  *tun.TunActive
	m_tun_passive *tun.TunPassive
)

func Init(m_cli_redis_addr, m_cli_redis_pass string, m_cli_redis_id int, tun_key string) {
	if m_cli_redis_addr == "" {
		config.Init()
		m_cli_redis_addr = config.GetAddr()
		m_cli_redis_pass = config.GetPasswd()
		m_cli_redis_id = config.GetID()
	}

	m_redis_db = redis.NewClient(&redis.Options{
		Addr:     m_cli_redis_addr,
		Password: m_cli_redis_pass,
		DB:       m_cli_redis_id,
	})
	if m_redis_db == nil {
		log.Println("Redis初始化失败")
		os.Exit(0)
	}
	m_tun_active = nil
	m_tun_passive = nil

	m_tun_key = tun_key
	m_md5_tun_key = md5.Encode(tun_key)
}

func Release() {
	if m_tun_active != nil {
		m_tun_active.Release()
		m_tun_active = nil
	}
	if m_tun_passive != nil {
		m_tun_passive.Release()
		m_tun_passive = nil
	}
}

type RedisJsonType struct {
	State         int           `bson:"state" json:"state"`
	SocketTimeOut time.Duration `bson:"SocketTimeOut" json:"SocketTimeOut"`
	RedisTimeOut  time.Duration `bson:"RedisTimeOut" json:"RedisTimeOut"`
	SendPortCount int           `bson:"send_port_count" json:"send_port_count"`
	ConnectCount  int           `bson:"connect_count" json:"connect_count"`
	ServerIP      string        `bson:"server_ip" json:"server_ip"`
	ServerPort    int           `bson:"server_port" json:"server_port"`
	ClientIP      string        `bson:"client_ip" json:"client_ip"`
	ClientPort    int           `bson:"client_port" json:"client_port"`
}

func RedisSet(time_out time.Duration, redisJson *RedisJsonType) {
	if jsonByte, err := json.Marshal(*redisJson); err == nil {
		m_redis_db.Set(m_md5_tun_key, aes.Encrypt(jsonByte, m_tun_key), time_out)
	}
}

func RedisGet(redisJson *RedisJsonType) error {
	aes_res, err := m_redis_db.Get(m_md5_tun_key).Bytes()
	if err != nil || aes_res == nil || len(aes_res) == 0 {
		return fmt.Errorf("   获取信令数据失败: %v", err)
	}

	if err = json.Unmarshal(aes.Decrypt(aes_res, m_tun_key), redisJson); err != nil {
		return fmt.Errorf("   解析信令数据失败: %v", err)
	}

	return nil
}
