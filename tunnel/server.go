package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"goodlink/proxy"
	"goodlink/tools"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
)

type TunnelServer struct {
	m_stun_quic_conn     quic.Connection
	m_stun_health_stream quic.Stream
	m_process_stop       bool
	m_process_lock       sync.Mutex
	m_process_chain      chan quic.Connection
}

func (c *TunnelServer) process_server2(conn *net.UDPConn, ip string, port int, m_send_data []byte) {
	for i := 0; i <= 16; i++ {
		go process_send(conn, ip, port+i, m_send_data, &c.m_process_stop)
	}
}

func (c *TunnelServer) process_server4(conn *net.UDPConn, ip string, m_send_data []byte) {
	dst_port_map := make(map[int]bool)
	dst_port := 0

	for len(dst_port_map) <= 512 {
		dst_port = rand.Intn(65535)
		if dst_port == 0 {
			continue
		}
		if _, ok := dst_port_map[dst_port]; ok {
			continue
		}
		dst_port_map[dst_port] = true
		go process_send(conn, ip, dst_port, m_send_data, &c.m_process_stop)
	}
}

func (c *TunnelServer) process_server5(conn *net.UDPConn, remoteAddr *net.UDPAddr, send_data, recv_data []byte) {
	c.m_process_lock.Lock()
	defer c.m_process_lock.Unlock()

	if c.m_process_stop {
		return
	}
	c.m_process_stop = true

	conn.SetDeadline(time.Time{})

	log.Printf("quic.Listen: %v\n", conn.LocalAddr())
	listener, err := quic.Listen(conn, getServerTLSConfig(), nil)
	tools.AssertErrorToNilf("process_client3 quic.Listen: %v", err)

	log.Printf("process_server5 listener.Accept: %v\n", conn.LocalAddr())
	new_quic_conn, err := listener.Accept(context.Background())
	tools.AssertErrorToNilf("process_server5 listener.Accept: %v", err)

	log.Printf("process_server5 quic.AcceptStream: %v==>%v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	new_quic_stream, err := new_quic_conn.AcceptStream(context.Background())
	tools.AssertErrorToNilf("process_server5 new_quic_conn.AcceptStream: %v", err)

	log.Printf("process_client3 new_quic_stream.Read: %v==>%v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	for {
		if n, err := new_quic_stream.Read(recv_data); err == nil && n > 0 {
			log.Printf("process_server5 quic local:%v remote:%v recv:%v... count:%v\n", new_quic_conn.LocalAddr(), remoteAddr, string(recv_data[:10]), n)
			c.m_stun_quic_conn = new_quic_conn
			c.m_stun_health_stream = new_quic_stream
			c.m_process_chain <- new_quic_conn
			break
		}
	}
}

func (c *TunnelServer) ProcessServerChild(local_addr, remote_addr string, send_data, recv_data []byte) quic.Connection {
	var conn *net.UDPConn

	process_time_out := 15 * time.Second

	c.m_process_chain = make(chan quic.Connection, 1)

	localAddr, err := net.ResolveUDPAddr("udp4", local_addr)
	tools.AssertErrorToNilf("process_server net.ResolveUDPAddr: %v", err)

	conn, err = net.ListenUDP("udp4", localAddr)
	tools.AssertErrorToNilf("process_server net.ListenUDP: %v", err)

	conn.SetDeadline(time.Now().Add(process_time_out))

	go func() {
		for !c.m_process_stop {
			n, remoteAddr, err := conn.ReadFromUDP(recv_data) // 接收数据
			if err == nil && n > 0 {
				log.Printf("process_server udp local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), remoteAddr, string(recv_data[:10]), n)
				c.process_server5(conn, remoteAddr, send_data, recv_data)
				break
			}
		}
	}()

	clientIP := strings.Split(remote_addr, ":")[0]
	clientPort, _ := strconv.Atoi(strings.Split(remote_addr, ":")[1])

	for i := -32; i >= 64 && !c.m_process_stop; i += 2 {
		c.process_server2(conn, clientIP, clientPort+i, send_data)
	}

	if !c.m_process_stop {
		c.process_server4(conn, clientIP, send_data)
	}

	select {
	case <-c.m_process_chain:
		return c.m_stun_quic_conn
	case <-time.After(process_time_out):
		return nil
	}
}

func (c *TunnelServer) GetQuicConn() quic.Connection {
	return c.m_stun_quic_conn
}

func ProcessServer2(m_cli_tun_remote_addr string, local_addr, remote_addr string) {
	log.Printf("start_server_child: %s==>%s\n", local_addr, remote_addr)
	var tunnelServer TunnelServer
	recv_data := make([]byte, 1600)
	send_data := []byte(tools.RandomString(9))
	go proxy.ProcessProxyServer(m_cli_tun_remote_addr, tunnelServer.ProcessServerChild(local_addr, remote_addr, send_data, recv_data))
	process_health(tunnelServer.m_stun_health_stream, send_data, recv_data)

	log.Printf("stop_server_child: %s==>%s\n", local_addr, remote_addr)
	tunnelServer.m_stun_quic_conn.CloseWithError(0, "0")
}

func ProcessServer(m_cli_tun_remote_addr, redis_addr, redis_pass string, radis_id int, redis_key string) {
	var redisJson RedisJsonType
	var conn *net.UDPConn

	process_time_out := 15 * time.Second

	//连接服务器
	redisdb := redis.NewClient(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})

	if redisdb == nil {
		log.Println("Redis初始化失败")
		return
	}

	redisdb.Del(redis_key)

	for {
		if res, err := redisdb.Get(redis_key).Bytes(); err == nil && res != nil && len(res) > 0 {
			if err = json.Unmarshal(res, &redisJson); err == nil {
				if redisJson.ServerPort == 0 && redisJson.ClientPort == 0 { //收到客户端通知,发送IPPORT
					log.Println("收到客户端通知,发送IPPORT")
					if conn != nil {
						conn.Close()
						conn = nil
					}
					conn, err = net.ListenUDP("udp4", nil)
					tools.AssertErrorToNilf("process_server net.ListenUDP: %v", err)
					redisJson.ServerIP, redisJson.ServerPort = getWanIpPort(conn)
					if jsonByte, err := json.Marshal(redisJson); err == nil {
						redisdb.Set(redis_key, string(jsonByte), process_time_out)
					}
					goto NEXT_CHECK

				} else if redisJson.ServerPort > 0 && redisJson.ClientPort == 0 { //等待客户端响应
					log.Println("等待客户端响应")
					goto NEXT_CHECK

				} else if redisJson.ServerPort > 0 && redisJson.ClientPort > 0 { //客户端返回IPORT
					log.Println("收到客户端返回的IPORT")
					redisdb.Del(redis_key)
					local_addr := conn.LocalAddr().String()
					conn.Close()
					conn = nil
					go func() {
						ProcessServer2(m_cli_tun_remote_addr, local_addr, fmt.Sprintf("%s:%d", redisJson.ClientIP, redisJson.ClientPort))
					}()
					goto NEXT_CHECK
				}

				log.Println("redisdb.GetDB other")
				goto NEXT_CHECK
			}
		}

	NEXT_CHECK:
		time.Sleep(1 * time.Second)
	}
}
