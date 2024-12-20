package tunnel

import (
	"context"
	"fmt"
	"goodlink/stun2"
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
}

func (c *TunPassive) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	c.TunState = 0
	log.Println("   标记停止发送报文")

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
	_, err1 := conn.WriteToUDP(m_send_data, remoteAddr)
	_, err2 := conn.WriteToUDP(m_send_data, remoteAddr)
	if err1 != nil && err2 != nil {
		log.Printf("   process_quic conn.WriteToUDP: %v\n", err)
		return
	}

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
	}
}

func (c *TunPassive) process_send_map() int {
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

func (c *TunPassive) process3() {
	conn := tools.GetListenUDP()

	c.ConnList = append(c.ConnList, conn) //这里不用加锁

	go func(d *TunPassive, conn2 *net.UDPConn) {
		if n, remoteAddr, err := conn2.ReadFromUDP(m_recv_data); err == nil && n > 0 {
			m_process_lock.Lock()
			defer m_process_lock.Unlock()

			log.Printf("   锁定连接 local:%v remote:%v recv:%v... count:%v\n", conn2.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
			d.process_quic(conn2, remoteAddr)
		}
	}(c, conn)
}

func (c *TunPassive) process2(count int) {
	for i := 0; i <= count; i++ {
		c.process3()
	}
}

func (c *TunPassive) GetQuicConn(count int) quic.Connection {
	var err error

	redisJson := RedisJsonType{
		ConnectCount: count,
	}

	log.Println("0: 通知对端连接")
	RedisSet(30*time.Second, &redisJson)

	wan_ip_chain := make(chan string, 1)
	wan_port_chain := make(chan int, 1)

	go func() {
		ClientIP, ClientPort := stun2.GetWanIpPort()
		wan_ip_chain <- ClientIP
		wan_port_chain <- ClientPort
	}()

	for {
		time.Sleep(1 * time.Second)

		err = RedisGet(&redisJson)
		if err != nil {
			log.Println(err)
			return nil
		}

		switch redisJson.State {
		case 1:
			log.Printf("%d: 收到对端地址: %v\n", redisJson.State, redisJson)

			redisJson.ClientIP, redisJson.ClientPort = <-wan_ip_chain, <-wan_port_chain
			c.remote_addr, _ = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", redisJson.ServerIP, redisJson.ServerPort))

			c.process2(redisJson.SendPortCount)
			c.process_send_map()
			go func(d *TunPassive) {
				for {
					time.Sleep(3 * time.Second)
					if d.process_send_map() < 0 {
						return
					}
				}
			}(c)

			redisJson.State = 2
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(redisJson.RedisTimeOut, &redisJson)

		case 3:
			if c.TunQuicConn == nil {
				log.Println("   连接失败")
				return nil
			}
			log.Printf("%d: 连接成功\n", redisJson.State)
			return c.TunQuicConn

		case 4:
			log.Printf("%d: 连接超时\n", redisJson.State)
			return nil

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}
	}
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
}
