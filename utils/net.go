package utils

import (
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func GetUDPLocalIPPort(level string) (string, int) {
	conn, err := net.Dial(level, "ifconfig.co:80")
	if err != nil {
		return "", 0
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr).IP.String(), conn.LocalAddr().(*net.UDPAddr).Port
}

func GetTCPWanIPv4Port() (string, int) {
	var res []byte
	var err error
	var resp *http.Response

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // 跳过证书验证
			},
		},
		Timeout: 3 * time.Second,
	}
	if resp, err = client.Get("https://goodlink.kony.vip/ip"); err != nil {
		return "", 0
	}
	defer resp.Body.Close()

	if res, err = io.ReadAll(resp.Body); err != nil {
		return "", 0
	}

	localAddr := string(res)

	ip := strings.Split(localAddr, ":")[0]
	port, _ := strconv.Atoi(strings.Split(localAddr, ":")[1])
	return ip, port
}
