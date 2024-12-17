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
	stun_quic_conn       quic.Connection
	stun_health_stream   quic.Stream
	process_lock         sync.Mutex
	process_chain        chan quic.Connection
	redisdb              *redis.Client
	tun_key              string
	md5_tun_key          string
	redis_time_out       time.Duration
	wait_socket_time_out time.Duration
	SendData             []byte
	RecvData             []byte
}

func (c *TunnelClient) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	c.process_lock.Lock()
	defer c.process_lock.Unlock()

	if c.stun_quic_conn != nil {
		conn.Close()
		return
	}

	log.Printf("quic.Listen: %v\n", conn.LocalAddr())
	listener, err := quic.Listen(conn, tls2.GetServerTLSConfig(), nil)
	tools.AssertErrorToNilf("process_quic quic.Listen: %v", err)

	log.Printf("process_quic conn.WriteToUDP: %v==>%v\n", conn.LocalAddr(), remoteAddr)
	if _, err := conn.WriteToUDP(c.SendData, remoteAddr); err != nil {
		log.Printf("process_quic conn.WriteToUDP: %v\n", err)
		return
	}

	log.Printf("process_server5 listener.Accept: %v\n", conn.LocalAddr())
	new_quic_conn, err := listener.Accept(context.Background())
	tools.AssertErrorToNilf("process_server5 listener.Accept: %v", err)

	log.Printf("process_server5 quic.AcceptStream: %v==>%v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	new_quic_stream, err := new_quic_conn.AcceptStream(context.Background())
	tools.AssertErrorToNilf("process_server5 new_quic_conn.AcceptStream: %v", err)

	log.Printf("process_quic new_quic_stream.Read: %v==>%v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	if n, err := new_quic_stream.Read(c.RecvData); err == nil && n > 0 {
		conn.SetReadDeadline(time.Now().Add(c.wait_socket_time_out))
		log.Printf("process_server5 quic local:%v remote:%v recv:%v... count:%v\n", new_quic_conn.LocalAddr(), remoteAddr, string(c.RecvData[:10]), n)
		c.stun_health_stream = new_quic_stream
		c.stun_quic_conn = new_quic_conn
		c.process_chain <- new_quic_conn
	}
}

func (c *TunnelClient) process_send(ip string, port int) {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		log.Printf("process_server2 net.ListenUDP: %v\n", err)
		return
	}

	go func() {
		conn.SetReadDeadline(time.Now().Add(c.wait_socket_time_out))
		if n, remoteAddr, _ := conn.ReadFromUDP(c.RecvData); n > 0 {
			conn.SetReadDeadline(time.Now().Add(c.wait_socket_time_out))
			log.Printf("process_send local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), remoteAddr, string(c.RecvData[:10]), n)
			c.process_quic(conn, remoteAddr)
			return
		}
		conn.Close()
	}()

	remoteAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		log.Printf("process_send net.ResolveUDPAddr: %v\n", err)
		return
	}

	//log.Printf("process_send: %v => %v\n", conn.LocalAddr(), remoteAddr)

	conn.WriteToUDP(c.SendData, remoteAddr)
}

func (c *TunnelClient) process1() quic.Connection {
	var conn *net.UDPConn

	c.process_chain = make(chan quic.Connection, 1)

	redisJson := RedisJsonType{}

	log.Println("0: 请求建立连接")
	RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, c.redis_time_out, &redisJson)

	for {
		time.Sleep(1 * time.Second)

		err := RedisGet(c.redisdb, c.tun_key, c.md5_tun_key, &redisJson)
		if err != nil {
			log.Println(err)
			return nil
		}

		switch redisJson.State {
		case 0:
			log.Println("0: 等待对端响应")

		case 1:
			log.Printf("1: 收到对端地址: %v\n", redisJson)
			if conn, err = net.ListenUDP("udp4", nil); err != nil {
				log.Printf("net.ListenUDP: %v\n", err)
				return nil
			}
			redisJson.ClientIP, redisJson.ClientPort = stun2.GetWanIpPort(conn)
			conn.Close()

			for i := 0; i <= 1024; i++ {
				c.process_send(redisJson.ServerIP, redisJson.ServerPort)
			}

			log.Printf("2: 发送本端地址: %v\n", redisJson)
			redisJson.State = 2
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, c.redis_time_out, &redisJson)

		case 2:
			log.Println("2: 等待对端响应")

		case 3:
			log.Println("3: 等待对端连接")
			select {
			case <-c.process_chain:
				log.Println("连接成功!")
				return c.stun_quic_conn
			case <-time.After(c.wait_socket_time_out):
				log.Println("连接超时!")
				return nil
			}

		default:
			log.Printf("发现异常状态: %d\n", redisJson.State)
			return nil
		}
	}
}

func (c *TunnelClient) GetQuicConn() quic.Connection {
	return c.stun_quic_conn
}

func (c *TunnelClient) Release() {
	c.process_lock.Lock()
	defer c.process_lock.Unlock()

	c.process_chain = nil

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

	tunnelClient := TunnelClient{
		redisdb:              redisdb,
		tun_key:              tun_key,
		md5_tun_key:          md5.Encode(tun_key),
		redis_time_out:       6 * time.Second,
		wait_socket_time_out: 3 * time.Second,
		SendData:             []byte(tools.RandomString(3)),
		RecvData:             make([]byte, 1600),
	}

	for {
		tunnelClient.Release()

		conn := tunnelClient.process1()

		redisdb.Del(tunnelClient.md5_tun_key)

		if conn != nil {
			chain := make(chan int, 1)
			go func() {
				proxy.ProcessProxyClient(listener, conn)
				chain <- 1
			}()

			process_health(tunnelClient.stun_health_stream, tunnelClient.SendData, tunnelClient.RecvData)
			log.Println("连接已断开")
			tunnelClient.Release()

			if conn, err := net.Dial("tcp", tun_local_addr); conn != nil && err == nil {
				conn.Write(tunnelClient.SendData)
				conn.Close() // 关闭连接
			}

			<-chain
		}

		if !retry {
			return fmt.Errorf("连接已断开")
		}
		time.Sleep(1 * time.Second)
	}
}
