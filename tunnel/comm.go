package tunnel

import (
	"encoding/json"
	"fmt"
	"goodlink/aes"
	"time"

	"github.com/go-redis/redis"
)

type RedisJsonType struct {
	State         int           `bson:"state" json:"state"`
	SocketTimeOut time.Duration `bson:"time_out" json:"time_out"`
	RedisTimeOut  time.Duration `bson:"redis_time_out" json:"redis_time_out"`
	SendPortCount int           `bson:"send_port_count" json:"send_port_count"`
	ConnectCount  int           `bson:"connect_count" json:"connect_count"`
	ServerIP      string        `bson:"server_ip" json:"server_ip"`
	ServerPort    int           `bson:"server_port" json:"server_port"`
	ClientIP      string        `bson:"client_ip" json:"client_ip"`
	ClientPort    int           `bson:"client_port" json:"client_port"`
}

func RedisSet(redisdb *redis.Client, tun_key, md5_tun_key string, time_out time.Duration, redisJson *RedisJsonType) {
	if jsonByte, err := json.Marshal(*redisJson); err == nil {
		redisdb.Set(md5_tun_key, aes.Encrypt(jsonByte, tun_key), time_out)
	}
}

func RedisGet(redisdb *redis.Client, tun_key, md5_tun_key string, redisJson *RedisJsonType) error {
	aes_res, err := redisdb.Get(md5_tun_key).Bytes()
	if err != nil || aes_res == nil || len(aes_res) == 0 {
		return fmt.Errorf("   获取信令数据失败: %v", err)
	}

	if err = json.Unmarshal(aes.Decrypt(aes_res, tun_key), redisJson); err != nil {
		return fmt.Errorf("   解析信令数据失败: %v", err)
	}

	return nil
}
