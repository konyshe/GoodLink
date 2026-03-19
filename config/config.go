package config

import (
	"encoding/json"
	"fmt"
	"go2"
	go2aes "go2/aes"
	go2http "go2/http"
	"log"
	"os"
)

type RedisInfo struct {
	Addr    string `bson:"addr" json:"addr"`
	TlsAddr string `bson:"tls_addr" json:"tls_addr"`
	Passwd  string `bson:"passwd" json:"passwd"`
	Id      int    `bson:"id" json:"id"`
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

const (
	ConfigFileName = "config.json"
)

var (
	configInfo ConfigInfo
)

func DeleteLocalConfig() {
	log.Println("删除本地配置")
	os.Remove(ConfigFileName)
}

func Init() error {
	var res []byte
	var err error

	if res = go2.FileReadAll(ConfigFileName); res == nil {
		DeleteLocalConfig()
		go2http.DownloadSimple(fmt.Sprintf("https://gitee.com/konyshe/goodlink_conf/raw/master/%s", ConfigFileName), ConfigFileName)
		res = go2.FileReadAll(ConfigFileName)
	}

	if err = json.Unmarshal(go2aes.Decrypt7(res, "goodlink"), &configInfo); err != nil {
		DeleteLocalConfig()
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

func GetStunList() []string {
	return GetConfig().StunList
}

func GetAddr() string {
	return GetConfig().Redis.TlsAddr
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
