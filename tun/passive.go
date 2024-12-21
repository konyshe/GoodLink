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
)

type TunPassive struct {
	TunQuicConn     quic.Connection
	TunHealthStream quic.Stream
	remote_addr     *net.UDPAddr
	ConnList        []*net.UDPConn
	TunState        int
	process_chain   chan quic.Connection
}

func CteateTunPassive(conn *net.UDPConn, ip string, port int, count int) *TunPassive {
	c := &TunPassive{
		TunQuicConn:     nil,
		TunHealthStream: nil,
		TunState:        1,
		ConnList:        make([]*net.UDPConn, 0),
		process_chain:   make(chan quic.Connection, 1),
	}

	log.Printf("   发起被动连接: %v:%v\n", ip, port)

	c.SetReadFunc(conn)
	c.ConnList = append(c.ConnList, conn)

	for i := 0; i < count; i++ {
		conn2 := tools.GetListenUDP()
		c.SetReadFunc(conn2)
		c.ConnList = append(c.ConnList, conn2) //这里不用加锁
	}

	c.remote_addr, _ = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ip, port))

	return c
}

func (c *TunPassive) Release() {
	log.Println("   清空缓存和连接")

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

	if c.process_chain != nil {
		close(c.process_chain)
		c.process_chain = nil
	}
}

func (c *TunPassive) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	c.TunState = 0
	log.Println("   请求停止发包")

	if c.TunQuicConn != nil {
		return
	}

	log.Printf("   quic.Listen: %v\n", conn.LocalAddr())
	listener, err := quic.Listen(conn, tls2.GetServerTLSConfig(), nil)
	if err != nil {
		log.Printf("   process_quic quic.Listen: %v\n", err)
		return
	}

	log.Printf("   process_quic conn.WriteToUDP: %v ==> %v\n", conn.LocalAddr(), remoteAddr)
	conn.WriteToUDP(m_send_data, remoteAddr)

	log.Printf("   process_server5 listener.Accept: %v\n", conn.LocalAddr())
	new_quic_conn, err := listener.Accept(context.Background())
	if err != nil {
		log.Printf("   process_server5 listener.Accept: %v", err)
		return
	}

	log.Printf("   process_server5 quic.AcceptStream: %v ==> %v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	new_quic_stream, err := new_quic_conn.AcceptStream(context.Background())
	if err != nil {
		log.Printf("   process_server5 new_quic_conn.AcceptStream: %v", err)
		return
	}

	log.Printf("   process_quic new_quic_stream.Read: %v ==> %v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	if n, err := new_quic_stream.Read(m_recv_data); err == nil && n > 0 {
		log.Printf("   process_server5 quic local:%v remote:%v recv:%v... count:%v\n", new_quic_conn.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
		c.TunHealthStream = new_quic_stream
		c.TunQuicConn = new_quic_conn
		c.process_chain <- new_quic_conn
	}
}

func (c *TunPassive) Send() int {
	count := 0

	log.Printf("   发包开始(0): %v\n", c.remote_addr)

	for _, conn := range c.ConnList {
		if c.TunState == 1 && conn != nil && c.TunQuicConn == nil {
			_, err1 := conn.WriteToUDP(m_send_data, c.remote_addr)
			_, err2 := conn.WriteToUDP(m_send_data, c.remote_addr)
			if err1 == nil && err2 == nil {
				count += 1
				continue
			}
		}
		log.Printf("   发包异常(%d): %v\n", count, c.remote_addr)
		return -1
	}
	log.Printf("   发包结束(%d): %v\n", count, c.remote_addr)
	return 0
}

func (c *TunPassive) Start() {
	c.Send()
	go func(d *TunPassive) {
		for {
			time.Sleep(3 * time.Second)
			if d.Send() < 0 {
				return
			}
		}
	}(c)
}

func (c *TunPassive) SetReadFunc(conn *net.UDPConn) {
	go func(d *TunPassive, conn2 *net.UDPConn) {
		if n, remoteAddr, err := conn2.ReadFromUDP(m_recv_data); err == nil && n > 0 {
			m_process_lock.Lock()
			defer m_process_lock.Unlock()

			log.Printf("   锁定连接 local:%v remote:%v recv:%v... count:%v\n", conn2.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
			d.process_quic(conn2, remoteAddr)
		}
	}(c, conn)
}

func (c *TunPassive) GetChain() chan quic.Connection {
	return c.process_chain
}
