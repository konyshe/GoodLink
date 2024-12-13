package proxy

import (
	"log"

	"github.com/armon/go-socks5"
)

func ListenSocks5(addr string) {
	// Create a SOCKS5 server
	conf := &socks5.Config{}
	server, err := socks5.New(conf)
	if err != nil {
		panic(err)
	}

	log.Printf("ListenSocks5: %v\n", addr)

	// Create SOCKS5 proxy on addr
	if err := server.ListenAndServe("tcp", addr); err != nil {
		panic(err)
	}
}
