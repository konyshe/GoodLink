package tunnel

import (
	"encoding/json"
	"fmt"
	"goodlink/aes"
	"goodlink/tools"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

var (
	m_send_data    []byte
	m_recv_data    []byte
	m_redisdb      *redis.Client
	m_tun_key      string
	m_md5_tun_key  string
	m_process_lock sync.Mutex
)

type RedisJsonType struct {
	State         int           `bson:"state" json:"state"`
	RedisTimeOut  time.Duration `bson:"redis_time_out" json:"redis_time_out"`
	SendPortCount int           `bson:"send_port_count" json:"send_port_count"`
	ConnectCount  int           `bson:"connect_count" json:"connect_count"`
	ServerIP      string        `bson:"server_ip" json:"server_ip"`
	ServerPort    int           `bson:"server_port" json:"server_port"`
	ClientIP      string        `bson:"client_ip" json:"client_ip"`
	ClientPort    int           `bson:"client_port" json:"client_port"`
}

func RedisSet(time_out time.Duration, redisJson *RedisJsonType) {
	if jsonByte, err := json.Marshal(*redisJson); err == nil {
		m_redisdb.Set(m_md5_tun_key, aes.Encrypt(jsonByte, m_tun_key), time_out)
	}
}

func RedisGet(redisJson *RedisJsonType) error {
	aes_res, err := m_redisdb.Get(m_md5_tun_key).Bytes()
	if err != nil || aes_res == nil || len(aes_res) == 0 {
		return fmt.Errorf("   获取信令数据失败: %v", err)
	}

	if err = json.Unmarshal(aes.Decrypt(aes_res, m_tun_key), redisJson); err != nil {
		return fmt.Errorf("   解析信令数据失败: %v", err)
	}

	return nil
}

func init() {
	m_send_data = []byte(tools.RandomString(3))
	m_recv_data = make([]byte, 128)
}
