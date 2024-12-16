package config

import (
	"encoding/json"
	"goodlink/aes"
	"io"
	"log"
	"net/http"
	"os"
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

func Init() {
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("https://gitee.com/konyshe/goodlink_conf/raw/master/config.json")
	if resp == nil || err != nil {
		log.Fatalln(err)
		os.Exit(0)
	}
	defer resp.Body.Close()

	var res []byte
	res, err = io.ReadAll(resp.Body)
	if res == nil || err != nil {
		log.Fatalln(err)
		os.Exit(0)
	}

	temp2 := aes.Decrypt(res, "goodlink")
	err = json.Unmarshal(temp2, &configInfo)
	if err != nil {
		log.Fatalln(err)
		os.Exit(0)
	}
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
