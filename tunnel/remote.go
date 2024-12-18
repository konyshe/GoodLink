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
	remote_addr_list   []*net.UDPAddr
	redis_time_out     time.Duration
	socket_time_out    time.Duration
	conn               *net.UDPConn
}

func (c *TunnelServer) process_send_map() {
	for _, remoteAddr := range c.remote_addr_list {
		c.process_lock.Lock()
		if c.conn == nil {
			c.process_lock.Unlock()
			break
		}
		if c.stun_quic_conn != nil {
			c.process_lock.Unlock()
			break
		}
		_, err := c.conn.WriteToUDP(c.SendData, remoteAddr)
		c.process_lock.Unlock()
		if err != nil {
			break
		}
	}
}

/*
	func (c *TunnelServer) process_rand(conn *net.UDPConn, tun_remote_ip string, count int) {
		count2 := len(c.send_conn_map) + count

		for len(c.send_conn_map) <= count2 && c.stun_quic_conn == nil {
			if port := 8196 + rand.Intn(65535); port < 65535 {
				if _, ok := c.send_conn_map[port]; !ok {
					c.process_send_map(conn, tun_remote_ip, port)
				}
			}
		}
	}
*/
func (c *TunnelServer) process_quic(remoteAddr *net.UDPAddr) {
	if c.stun_quic_conn != nil {
		return
	}

	time.Sleep(1000 * time.Millisecond)

	log.Printf("   process_client3 quic.Dial: %v ==> %v\n", c.conn.LocalAddr(), remoteAddr)
	new_quic_conn, err := quic.Dial(context.Background(), c.conn, remoteAddr, tls2.GetClientTLSConfig(), nil)
	if err != nil {
		log.Printf("   process_client3 quic.Dial: %v\n", err)
		return
	}

	log.Printf("   process_client3 new_quic_conn.OpenStreamSync: %v ==> %v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	new_quic_stream, err := new_quic_conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("   process_quic quic_conn.OpenStreamSync: %v\n", err)
		return
	}

	log.Printf("   process_quic new_quic_stream.Write: %v ==> %v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	//new_quic_stream.SetDeadline(time.Now().Add(30 * time.Second))
	if n, err := new_quic_stream.Write(c.SendData); n > 0 && err == nil {
		//new_quic_stream.SetDeadline(time.Time{})
		c.stun_quic_conn = new_quic_conn
		c.stun_health_stream = new_quic_stream
		c.process_chain <- new_quic_conn
	}
}

func (c *TunnelServer) process2() {
	//log.Printf("   start_server_child: %v ==> %s\n", c.conn.LocalAddr(), c.tun_remote_addr)

	c.process_chain = make(chan quic.Connection, 1)
	//c.send_conn_map = make(map[int]string)

	//c.conn.SetDeadline(time.Now().Add(30 * time.Second))

	go func() {
		n, remoteAddr, err := c.conn.ReadFromUDP(c.RecvData) // 接收数据
		if err == nil && n > 0 {
			c.process_lock.Lock()
			log.Printf("   process udp local:%v remote:%v recv:%v... count:%v\n", c.conn.LocalAddr(), remoteAddr, string(c.RecvData[:10]), n)
			c.process_quic(remoteAddr)
			c.process_lock.Unlock()
			return
		}
	}()

	clientIP := strings.Split(c.tun_remote_addr, ":")[0]
	clientPort, _ := strconv.Atoi(strings.Split(c.tun_remote_addr, ":")[1])

	for i := clientPort - 0x400; i > 0x400 && i < clientPort+0x2800 && c.stun_quic_conn == nil; i++ {
		remoteAddr, _ := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", clientIP, i))
		c.remote_addr_list = append(c.remote_addr_list, remoteAddr)
	}

	//c.process_rand(conn, clientIP)
}

func (c *TunnelServer) GetQuicConn() quic.Connection {
	return c.stun_quic_conn
}

func (c *TunnelServer) Release() {
	log.Println("   清空缓存和连接")

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

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *TunnelServer) process1() quic.Connection {
	var err error
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
			if c.conn, err = net.ListenUDP("udp4", nil); err != nil {
				log.Printf("   net.ListenUDP: %v\n", err)
				return nil
			}
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort2(c.conn)

			redisJson.SocketTimeOut = c.socket_time_out
			redisJson.RedisTimeOut = c.redis_time_out
			redisJson.State = 1
			redisJson.SendPortCount = 0x100
			log.Printf("%d: 发送本端地址: %v\n", redisJson.State, redisJson)
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)

		case 2:
			log.Printf("%d: 收到对端地址, 发起连接: %v\n", redisJson.State, redisJson)
			c.tun_remote_addr = fmt.Sprintf("%s:%d", redisJson.ClientIP, redisJson.ClientPort)
			c.process2()
			c.process_send_map()
			go func() {
				for {
					time.Sleep(1000 * time.Millisecond)
					c.process_send_map()
				}
			}()

			select {
			case <-c.process_chain:
				redisJson.State = 3
				log.Printf("%d: 通知对端, 连接成功\n", redisJson.State)
				RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)
				return c.stun_quic_conn
			case <-time.After(redisJson.SocketTimeOut):
				redisJson.State = 4
				log.Printf("%d: 通知对端, 连接超时\n", redisJson.State)
				RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)
				return nil
			}
		}

		last_state = redisJson.State
	}
}

func ProcessServer(tun_remote_addr, redis_addr, redis_pass string, radis_id int, tun_key string) {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})
	if redisdb == nil {
		log.Fatalln("   Redis初始化失败")
		os.Exit(0)
	}
	defer redisdb.Close()

	if tun_remote_addr == "" {
		tun_remote_addr = tools.GetFreeLocalAddr()
		if tun_remote_addr == "" {
			log.Fatalln("   获取本地端口失败")
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
			RecvData:        make([]byte, 1600),
			redis_time_out:  30 * time.Second,
			socket_time_out: 6 * time.Second,
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
		}(tun_remote_addr, &tunnelServer, conn)
	}
}
