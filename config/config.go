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
	Redis RedisInfo `bson:"redis" json:"redis"`
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

func GetAddr() string {
	return configInfo.Redis.Addr
}

func GetPasswd() string {
	return configInfo.Redis.Passwd
}

func GetID() int {
	return configInfo.Redis.Id
}
