package tun

import (
	"context"
	"fmt"
	_ "goodlink/stun2"
	"goodlink/tls2"
	"goodlink/tools"
	"log"
	"math/big"
	"net"
	"time"

	"crypto/rand"

	"github.com/quic-go/quic-go"
)

type TunActive struct {
	TunQuicConn     quic.Connection
	TunHealthStream quic.Stream
	process_chain   chan quic.Connection
	RedisTimeOut    time.Duration
	Conn            *net.UDPConn
	ConnList        []*net.UDPConn
	port_map        map[int]bool
	State           int
}

func CreateTunActive(conn *net.UDPConn, time_out time.Duration) *TunActive {
	return &TunActive{
		RedisTimeOut:    time_out,
		TunQuicConn:     nil,
		TunHealthStream: nil,
		State:           1,
		Conn:            conn,
		ConnList:        make([]*net.UDPConn, 0),
		port_map:        make(map[int]bool),
		process_chain:   make(chan quic.Connection, 1),
	}
}

func (c *TunActive) Release() {
	log.Println("   清空缓存和连接")

	if c.process_chain != nil {
		close(c.process_chain)
		c.process_chain = nil
	}

	if c.TunHealthStream != nil {
		c.TunHealthStream.Close()
		c.TunHealthStream = nil
	}

	if c.TunQuicConn != nil {
		c.TunQuicConn.CloseWithError(0, "0")
		c.TunQuicConn = nil
	}

	for _, conn := range c.ConnList {
		if conn != nil {
			conn.Close()
		}
	}

	/*
		if c.Conn != nil {
			c.Conn.Close()
			c.Conn = nil
		}
	*/
}

func (c *TunActive) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	c.State = 0
	//log.Println("   请求停止发包")

	m_process_lock.Lock()
	defer m_process_lock.Unlock()

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
		conn.SetDeadline(time.Time{})
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
		c.send(conn2, ip, i)
	}
}

func (c *TunActive) SetReadFunc(conn *net.UDPConn) {
	go func(d *TunActive, conn2 *net.UDPConn) {
		n, remote_addr, err := conn2.ReadFromUDP(m_recv_data) // 接收数据
		if err == nil && n > 0 {
			log.Printf("   process_server1 udp local:%v remote:%v recv:%v... count:%v\n", conn2.LocalAddr(), remote_addr, string(m_recv_data[:10]), n)
			d.process_quic(c.Conn, remote_addr)
			return
		}
	}(c, conn)
}

func (c *TunActive) send(conn2 *net.UDPConn, dst_ip string, dst_port int) int {
	if conn2 == nil || dst_ip == "" || dst_port <= 0 || dst_port >= 0xFFFF {
		return 0
	}

	if c.State != 1 {
		return -1
	}

	if _, ok := c.port_map[dst_port]; ok {
		return 0
	}
	c.port_map[dst_port] = true

	remoteAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dst_ip, dst_port))

	if n, err := conn2.WriteToUDP(m_send_data, remoteAddr); n == 0 || err != nil {
		return -1
	}

	return 1
}

func (c *TunActive) process_server4(ip string) int {
	var r *big.Int
	var po int
	var n int

	for i1 := 0; i1 < 0x40; i1++ {
		for i2 := 0; i2 < 0x10; {
			r, _ = rand.Int(rand.Reader, big.NewInt(0x400))
			po = 0x400*i1 + int(r.Int64())
			n = c.send(c.Conn, ip, po)
			switch n {
			case 0:
				continue
			case 1:
				i2++
				continue
			default:
				return -1
			}
		}
	}
	return 1
}

func (c *TunActive) Start(ip string, port int, time_out time.Duration) {
	log.Printf("   发起主动连接: %v:%v\n", ip, port)

	for i := port; i < port+8; i += 2 {
		conn2 := tools.GetListenUDP()
		c.ConnList = append(c.ConnList, conn2)
		c.SetReadFunc(conn2)
		c.process3(conn2, ip, i)
		c.Conn.SetDeadline(time.Now().Add(3000 * time.Millisecond))
	}

	c.SetReadFunc(c.Conn)
	for i := -32; i <= 64; i += 1 {
		c.send(c.Conn, ip, port+i)
	}
	c.Conn.SetDeadline(time.Now().Add(time_out))

	go func() {
		for {
			c.send(c.Conn, ip, port)

			if c.process_server4(ip) < 0 {
				return
			}

			time.Sleep(2 * time.Second)
		}
	}()
}

func (c *TunActive) GetChain() chan quic.Connection {
	return c.process_chain
}
