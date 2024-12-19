package tools

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"net"
)

func RandomString(length int) string {
	k := make([]byte, length)
	_, err := io.ReadFull(rand.Reader, k)
	if err != nil {
		panic(err.Error())
	}
	return base64.StdEncoding.EncodeToString(k)
}

// 检测未使用的端口
func GetFreeLocalAddr() string {
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return ""
	}
	defer listener.Close() // 确保在函数退出时关闭监听器

	return listener.Addr().String()
}
