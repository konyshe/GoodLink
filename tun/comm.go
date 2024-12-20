package tun

import (
	"goodlink/tools"
	"sync"
)

var (
	m_send_data    []byte
	m_recv_data    []byte
	m_process_lock sync.Mutex
)

func init() {
	m_send_data = []byte(tools.RandomString(3))
	m_recv_data = make([]byte, 128)
}
