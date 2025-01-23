package utils

import (
	"crypto/rand"
	"math/big"
)

func RandomBytes(length int) []byte {
	bytes := make([]byte, length)
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := 0; i < length; {
		bint, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return nil
		}
		bytes[i] = charset[bint.Int64()]
		i++
	}
	return bytes
}
