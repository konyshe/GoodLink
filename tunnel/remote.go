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
	"golang.org/x/exp/rand"
)

type TunnelServer struct {
	stun_quic_conn     quic.Connection
	stun_health_stream quic.Stream
	process_lock       sync.Mutex
	process_chain      chan quic.Connection
	redisdb            *redis.Client
	tun_key            string
	md5_tun_key        string
	SendData           []byte
	RecvData           []byte
	redis_time_out     time.Duration
	socket_time_out    time.Duration
	conn               *net.UDPConn
}

func (c *TunnelServer) process_send(dst_ip string, dst_port int) {
	if c.conn == nil || dst_ip == "" || dst_port <= 0 || dst_port >= 65535 {
		return
	}

	remoteAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dst_ip, dst_port))

	c.process_lock.Lock()
	defer c.process_lock.Unlock()

	if c.stun_quic_conn != nil {
		return
	}

	c.conn.WriteToUDP(c.SendData, remoteAddr)
}

func (c *TunnelServer) process_server2(remote_ip string, remote_port int) {
	for i := remote_port - 16; i <= remote_port; i++ {
		c.process_send(remote_ip, i)
	}
}

func (c *TunnelServer) process_server4(remote_ip string) {
	for i := 1; i <= 8; i++ {
		for remote_port_map := make(map[int]bool); len(remote_port_map) <= 128; {
			if remote_port := rand.Intn(8196 * i); remote_port > 8196*(i-1) && remote_port <= 8196*i {
				if _, ok := remote_port_map[remote_port]; !ok {
					//log.Printf("rand port: %d\n", tun_remote_port)
					remote_port_map[remote_port] = true
					c.process_send(remote_ip, remote_port)
				}
			}
		}
	}
}

func (c *TunnelServer) process_server5(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	c.process_lock.Lock()
	defer c.process_lock.Unlock()

	if c.stun_quic_conn != nil {
		return
	}

	//conn.SetReadDeadline(time.Now().Add(c.socket_time_out))

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
	if n, err := new_quic_stream.Write(c.SendData); n > 0 && err == nil {
		//conn.SetReadDeadline(time.Time{})
		c.stun_quic_conn = new_quic_conn
		c.stun_health_stream = new_quic_stream
		c.process_chain <- new_quic_conn
	}
}

func (c *TunnelServer) GetQuicConn() quic.Connection {
	return c.stun_quic_conn
}

func (c *TunnelServer) Release() {
	log.Println("   清空缓存和连接")

	if c.stun_health_stream != nil {
		c.stun_health_stream.Close()
		c.stun_health_stream = nil
	}

	if c.stun_quic_conn != nil {
		c.stun_quic_conn.CloseWithError(0, "0")
		c.stun_quic_conn = nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *TunnelServer) ProcessServerChild(ip string, port int) {
	go func() {
		//conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		n, remote_addr, err := c.conn.ReadFromUDP(c.RecvData) // 接收数据
		if err == nil && n > 0 {
			//conn.SetReadDeadline(time.Time{})
			log.Printf("process_server udp local:%v remote:%v recv:%v... count:%v\n", c.conn.LocalAddr(), remote_addr, string(c.RecvData[:10]), n)
			c.process_server5(c.conn, remote_addr)
			return
		}
		c.conn.Close()
	}()

	for i := -32; i <= 64 && c.stun_quic_conn == nil; i += 2 {
		c.process_server2(ip, port+i)
	}

	if c.stun_quic_conn == nil {
		c.process_server4(ip)
	}
}

func (c *TunnelServer) process1() quic.Connection {
	c.process_chain = make(chan quic.Connection, 1)

	redisJson := RedisJsonType{}
	last_state := redisJson.State

	for {
		time.Sleep(1 * time.Second)

		for RedisGet(c.redisdb, c.tun_key, c.md5_tun_key, &redisJson) != nil {
			time.Sleep(3 * time.Second)
		}

		if redisJson.State < last_state {
			log.Println("   对端已重置连接")
			return nil
		}

		if redisJson.State-last_state > 1 {
			log.Println("   状态异常")
			c.redisdb.Del(c.md5_tun_key)
			return nil
		}

		switch redisJson.State {
		case 0:
			log.Printf("%d: 收到对端请求\n", redisJson.State)
			c.conn = tools.GetListenUDP()
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(c.conn)

			redisJson.SocketTimeOut = c.socket_time_out
			redisJson.RedisTimeOut = c.redis_time_out
			redisJson.State = 1
			redisJson.SendPortCount = 0x100
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)

		case 2:
			log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)

			go c.ProcessServerChild(redisJson.ClientIP, redisJson.ClientPort)

			select {
			case <-c.process_chain:
				redisJson.State = 3
				log.Printf("%d: 通知对端, 连接成功\n", redisJson.State)
				RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)
				return c.stun_quic_conn

			case <-time.After(c.socket_time_out):
				redisJson.State = 4
				log.Printf("%d: 通知对端, 连接超时\n", redisJson.State)
				RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)
				c.Release()
				return nil
			}

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}

		last_state = redisJson.State
	}
}

func ProcessServer(remote_addr, redis_addr, redis_pass string, radis_id int, tun_key string, time_out time.Duration) {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})
	if redisdb == nil {
		log.Panic("   Redis初始化失败")
		os.Exit(0)
	}
	defer redisdb.Close()

	if remote_addr == "" {
		remote_addr = tools.GetFreeLocalAddr()
		if remote_addr == "" {
			log.Panic("   获取本地端口失败")
			os.Exit(0)
		}
		go proxy.ListenSocks5(remote_addr)
	}

	for {
		tunnelServer := TunnelServer{
			redisdb:         redisdb,
			tun_key:         tun_key,
			md5_tun_key:     md5.Encode(tun_key),
			SendData:        []byte(tools.RandomString(3)),
			RecvData:        make([]byte, 128),
			redis_time_out:  time_out * 3,
			socket_time_out: time_out,
			stun_quic_conn:  nil,
		}

		conn := tunnelServer.process1()
		if conn == nil {
			continue
		}

		go func(remote string, svr *TunnelServer, conn quic.Connection) {
			defer svr.Release()
			go proxy.ProcessProxyServer(remote, conn)
			process_health(svr.stun_health_stream, svr.SendData, svr.RecvData)
			log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		}(remote_addr, &tunnelServer, conn)
	}
}
