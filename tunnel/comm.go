package tunnel

import (
	"encoding/json"
	"goodlink/aes"
	"time"

	"github.com/go-redis/redis"
)

type RedisJsonType struct {
	State      int    `bson:"state" json:"state"`
	ServerIP   string `bson:"server_ip" json:"server_ip"`
	ServerPort int    `bson:"server_port" json:"server_port"`
	ClientIP   string `bson:"client_ip" json:"client_ip"`
	ClientPort int    `bson:"client_port" json:"client_port"`
}

func RedisSet(redisdb *redis.Client, tun_key, md5_tun_key string, time_out time.Duration, redisJson RedisJsonType) {
	if jsonByte, err := json.Marshal(redisJson); err == nil {
		redisdb.Set(md5_tun_key, aes.Encrypt(jsonByte, tun_key), time_out)
	}
}
