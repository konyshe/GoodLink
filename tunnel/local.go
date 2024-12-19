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
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
)

type TunnelClient struct {
	stun_quic_conn     quic.Connection
	stun_health_stream quic.Stream
	process_lock       sync.Mutex
	redisdb            *redis.Client
	tun_key            string
	md5_tun_key        string
	SendData           []byte
	RecvData           []byte
	remote_addr        *net.UDPAddr
}

func (c *TunnelClient) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr, time_out time.Duration) {
	if c.stun_quic_conn != nil {
		return
	}

	conn.SetDeadline(time.Now().Add(time_out))

	log.Printf("   quic.Listen: %v\n", conn.LocalAddr())
	listener, err := quic.Listen(conn, tls2.GetServerTLSConfig(), nil)
	if err != nil {
		log.Printf("   process_quic quic.Listen: %v\n", err)
		return
	}

	log.Printf("   process_quic conn.WriteToUDP: %v ==> %v\n", conn.LocalAddr(), remoteAddr)
	conn.WriteToUDP(c.SendData, remoteAddr)
	_, err = conn.WriteToUDP(c.SendData, remoteAddr)
	if err != nil {
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
	if n, err := new_quic_stream.Read(c.RecvData); err == nil && n > 0 {
		conn.SetDeadline(time.Time{})
		log.Printf("   process_server5 quic local:%v remote:%v recv:%v... count:%v\n", new_quic_conn.LocalAddr(), remoteAddr, string(c.RecvData[:10]), n)
		c.stun_health_stream = new_quic_stream
		c.stun_quic_conn = new_quic_conn
	}
}

func (c *TunnelClient) process3(time_out time.Duration) {
	conn := tools.GetListenUDP()
	conn.SetDeadline(time.Now().Add(time_out))

	go func(conn2 *net.UDPConn) {
		if n, remoteAddr, err := conn2.ReadFromUDP(c.RecvData); err == nil && n > 0 {
			c.process_lock.Lock()
			defer c.process_lock.Unlock()

			log.Printf("   锁定连接 local:%v remote:%v recv:%v... count:%v\n", conn2.LocalAddr(), remoteAddr, string(c.RecvData[:10]), n)
			c.process_quic(conn2, remoteAddr, time_out)
		}
	}(conn)

	//log.Printf("   process_server5 conn.WriteToUDP: %v ==> %v\n", conn.LocalAddr(), c.remote_addr)

	conn.WriteToUDP(c.SendData, c.remote_addr)
	conn.WriteToUDP(c.SendData, c.remote_addr)
}

func (c *TunnelClient) process2(count int, time_out time.Duration) {
	for i := 0; i <= count; i++ {
		c.process3(time_out)
	}
}

func (c *TunnelClient) process1(count int) quic.Connection {
	var err error

	redisJson := RedisJsonType{
		ConnectCount: count,
	}

	log.Println("0: 通知对端连接")
	RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, 30*time.Second, &redisJson)

	wan_ip_chain := make(chan string, 1)
	wan_port_chain := make(chan int, 1)

	go func() {
		ClientIP, ClientPort := stun2.GetWanIpPort()
		wan_ip_chain <- ClientIP
		wan_port_chain <- ClientPort
	}()

	for {
		time.Sleep(1 * time.Second)

		err = RedisGet(c.redisdb, c.tun_key, c.md5_tun_key, &redisJson)
		if err != nil {
			log.Println(err)
			return nil
		}

		switch redisJson.State {
		case 1:
			log.Printf("%d: 收到对端地址: %v\n", redisJson.State, redisJson)

			redisJson.ClientIP, redisJson.ClientPort = <-wan_ip_chain, <-wan_port_chain
			c.remote_addr, _ = net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", redisJson.ServerIP, redisJson.ServerPort))

			c.process2(redisJson.SendPortCount, redisJson.SocketTimeOut)

			redisJson.State = 2
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)

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
}

func ProcessClient(tun_local_addr, redis_addr, redis_pass string, radis_id int, tun_key string, retry bool) error {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})
	if redisdb == nil {
		log.Println("Redis初始化失败")
		os.Exit(0)
	}
	defer redisdb.Close()

	listener, err := net.Listen("tcp", tun_local_addr)
	if listener == nil || err != nil {
		return fmt.Errorf("地址监听失败: %v", tun_local_addr)
	}
	defer listener.Close()

	count := 0

	for {
		tunnelClient := TunnelClient{
			redisdb:        redisdb,
			tun_key:        tun_key,
			md5_tun_key:    md5.Encode(tun_key),
			SendData:       []byte(tools.RandomString(3)),
			RecvData:       make([]byte, 128),
			stun_quic_conn: nil,
		}

		count++

		conn := tunnelClient.process1(count)
		if conn == nil {
			tunnelClient.Release()
			continue
		}
		redisdb.Del(tunnelClient.md5_tun_key)

		chain := make(chan int, 1)
		go func() {
			proxy.ProcessProxyClient(listener, conn)
			chain <- 1
		}()

		process_health(tunnelClient.stun_health_stream, tunnelClient.SendData, tunnelClient.RecvData)
		log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		tunnelClient.Release()

		if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
			conn.Write(tunnelClient.SendData)
			conn.Close() // 关闭连接
		}

		<-chain
		count = 0

		if !retry {
			return fmt.Errorf("   连接已断开")
		}
	}
}
