package tunnel

import (
	"context"
	"fmt"
	"goodlink/md5"
	"goodlink/proxy"
	"goodlink/stun2"
	_ "goodlink/stun2"
	"goodlink/tls2"
	"goodlink/tools"
	"log"
	"net"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
)

type TunnelClient struct {
	stun_quic_conn     quic.Connection
	stun_health_stream quic.Stream
	remote_addr        *net.UDPAddr
	conn_list          []*net.UDPConn
}

func (c *TunnelClient) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	if c.stun_quic_conn != nil {
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
		c.stun_health_stream = new_quic_stream
		c.stun_quic_conn = new_quic_conn
	}
}

func (c *TunnelClient) process3() {
	conn := tools.GetListenUDP()
	conn.SetDeadline(time.Time{})
	c.conn_list = append(c.conn_list, conn)

	go func(d *TunnelClient, conn2 *net.UDPConn) {
		if n, remoteAddr, err := conn2.ReadFromUDP(m_recv_data); err == nil && n > 0 {
			m_process_lock.Lock()
			defer m_process_lock.Unlock()

			log.Printf("   锁定连接 local:%v remote:%v recv:%v... count:%v\n", conn2.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
			d.process_quic(conn2, remoteAddr)
		}
	}(c, conn)

	//log.Printf("   process3 conn.WriteToUDP: %v ==> %v\n", conn.LocalAddr(), c.remote_addr)

	conn.WriteToUDP(m_send_data, c.remote_addr)
	conn.WriteToUDP(m_send_data, c.remote_addr)
}

func (c *TunnelClient) process2(count int) {
	for i := 0; i <= count; i++ {
		c.process3()
	}
}

func (c *TunnelClient) process1(count int) quic.Connection {
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

			redisJson.State = 2
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(redisJson.RedisTimeOut, &redisJson)

		case 3:
			if c.stun_quic_conn == nil {
				log.Println("   连接失败")
				return nil
			}
			log.Printf("%d: 连接成功\n", redisJson.State)
			return c.stun_quic_conn

		case 4:
			log.Printf("%d: 连接超时\n", redisJson.State)
			return nil

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}
	}
}

func (c *TunnelClient) GetQuicConn() quic.Connection {
	return c.stun_quic_conn
}

func (c *TunnelClient) Release() {
	log.Println("   清空缓存和连接")

	if c.stun_health_stream != nil {
		c.stun_health_stream.Close()
		c.stun_health_stream = nil
	}

	if c.stun_quic_conn != nil {
		c.stun_quic_conn.CloseWithError(0, "0")
		c.stun_quic_conn = nil
	}

	for _, conn := range c.conn_list {
		if conn != nil {
			conn.Close()
		}
	}
}

func ProcessClient(tun_local_addr, redis_addr, redis_pass string, radis_id int, tun_key string, retry bool) error {
	m_redisdb = redis.NewClient(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})
	if m_redisdb == nil {
		log.Println("Redis初始化失败")
		os.Exit(0)
	}
	defer m_redisdb.Close()

	listener, err := net.Listen("tcp", tun_local_addr)
	if listener == nil || err != nil {
		return fmt.Errorf("地址监听失败: %v", tun_local_addr)
	}
	defer listener.Close()

	m_tun_key = tun_key
	m_md5_tun_key = md5.Encode(m_tun_key)

	count := 0

	for {
		tunnelClient := TunnelClient{
			stun_quic_conn:     nil,
			stun_health_stream: nil,
			conn_list:          make([]*net.UDPConn, 0),
		}

		count++

		conn := tunnelClient.process1(count)
		if conn == nil {
			tunnelClient.Release()
			continue
		}
		m_redisdb.Del(m_md5_tun_key)

		chain := make(chan int, 1)
		go func() {
			proxy.ProcessProxyClient(listener, conn)
			chain <- 1
		}()

		process_health(tunnelClient.stun_health_stream)
		log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		tunnelClient.Release()

		if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
			conn.Write(m_send_data)
			conn.Close() // 关闭连接
		}

		<-chain
		count = 0

		if !retry {
			return fmt.Errorf("   连接已断开")
		}
	}
}
