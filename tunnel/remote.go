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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
	"golang.org/x/exp/rand"
)

type TunnelServer struct {
	tun_remote_addr    string
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
}

func (c *TunnelServer) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	if c.stun_quic_conn != nil {
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
	if n, err := new_quic_stream.Write(c.SendData); n > 0 && err == nil {
		conn.SetReadDeadline(time.Time{})
		c.stun_quic_conn = new_quic_conn
		c.stun_health_stream = new_quic_stream
		c.process_chain <- new_quic_conn
	}
}

func (c *TunnelServer) SetTimeOut(conn *net.UDPConn) {
	if conn == nil {
		return
	}

	go func() {
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, remoteAddr, err := conn.ReadFromUDP(c.RecvData) // 接收数据
		if err == nil && n > 0 && remoteAddr != nil {
			conn.SetReadDeadline(time.Now().Add(6 * time.Second))
			c.process_lock.Lock()
			defer c.process_lock.Unlock()

			log.Printf("   process udp local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), remoteAddr, string(c.RecvData[:10]), n)
			c.process_quic(conn, remoteAddr)
		}
	}()
}

func (c *TunnelServer) process_send(conn *net.UDPConn, ip string, port int) {
	if c.stun_quic_conn != nil {
		return
	}
	addr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", ip, port))
	conn.WriteToUDP(c.SendData, addr)
	//log.Printf("   process_send: %v ==> %v\n", conn.LocalAddr(), addr)
}

func (c *TunnelServer) process3(conn *net.UDPConn, ip string, port int) {
	if port < 1024 || port > 65534 {
		return
	}

	for i := port - 16; i < port; i++ {
		c.process_send(conn, ip, i)
	}
}

func (c *TunnelServer) process2(conn *net.UDPConn) {
	c.process_chain = make(chan quic.Connection, 1)

	clientIP := strings.Split(c.tun_remote_addr, ":")[0]
	clientPort, _ := strconv.Atoi(strings.Split(c.tun_remote_addr, ":")[1])

	c.SetTimeOut(conn)
	c.process3(conn, clientIP, clientPort-16)

	for i := clientPort; i < clientPort+128; i += 2 {
		conn2 := tools.GetListenUDP()
		c.SetTimeOut(conn2)
		c.process3(conn2, clientIP, i)
	}

	conn3 := tools.GetListenUDP()
	c.SetTimeOut(conn3)

	port_map := make(map[int]bool)
	for i := 1; i <= 6; i++ {
		for len(port_map) < 64*i {
			if port := rand.Intn(0x2710) + i*0x2710; port > 0x400 && port <= 0xFFFF {
				if _, ok := port_map[port]; !ok {
					port_map[port] = true
					c.process_send(conn, clientIP, port)
				}
			}
		}
	}

	for i := 0x400; i < 0xFFFF; i += 2 {
		c.process3(conn, clientIP, i)
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
}

func (c *TunnelServer) process1() quic.Connection {
	c.process_chain = make(chan quic.Connection, 1)

	redisJson := RedisJsonType{}
	last_state := redisJson.State

	var conn *net.UDPConn

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
			conn = tools.GetListenUDP()
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(conn)

			redisJson.SocketTimeOut = c.socket_time_out
			redisJson.RedisTimeOut = c.redis_time_out
			redisJson.State = 1
			redisJson.SendPortCount = 0x100
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)

		case 2:
			log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)
			c.tun_remote_addr = fmt.Sprintf("%s:%d", redisJson.ClientIP, redisJson.ClientPort)
			go c.process2(conn)

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
				return nil
			}

		default:
			log.Printf("%d: 等待对端状态\n", redisJson.State)
		}

		last_state = redisJson.State
	}
}

func ProcessServer(tun_remote_addr, redis_addr, redis_pass string, radis_id int, tun_key string, max_live int, time_out time.Duration) {
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

	if tun_remote_addr == "" {
		tun_remote_addr = tools.GetFreeLocalAddr()
		if tun_remote_addr == "" {
			log.Panic("   获取本地端口失败")
			os.Exit(0)
		}
		go proxy.ListenSocks5(tun_remote_addr)
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
			tunnelServer.Release()
			continue
		}

		go func(remote string, svr *TunnelServer, conn quic.Connection) {
			defer svr.Release()
			go proxy.ProcessProxyServer(remote, conn)
			process_health(svr.stun_health_stream, svr.SendData, svr.RecvData)
			log.Printf("   心跳异常, 释放连接: %v\n", conn.LocalAddr())
		}(tun_remote_addr, &tunnelServer, conn)
	}
}
