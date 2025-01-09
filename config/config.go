package config

import (
	"encoding/json"
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
	Redis       RedisInfo `bson:"redis" json:"redis"`
	WorkType    string    `bson:"work_type" json:"work_type"`
	TunKey      string    `bson:"tun_key" json:"tun_key"`
	ConnType    string    `bson:"conn_type" json:"conn_type"`
	LocalIP     string    `bson:"local_ip" json:"local_ip"`
	LocalPort   string    `bson:"local_port" json:"local_port"`
	RemoteType  string    `bson:"remote_type" json:"remote_type"`
	RemoteIP    string    `bson:"remote_ip" json:"remote_ip"`
	RemotePort  string    `bson:"remote_port" json:"remote_port"`
	StunList    []string  `bson:"stun_list" json:"stun_list"`
	DingTalkUrl string    `bson:"ding_talk_url" json:"ding_talk_url"`
}

var configInfo ConfigInfo

func Init() error {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/config.json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var res []byte
	res, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	temp2 := aes.Decrypt(res, "goodlink")
	err = json.Unmarshal(temp2, &configInfo)
	if err != nil {
		return err
	}

	return nil
}

func GetConfig() ConfigInfo {
	if len(configInfo.StunList) == 0 {
		Init()
	}
	return configInfo
}

func GetAddr() string {
	return GetConfig().Redis.Addr
}

func GetPasswd() string {
	return GetConfig().Redis.Passwd
}

func GetID() int {
	return GetConfig().Redis.Id
}

func GetDingTalkUrl() string {
	return GetConfig().DingTalkUrl
}
