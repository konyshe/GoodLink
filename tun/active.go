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

func (c *TunActive) process_send(conn2 *net.UDPConn, dst_ip string, dst_port int) error {
	if conn2 == nil || dst_ip == "" || dst_port <= 0 || dst_port >= 0xFFFF {
		return fmt.Errorf("   发包异常: %v:%v", dst_ip, dst_port)
	}

	if c.State != 1 {
		return fmt.Errorf("   发包结束: %v:%v", dst_ip, dst_port)
	}

	remoteAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dst_ip, dst_port))

	conn2.WriteToUDP(m_send_data, remoteAddr)

	return nil
}

func (c *TunActive) process_server4(remote_ip string) error {
	for i := 1; i <= 8; i++ {
		for remote_port_map := make(map[int]bool); len(remote_port_map) <= 0x80; {
			if remote_port := rand.Intn(0x2004 * i); remote_port > 0x2004*(i-1) && remote_port <= 0x2004*i && remote_port > 0 && remote_port < 0xFFFF {
				if _, ok := remote_port_map[remote_port]; !ok {
					if err := c.process_send(c.Conn, remote_ip, remote_port); err != nil {
						return err
					}
					remote_port_map[remote_port] = true
				}
			}
		}
	}
	return nil
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

func (c *TunActive) Start(ip string, port int) {
	log.Printf("   发起主动连接: %v:%v\n", ip, port)

	for i := port; i < port+8; i += 2 {
		conn2 := tools.GetListenUDP()
		c.ConnList = append(c.ConnList, conn2)
		c.SetReadFunc(conn2)
		c.process3(conn2, ip, i)
	}

	c.SetReadFunc(c.Conn)

	for i := -32; i <= 64; i += 1 {
		c.process_send(c.Conn, ip, port+i)
	}

	go func() {
		for {
			//log.Printf("   发包开始: %v:%v\n", ip, port)
			c.process_send(c.Conn, ip, port)

			if err := c.process_server4(ip); err != nil {
				//log.Println(err)
				return
			}

			//log.Printf("   发包结束: %v:%v\n", ip, port)
			time.Sleep(time.Second)
		}
	}()
}

func (c *TunActive) GetChain() chan quic.Connection {
	return c.process_chain
}
