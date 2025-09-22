package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

func getKey(key string) []byte {
	if len(key) > 24 {
		return []byte(key[:24])
	}

	// 填充key
	for len(key) < 24 {
		key = key + key[0:1]
	}
	return []byte(key)
}

func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func Encrypt(origData []byte, key string) string {
	k := getKey(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return base64.StdEncoding.EncodeToString(cryted)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func Decrypt(cryted []byte, key string) []byte {
	crytedByte, _ := base64.StdEncoding.DecodeString(string(cryted))
	k := getKey(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)
	return PKCS7UnPadding(orig)
}

func AesTest() {
	text := []byte("hello world")

	key := "1234567812345677777777777781238"
	temp1 := Encrypt(text, key)
	fmt.Printf("加密后: %s\n", temp1)

	temp2 := Decrypt([]byte(temp1), key)
	fmt.Printf("解密后: %s\n", temp2)

	key = "123456781238"
	temp1 = Encrypt(text, key)
	fmt.Printf("加密后: %s\n", temp1)

	temp2 = Decrypt([]byte(temp1), key)
	fmt.Printf("解密后: %s\n", temp2)
}
