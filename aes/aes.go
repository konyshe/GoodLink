package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"fmt"
)

func Encrypt(orig string, key string) string {
	origData := []byte(orig)
	k := []byte(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)
	blockMode := cipher.NewCBCEncrypter(block, k[:blockSize])
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)
	return base64.StdEncoding.EncodeToString(cryted)
}

func Decrypt(cryted string, key string) string {
	crytedByte, _ := base64.StdEncoding.DecodeString(cryted)
	k := []byte(key)
	block, _ := aes.NewCipher(k)
	blockSize := block.BlockSize()
	blockMode := cipher.NewCBCDecrypter(block, k[:blockSize])
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)
	orig = PKCS7UnPadding(orig)
	return string(orig)
}

func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func PKCS7UnPadding(origData []byte) []byte {
	length := len(origData)
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func EncodeTest() {
	orig := "hello world"
	key := "123456781234567812345678"
	fmt.Println("原文：", orig)

	encryptCode := Encrypt(orig, key)
	fmt.Println("密文：", encryptCode)

	decryptCode := Decrypt(encryptCode, key)
	fmt.Println("解密结果：", decryptCode)
}
