package proxy

import (
	"log"

	"goodlink/socks5"
)

var (
	m_socks5_server *socks5.Server
)

func StopSocks5() {
	if m_socks5_server != nil {
		m_socks5_server.StopSocks5()
		m_socks5_server = nil
	}
}

func ListenSocks5(addr string) error {
	var err error

	// Create a SOCKS5 server
	m_socks5_server, err = socks5.New(&socks5.Config{})
	if err != nil {
		return err
	}

	log.Printf("   ListenSocks5: %v\n", addr)

	// Create SOCKS5 proxy on addr
	if err := m_socks5_server.ListenAndServe("tcp", addr); err != nil {
		return err
	}

	return nil
}
