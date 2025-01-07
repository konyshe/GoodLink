package config

import (
	"encoding/json"
	"errors"
	"gogo"
	"goodlink/aes"
	"io"
	"net/http"
	"time"
)

type RedisInfo struct {
	Addr   string `bson:"addr" json:"addr"`
	Passwd string `bson:"passwd" json:"passwd"`
	Id     int    `bson:"id" json:"id"`
}

type ConfigInfo struct {
	Redis      RedisInfo `bson:"redis" json:"redis"`
	WorkType   string    `bson:"work_type" json:"work_type"`
	TunKey     string    `bson:"tun_key" json:"tun_key"`
	ConnType   string    `bson:"conn_type" json:"conn_type"`
	LocalIP    string    `bson:"local_ip" json:"local_ip"`
	LocalPort  string    `bson:"local_port" json:"local_port"`
	RemoteType string    `bson:"remote_type" json:"remote_type"`
	RemoteIP   string    `bson:"remote_ip" json:"remote_ip"`
	RemotePort string    `bson:"remote_port" json:"remote_port"`
	StunList   []string  `bson:"stun_list" json:"stun_list"`
}

var configInfo ConfigInfo

func GetConfig() ConfigInfo {
	return configInfo
}

func Init() error {
	gogo.Log().Debug("   初始化配置中")
	defer gogo.Log().Debug("   初始化配置完成")

	client := &http.Client{Timeout: 3 * time.Second}
	var resp *http.Response
	var err error
	var res []byte

	for {
		resp, err = client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/config.json")
		if resp != nil && err == nil {
			res, err = io.ReadAll(resp.Body)
			resp.Body.Close()
			if res != nil && err == nil {
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	temp2 := aes.Decrypt(res, "goodlink")
	err = json.Unmarshal(temp2, &configInfo)
	if err != nil {
		return errors.New("解析配置失败, 请重启程序")
	}

	return nil
}

func GetAddr() string {
	return configInfo.Redis.Addr
}

func GetPasswd() string {
	return configInfo.Redis.Passwd
}

func GetID() int {
	return configInfo.Redis.Id
}
