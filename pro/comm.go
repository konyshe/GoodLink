package pro

import (
	"encoding/json"
	"errors"
	"fmt"
	"goodlink/aes"
	"goodlink/config"
	"goodlink2/tun"
	_ "goodlink2/tun"
	"log"
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

func Init(m_cli_redis_addr, m_cli_redis_pass string, m_cli_redis_id int) error {
	if err := config.Init(); err != nil {
		return err
	}

	if m_cli_redis_addr == "" {
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
		return errors.New("Redis失败, 请重启程序")
	}
	if _, err := m_redis_db.Ping().Result(); err != nil { //心跳
		return errors.New("请检查网络故障, 退出重启")
	}

	m_tun_active = nil
	m_tun_passive = nil

	return nil
}

func Release() {
	log.Println("   全局释放资源开始")
	defer log.Println("   全局释放资源结束")

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

func RedisDel() {
	m_redis_db.Del(m_md5_tun_key)
}
