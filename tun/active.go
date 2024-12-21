package tun

import (
	"context"
	"fmt"
	_ "goodlink/stun2"
	"goodlink/tls2"
	"goodlink/tools"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
	"golang.org/x/exp/rand"
)

type TunActive struct {
	TunQuicConn     quic.Connection
	TunHealthStream quic.Stream
	process_chain   chan quic.Connection
	RedisTimeOut    time.Duration
	Conn            *net.UDPConn
	ConnList        []*net.UDPConn
}

func CreateTunActive(conn *net.UDPConn, time_out time.Duration) *TunActive {
	return &TunActive{
		RedisTimeOut:    time_out,
		TunQuicConn:     nil,
		TunHealthStream: nil,
		Conn:            conn,
		ConnList:        make([]*net.UDPConn, 0),
		process_chain:   make(chan quic.Connection, 1),
	}
}

func (c *TunActive) Release() {
	log.Println("   清空缓存和连接")

	if c.TunHealthStream != nil {
		c.TunHealthStream.Close()
		c.TunHealthStream = nil
	}

	if c.TunQuicConn != nil {
		c.TunQuicConn.CloseWithError(0, "0")
		c.TunQuicConn = nil
	}

	if c.Conn != nil {
		c.Conn.Close()
		c.Conn = nil
	}

	for _, conn := range c.ConnList {
		if conn != nil {
			conn.Close()
		}
	}

	if c.process_chain != nil {
		close(c.process_chain)
		c.process_chain = nil
	}
}

func (c *TunActive) process_send(conn2 *net.UDPConn, dst_ip string, dst_port int) {
	if conn2 == nil || dst_ip == "" || dst_port <= 0 || dst_port >= 0xFFFF {
		return
	}

	remoteAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dst_ip, dst_port))

	m_process_lock.Lock()
	defer m_process_lock.Unlock()

	if c.TunQuicConn == nil {
		conn2.WriteToUDP(m_send_data, remoteAddr)
		conn2.WriteToUDP(m_send_data, remoteAddr)
	}
}

func (c *TunActive) process_server4(remote_ip string) {
	for i := 1; i <= 8; i++ {
		for remote_port_map := make(map[int]bool); len(remote_port_map) <= 0x80; {
			if remote_port := rand.Intn(0x2004 * i); remote_port > 0x2004*(i-1) && remote_port <= 0x2004*i && remote_port > 0 && remote_port < 0xFFFF {
				if _, ok := remote_port_map[remote_port]; !ok {
					//log.Printf("rand port: %d\n", tun_remote_port)
					remote_port_map[remote_port] = true
					c.process_send(c.Conn, remote_ip, remote_port)
				}
			}
		}
	}
}

func (c *TunActive) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	m_process_lock.Lock()
	defer m_process_lock.Unlock()

	if c.TunQuicConn != nil {
		return
	}

	time.Sleep(1000 * time.Millisecond)

	log.Printf("   process_quic quic.Dial: %v ==> %v\n", conn.LocalAddr(), remoteAddr)
	new_quic_conn, err := quic.Dial(context.Background(), conn, remoteAddr, tls2.GetClientTLSConfig(), nil)
	if err != nil {
		log.Printf("   process_quic quic.Dial: %v\n", err)
		return
	}

	log.Printf("   process_quic new_quic_conn.OpenStreamSync: %v ==> %v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	new_quic_stream, err := new_quic_conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("   process_quic quic_conn.OpenStreamSync: %v\n", err)
		return
	}

	log.Printf("   process_quic new_quic_stream.Write: %v ==> %v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	if n, err := new_quic_stream.Write(m_send_data); n > 0 && err == nil {
		c.TunQuicConn = new_quic_conn
		c.TunHealthStream = new_quic_stream
		c.process_chain <- new_quic_conn
	}
}

func (c *TunActive) process3(conn2 *net.UDPConn, ip string, port int) {
	if port < 1024 || port > 65534 {
		return
	}

	for i := port - 16; i < port; i++ {
		c.process_send(conn2, ip, i)
	}
}

func (c *TunActive) SetReadFunc(conn2 *net.UDPConn) {
	go func(d *TunActive, conn3 *net.UDPConn) {
		n, remote_addr, err := conn3.ReadFromUDP(m_recv_data) // 接收数据
		if err == nil && n > 0 {
			log.Printf("   process_server1 udp local:%v remote:%v recv:%v... count:%v\n", conn3.LocalAddr(), remote_addr, string(m_recv_data[:10]), n)
			d.process_quic(c.Conn, remote_addr)
			return
		}
	}(c, conn2)
}

func (c *TunActive) Start(ip string, port int) {
	log.Printf("   发起主动连接: %v:%v\n", ip, port)

	for i := port; i < port+8; i += 2 {
		conn2 := tools.GetListenUDP()
		c.ConnList = append(c.ConnList, conn2)
		c.SetReadFunc(conn2)
		c.process3(conn2, ip, i)
	}

	c.SetReadFunc(c.Conn)
	for i := -32; i <= 64 && c.TunQuicConn == nil; i += 1 {
		c.process_send(c.Conn, ip, port+i)
	}
	if c.TunQuicConn == nil {
		c.process_server4(ip)
	}
}

func (c *TunActive) GetChain() chan quic.Connection {
	return c.process_chain
}
