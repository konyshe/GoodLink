package md5

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

func Encode(data string) string {
	h := md5.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func EncodeTest() {
	strTest := "I love this beautiful world!"
	strEncrypted := "98b4fc4538115c4980a8b859ff3d27e1"
	fmt.Println(Encode(strTest) == strEncrypted)
}
