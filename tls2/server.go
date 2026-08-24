package tls2

import (
	"crypto/tls"
)

func GetServerTLSConfig() *tls.Config {
	initOnce.Do(initTLSConfigs)
	return serverConfig
}
