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
	"math/rand"
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
	tun_local_addr     string
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
	send_conn_map      map[int]*net.UDPConn
}

func (c *TunnelServer) process_send(conn *net.UDPConn, dst_ip string, dst_port int) {
	if conn == nil || dst_ip == "" || dst_port <= 0 || dst_port >= 65535 {
		return
	}

	c.process_lock.Lock()
	defer c.process_lock.Unlock()

	if c.stun_quic_conn != nil {
		return
	}

	remoteAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dst_ip, dst_port))
	tools.AssertErrorToNilf("process_send net.ResolveUDPAddr: %v", err)

	//log.Printf("process_send send: %v => %v\n", conn.LocalAddr(), remoteAddr)

	c.send_conn_map[dst_port] = conn

	conn.WriteToUDP(c.SendData, remoteAddr)
}

func (c *TunnelServer) process_rand(conn *net.UDPConn, tun_remote_ip string, count int) {
	count2 := len(c.send_conn_map) + count

	for len(c.send_conn_map) <= count2 && c.stun_quic_conn == nil {
		if port := 8196 + rand.Intn(65535); port < 65535 {
			if _, ok := c.send_conn_map[port]; !ok {
				c.process_send(conn, tun_remote_ip, port)
			}
		}
	}
}

func (c *TunnelServer) process_quic(conn *net.UDPConn, remoteAddr *net.UDPAddr, send_data, recv_data []byte) {
	c.process_lock.Lock()
	defer c.process_lock.Unlock()

	if c.stun_quic_conn != nil {
		return
	}

	log.Printf("process_client3 quic.Dial: %v==>%v\n", conn.LocalAddr(), remoteAddr)
	new_quic_conn, err := quic.Dial(context.Background(), conn, remoteAddr, tls2.GetClientTLSConfig(), nil)
	if err != nil {
		log.Printf("process_client3 quic.Dial: %v\n", err)
		return
	}

	log.Printf("process_client3 new_quic_conn.OpenStreamSync: %v==>%v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	new_quic_stream, err := new_quic_conn.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("process_quic quic_conn.OpenStreamSync: %v\n", err)
		return
	}

	log.Printf("process_quic new_quic_stream.Write: %v==>%v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	if n, err := new_quic_stream.Write([]byte(send_data)); n > 0 && err == nil {
		c.stun_quic_conn = new_quic_conn
		c.stun_health_stream = new_quic_stream
		c.process_chain <- new_quic_conn
	}
}

