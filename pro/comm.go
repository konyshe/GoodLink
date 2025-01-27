package pro

import (
	"encoding/json"
	"errors"
	"fmt"
	"goodlink/aes"
	"goodlink/config"
	"goodlink/stun2"
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

func GetUDPAddr() (conn *net.UDPConn, addr tun.AddrType) {
	addr.LocalIPv4, _ = utils.GetUDPLocalIPPort("udp4")
	addr.IPv6, _ = utils.GetUDPLocalIPPort("udp6")

	var err error

	for {
		conn, err = net.ListenUDP("udp4", nil) // 只监听IPv4
		if err != nil {
			utils.Log().Debug(err.Error())
			time.Sleep(1 * time.Second)
			continue
		}

		addr.LocalPort = conn.LocalAddr().(*net.UDPAddr).Port
		addr.WanIPv4, addr.WanPort1, addr.WanPort2 = stun2.GetWanIpPort2(conn)
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
