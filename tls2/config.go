package tls2

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"math/big"
	"sync"
	"time"
)

var (
	initOnce     sync.Once
	clientConfig *tls.Config
	serverConfig *tls.Config
)

func initTLSConfigs() {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		panic(err)
	}

	tlsCert := tls.Certificate{
		Certificate: [][]byte{certDER},
		PrivateKey:  priv,
	}

	nextProtos := []string{"goodlink"}
	curves := []tls.CurveID{tls.X25519}

	serverConfig = &tls.Config{
		Certificates:     []tls.Certificate{tlsCert},
		NextProtos:       nextProtos,
		MinVersion:       tls.VersionTLS13,
		CurvePreferences: curves,
	}
	clientConfig = &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         nextProtos,
		MinVersion:         tls.VersionTLS13,
		CurvePreferences:   curves,
		ClientSessionCache: tls.NewLRUClientSessionCache(128),
	}
}
