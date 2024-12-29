package tools

import (
	"crypto/rand"
	"math/big"
	"net"
)

func RandomString(length int) string {
	bytes := make([]byte, length)
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < length; {
		bint, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return ""
		}
		bytes[i] = charset[bint.Int64()]
		i++
	}
	return string(bytes)
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
