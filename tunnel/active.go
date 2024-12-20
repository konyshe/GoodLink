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
	"golang.org/x/exp/rand"
)

type TunnelServer struct {
	TunQuicConn     quic.Connection
	TunHealthStream quic.Stream
	process_chain   chan quic.Connection
	RedisTimeOut    time.Duration
	SocketTimeOut   time.Duration
	Conn            *net.UDPConn
	ConnList        []*net.UDPConn
}

func (c *TunnelServer) process_send(conn2 *net.UDPConn, dst_ip string, dst_port int) {
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

func (c *TunnelServer) process_server4(remote_ip string) {
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

func (c *TunnelServer) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
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

func (c *TunnelServer) Release() {
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
}

func (c *TunnelServer) process3(conn2 *net.UDPConn, ip string, port int) {
	if port < 1024 || port > 65534 {
		return
	}

	for i := port - 16; i < port; i++ {
		c.process_send(conn2, ip, i)
	}
}

func (c *TunnelServer) SetReadFunc(conn2 *net.UDPConn) {
	go func(d *TunnelServer, conn3 *net.UDPConn) {
		n, remote_addr, err := conn3.ReadFromUDP(m_recv_data) // 接收数据
		if err == nil && n > 0 {
			log.Printf("   process_server1 udp local:%v remote:%v recv:%v... count:%v\n", conn3.LocalAddr(), remote_addr, string(m_recv_data[:10]), n)
			d.process_quic(c.Conn, remote_addr)
			return
		}
	}(c, conn2)
}

func (c *TunnelServer) ProcessServerChild(ip string, port int) {
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

func (c *TunnelServer) GetQuicConn() quic.Connection {
	c.process_chain = make(chan quic.Connection, 1)

	redisJson := RedisJsonType{}
	last_state := redisJson.State

	for {
		time.Sleep(1 * time.Second)

		for RedisGet(&redisJson) != nil {
			time.Sleep(3 * time.Second)
		}

		if redisJson.State < last_state {
			log.Println("   对端已重置连接")
			return nil
		}

		if redisJson.State-last_state > 1 {
			log.Println("   状态异常")
			M_redis_db.Del(M_md5_tun_key)
			return nil
		}

		switch redisJson.State {
		case 0:
			log.Printf("%d: 收到对端请求\n", redisJson.State)
			c.Conn = tools.GetListenUDP()
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(c.Conn)

			redisJson.RedisTimeOut = c.RedisTimeOut
			redisJson.State = 1
			redisJson.SendPortCount = 0x100 //0x400
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(redisJson.RedisTimeOut, &redisJson)

		case 2:
			log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)

			c.ProcessServerChild(redisJson.ClientIP, redisJson.ClientPort)

			select {
			case <-c.process_chain:
				redisJson.State = 3
				log.Printf("%d: 通知对端, 连接成功\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return c.TunQuicConn

			case <-time.After(c.SocketTimeOut):
				redisJson.State = 4
				log.Printf("%d: 通知对端, 连接超时\n", redisJson.State)
				RedisSet(redisJson.RedisTimeOut, &redisJson)
				return nil
			}

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}

		last_state = redisJson.State
	}
}
