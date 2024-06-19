package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"gogo"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
)

type TunnelServer struct {
	m_stun_quic_conn quic.Connection
	m_process_stop   bool
	m_process_lock   sync.Mutex
	m_process_chain  chan quic.Connection
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

func (c *TunnelServer) process_server5(conn *net.UDPConn, remoteAddr *net.UDPAddr) {
	c.m_process_lock.Lock()
	defer c.m_process_lock.Unlock()

	if c.m_process_stop {
		return
	}
	c.m_process_stop = true

	conn.SetDeadline(time.Time{})

	log.Printf("quic.Listen: %v\n", conn.LocalAddr())
	listener, err := quic.Listen(conn, getServerTLSConfig(), nil)
	assertErrorToNilf("process_client3 quic.Listen: %v", err)

	log.Printf("process_server5 listener.Accept: %v\n", conn.LocalAddr())
	new_quic_conn, err := listener.Accept(context.Background())
	assertErrorToNilf("process_server5 listener.Accept: %v", err)

	log.Printf("process_server5 quic.AcceptStream: %v==>%v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	new_quic_stream, err := new_quic_conn.AcceptStream(context.Background())
	assertErrorToNilf("process_server5 new_quic_conn.AcceptStream: %v", err)

	log.Printf("process_client3 new_quic_stream.Read: %v==>%v\n", new_quic_conn.RemoteAddr(), new_quic_conn.LocalAddr())
	for {
		if n, err := new_quic_stream.Read(m_recv_data); err == nil && n > 0 {
			log.Printf("process_server5 quic local:%v remote:%v recv:%v... count:%v\n", new_quic_conn.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
			process_health(new_quic_stream)
			c.m_stun_quic_conn = new_quic_conn
			c.m_process_chain <- new_quic_conn
			break
		}
	}
}

func (c *TunnelServer) start_server_child(local_addr, remote_addr string) {
	var args []string

	log.Printf("start_server_child: %s==>%s\n", local_addr, remote_addr)

	args = append(args, fmt.Sprintf("--admin_remote_addr=%s", remote_addr))
	args = append(args, fmt.Sprintf("--admin_local_addr=%s", local_addr))
	for _, temp_arg := range os.Args {
		if strings.HasPrefix(temp_arg, "--remote") {
			args = append(args, temp_arg)
		}
	}

	cmd := exec.Command(os.Args[0], args...)
	cmd.Stdout = os.Stdout //指向标准输出
	cmd.Stderr = os.Stderr //指向标准错误输出
	assertErrorToNilf("cmd.Run(): %v", cmd.Run())
}

func (c *TunnelServer) process_server_parent() {
	var redisJson RedisJsonType
	var conn *net.UDPConn

	gogo.Redis().Init(&redis.Options{
		Addr:     m_cli_redis_addr,
		Password: m_cli_redis_pass,
		DB:       m_cli_redis_id,
	})

	gogo.Redis().Del(m_cli_redis_id, m_cli_tun_key)

	for {
		if res, err := gogo.Redis().GetDB(m_cli_redis_id).Get(m_cli_tun_key).Bytes(); err == nil && res != nil && len(res) > 0 {
			if err = json.Unmarshal(res, &redisJson); err == nil {
				if redisJson.ServerPort == 0 && redisJson.ClientPort == 0 { //收到客户端通知,发送IPPORT
					log.Println("收到客户端通知,发送IPPORT")
					if conn != nil {
						conn.Close()
						conn = nil
					}
					conn, err = net.ListenUDP("udp4", nil)
					assertErrorToNilf("process_server net.ListenUDP: %v", err)
					redisJson.ServerIP, redisJson.ServerPort = getWanIpPort(conn)
					if jsonByte, err := json.Marshal(redisJson); err == nil {
						gogo.Redis().Set(m_cli_redis_id, m_cli_tun_key, string(jsonByte), m_process_time_out)
					}
					goto NEXT_CHECK

				} else if redisJson.ServerPort > 0 && redisJson.ClientPort == 0 { //等待客户端响应
					log.Println("等待客户端响应")
					goto NEXT_CHECK

				} else if redisJson.ServerPort > 0 && redisJson.ClientPort > 0 { //客户端返回IPORT
					log.Println("收到客户端返回的IPORT")
					gogo.Redis().Del(m_cli_redis_id, m_cli_tun_key)
					localAddr := conn.LocalAddr().String()
					conn.Close()
					conn = nil
					go c.start_server_child(localAddr, fmt.Sprintf("%s:%d", redisJson.ClientIP, redisJson.ClientPort))
					goto NEXT_CHECK
				}

				log.Println("gogo.Redis().GetDB other")
				goto NEXT_CHECK
			}
		}

	NEXT_CHECK:
		gogo.Utils().TimeSleepSecond(1)
	}
}

func (c *TunnelServer) process_server_child() quic.Connection {
	var conn *net.UDPConn

	c.m_process_chain = make(chan quic.Connection, 1)

	localAddr, err := net.ResolveUDPAddr("udp4", m_cli_admin_local_addr)
	assertErrorToNilf("process_server net.ResolveUDPAddr: %v", err)

	conn, err = net.ListenUDP("udp4", localAddr)
	assertErrorToNilf("process_server net.ListenUDP: %v", err)

	conn.SetDeadline(time.Now().Add(m_process_time_out))

	go func() {
		for !c.m_process_stop {
			n, remoteAddr, err := conn.ReadFromUDP(m_recv_data) // 接收数据
			if err == nil && n > 0 {
				log.Printf("process_server udp local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
				c.process_server5(conn, remoteAddr)
				break
			}
		}
	}()

	clientIP := strings.Split(m_cli_admin_remote_addr, ":")[0]
	clientPort, _ := strconv.Atoi(strings.Split(m_cli_admin_remote_addr, ":")[1])

	for i := -32; i >= 64 && !c.m_process_stop; i += 2 {
		c.process_server2(conn, clientIP, clientPort+i, m_send_data)
	}

	if !c.m_process_stop {
		c.process_server4(conn, clientIP, m_send_data)
	}

	select {
	case <-c.m_process_chain:
		return c.m_stun_quic_conn
	case <-time.After(m_process_time_out):
		return nil
	}
}

func (c *TunnelServer) GetQuicConn() quic.Connection {
	return c.m_stun_quic_conn
}