func (c *TunnelServer) process2() {
	log.Printf("start_server_child: %s==>%s\n", c.tun_local_addr, c.tun_remote_addr)

	c.process_chain = make(chan quic.Connection, 1)
	c.send_conn_map = make(map[int]*net.UDPConn)

	localAddr, err := net.ResolveUDPAddr("udp4", c.tun_local_addr)
	tools.AssertErrorToNilf("process net.ResolveUDPAddr: %v", err)

	conn, err := net.ListenUDP("udp4", localAddr)
	tools.AssertErrorToNilf("process net.ListenUDP: %v", err)

	go func() {
		n, conn_remote_addr, err := conn.ReadFromUDP(c.RecvData) // 接收数据
		if err == nil && n > 0 {
			log.Printf("process udp local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), conn_remote_addr, string(c.RecvData[:10]), n)
			c.process_quic(conn, conn_remote_addr, c.SendData, c.RecvData)

			log.Println("   清空历史连接")
			for port, conn := range c.send_conn_map {
				if port != conn_remote_addr.Port {
					conn.Close()
					delete(c.send_conn_map, port)
				}
			}
			return
		}
		conn.Close()
	}()

	clientIP := strings.Split(c.tun_remote_addr, ":")[0]
	clientPort, _ := strconv.Atoi(strings.Split(c.tun_remote_addr, ":")[1])

	for i := -1024; i >= 1024 && c.stun_quic_conn == nil; i++ {
		c.process_send(conn, clientIP, clientPort+i)
	}

	c.process_rand(conn, clientIP, 1024)
}

func (c *TunnelServer) GetQuicConn() quic.Connection {
	return c.stun_quic_conn
}

func (c *TunnelServer) Release() {
	log.Printf("stop_server_child: %s==>%s\n", c.tun_local_addr, c.tun_remote_addr)

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

	for port, conn := range c.send_conn_map {
		conn.Close()
		delete(c.send_conn_map, port)
	}
	c.send_conn_map = nil
}

func (c *TunnelServer) process1() quic.Connection {
	var conn *net.UDPConn

	c.process_chain = make(chan quic.Connection, 1)

	redisJson := RedisJsonType{}
	last_state := redisJson.State

	for RedisGet(c.redisdb, c.tun_key, c.md5_tun_key, &redisJson) != nil {
		time.Sleep(3 * time.Second)
	}

	for {
		time.Sleep(1 * time.Second)

		err := RedisGet(c.redisdb, c.tun_key, c.md5_tun_key, &redisJson)
		if err != nil {
			log.Println(err)
			return nil
		}

		if redisJson.State < last_state {
			log.Println("   重启连接")
			return nil
		}

		switch redisJson.State {
		case 0:
			log.Println("0: 收到对端请求")
			if conn, err = net.ListenUDP("udp4", nil); err != nil {
				log.Printf("net.ListenUDP: %v\n", err)
				return nil
			}
			redisJson.ServerIP, redisJson.ServerPort = stun2.GetWanIpPort(conn)

			log.Printf("   发送本端地址: %v\n", redisJson)
			redisJson.State = 1
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)

		case 1:
			if last_state != redisJson.State {
				log.Println("1: 等待对端响应")
				last_state = redisJson.State
			}

		case 2:
			log.Println("2: 收到对端地址")
			c.tun_local_addr = conn.LocalAddr().String()
			conn.Close()
			conn = nil
			c.tun_remote_addr = fmt.Sprintf("%s:%d", redisJson.ServerIP, redisJson.ServerPort)
			c.process2()

			log.Printf("3: 通知对端等待连接: %v\n", redisJson)
			redisJson.State = 3
			RedisSet(c.redisdb, c.tun_key, c.md5_tun_key, redisJson.RedisTimeOut, &redisJson)

		case 3:
			if last_state != redisJson.State {
				log.Println("3: 等待对端响应")
				last_state = redisJson.State
			}

		case 4:
			log.Println("4: 收到对端响应, 开始计算超时")
			select {
			case <-c.process_chain:
				log.Println("   连接成功!")
				return c.stun_quic_conn
			case <-time.After(redisJson.SocketTimeOut):
				log.Println("   连接超时!")
				return nil
			}

		default:
			log.Printf("   发现异常状态: %d\n", redisJson.State)
			return nil
		}
	}
}

func ProcessServer(tun_remote_addr, redis_addr, redis_pass string, radis_id int, tun_key string) {
	redisdb := redis.NewClient(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})
	if redisdb == nil {
		log.Fatalln("Redis初始化失败")
		os.Exit(0)
	}
	defer redisdb.Close()

	if tun_remote_addr == "" {
		tun_remote_addr = tools.GetFreeLocalAddr()
		if tun_remote_addr == "" {
			log.Fatalln("获取本地端口失败")
			os.Exit(0)
		}
		go proxy.ListenSocks5(tun_remote_addr)
	}
	for {
		tunnelServer := TunnelServer{
			redisdb:     redisdb,
			tun_key:     tun_key,
			md5_tun_key: md5.Encode(tun_key),
			SendData:    []byte(tools.RandomString(3)),
			RecvData:    make([]byte, 1600),
		}

		conn := tunnelServer.process1()
		if conn == nil {
			tunnelServer.Release()

		} else {
			go func() {
				go proxy.ProcessProxyServer(tun_remote_addr, conn)
				process_health(tunnelServer.stun_health_stream, tunnelServer.SendData, tunnelServer.RecvData)
				tunnelServer.Release()
			}()
		}

		time.Sleep(3 * time.Second)
	}
}
