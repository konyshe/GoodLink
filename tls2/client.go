package tls2

import (
	"crypto/tls"
)

func GetClientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"goodlink"},
	}
}
