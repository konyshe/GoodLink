package tls2

import (
	"crypto/tls"
)

func GetClientTLSConfig() *tls.Config {
	initOnce.Do(initTLSConfigs)
	return clientConfig
}
