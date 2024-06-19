package tools

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"log"
)

func AssertErrorToNilf(message string, err error) {
	if err != nil {
		log.Fatalf(message, err)
	}
}

func RandomString(length int) string {
	k := make([]byte, length)
	_, err := io.ReadFull(rand.Reader, k)
	if err != nil {
		panic(err.Error())
	}
	return base64.StdEncoding.EncodeToString(k)
}
