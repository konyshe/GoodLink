package config

import (
	"crypto/tls"
	"encoding/json"
	"go2"
	"goodlink/aes"
	"io"
	"net/http"
	"time"
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

var configInfo ConfigInfo

func Init() error {
	var res []byte
	var err error
	var resp *http.Response

	if res = go2.Utils().FileReadAll("config.json"); res == nil {
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // 跳过证书验证
				},
			},
			Timeout: 3 * time.Second,
		}
		if resp, err = client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/config.json"); err != nil {
			return err
		}
		defer resp.Body.Close()

		if res, err = io.ReadAll(resp.Body); err != nil {
			return err
		}
	}

	if err = json.Unmarshal(aes.Decrypt(res, "goodlink"), &configInfo); err != nil {
		return err
	}

	/*
		var StunList []string
		StunList = append(StunList, "stun.easyvoip.com:3478")
		configInfo.StunList = StunList

		configInfo.Redis.TlsAddr = "goodlink.kony.vip:16378"

		body, _ := json.Marshal(configInfo)
		temp3 := aes.Encrypt(body, "goodlink")
		go2.Utils().FileDel("config.json")
		go2.Utils().FileAppend("config.json", []byte(temp3))
	*/
	return nil
}

func GetConfig() ConfigInfo {
	if len(configInfo.StunList) == 0 {
		Init()
	}
	return configInfo
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
