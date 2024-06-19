package main

import (
	"context"
	"encoding/json"
	"gogo"
	"gogo/workpool"
	"log"
	"net"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
)

type TunnelClient struct {
	m_stun_quic_conn quic.Connection
	m_process_stop   bool
	m_process_lock   sync.Mutex
	m_process_chain  chan quic.Connection
	m_work_pool      *workpool.WorkPool
}

func (c *TunnelClient) process_client3(conn *net.UDPConn, remoteAddr *net.UDPAddr, m_send_data []byte) {
	c.m_process_lock.Lock()
	defer c.m_process_lock.Unlock()

	if c.m_process_stop {
		return
	}
	c.m_process_stop = true

	conn.SetDeadline(time.Time{})

	log.Printf("process_client3 conn.WriteToUDP: %v==>%v\n", conn.LocalAddr(), remoteAddr)
	_, err := conn.WriteToUDP(m_send_data, remoteAddr)
	assertErrorToNilf("process_client3 conn.WriteToUDP: %v", err)

	gogo.Utils().TimeSleepSecond(1)

	log.Printf("process_client3 quic.Dial: %v==>%v\n", conn.LocalAddr(), remoteAddr)
	new_quic_conn, err := quic.Dial(context.Background(), conn, remoteAddr, getClientTLSConfig(), nil)
	assertErrorToNilf("process_client3 quic.Dial: %v", err)

	log.Printf("process_client3 new_quic_conn.OpenStreamSync: %v==>%v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	new_quic_stream, err := new_quic_conn.OpenStreamSync(context.Background())
	assertErrorToNilf("process_server5 quic_conn.OpenStreamSync: %v", err)

	log.Printf("process_server5 new_quic_stream.Write: %v==>%v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	for {
		if n, err := new_quic_stream.Write([]byte(m_send_data)); n > 0 && err == nil {
			process_health(new_quic_stream)
			c.m_stun_quic_conn = new_quic_conn
			c.m_process_chain <- new_quic_conn
			break
		}
	}
}

func (c *TunnelClient) process_client2(ip string, port int, m_send_data []byte) {
	conn, err := net.ListenUDP("udp4", nil)
	assertErrorToNilf("process_server2 net.ListenUDP: %v", err)

	conn.SetReadDeadline(time.Now().Add(6 * time.Second))

	c.m_work_pool.Do(func() error {
		for !c.m_process_stop {
			if n, remoteAddr, err := conn.ReadFromUDP(m_recv_data); err == nil && n > 0 {
				log.Printf("process_client2 udp local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), remoteAddr, string(m_recv_data[:10]), n)
				c.process_client3(conn, remoteAddr, m_send_data)
				return nil
			}
		}
		conn.Close()
		return nil
	})

	c.m_work_pool.Do(func() error {
		process_send(conn, ip, port, m_send_data, &c.m_process_stop)
		return nil
	})
}

type RedisJsonType struct {
	ServerIP   string `bson:"server_ip" json:"server_ip"`
	ServerPort int    `bson:"server_port" json:"server_port"`
	ClientIP   string `bson:"client_ip" json:"client_ip"`
	ClientPort int    `bson:"client_port" json:"client_port"`
}

func (c *TunnelClient) process_client1() quic.Connection {
	var redisJson RedisJsonType
	var conn *net.UDPConn

	c.m_process_stop = false
	c.m_process_chain = make(chan quic.Connection, 1)
	c.m_work_pool = workpool.NewWorkPool(10240)

	for {
		if res, err := gogo.Redis().GetDB(m_cli_redis_id).Get(m_cli_tun_key).Bytes(); err == nil && res != nil && len(res) > 0 {
			if err = json.Unmarshal(res, &redisJson); err == nil {
				if redisJson.ServerPort == 0 && redisJson.ClientPort == 0 { //等待服务器响应
					log.Println("等待服务器响应")
					goto NEXT_CHECK

				} else if redisJson.ServerPort > 0 && redisJson.ClientPort == 0 { //服务器已返回IPPORT
					log.Println("收到服务器返回的IPPORT")
					conn, err = net.ListenUDP("udp4", nil)
					assertErrorToNilf("main net.ListenUDP: %v", err)
					redisJson.ClientIP, redisJson.ClientPort = getWanIpPort(conn)
					if jsonByte, err := json.Marshal(redisJson); err == nil {
						gogo.Redis().Set(m_cli_redis_id, m_cli_tun_key, string(jsonByte), m_process_time_out)
						break
					}
				}

				log.Println("gogo.Redis().GetDB other")
				goto NEXT_CHECK
			}
		}

		//走到这里，表示当前没有其他正在建立隧道的会话，下面开始告知服务端准备建立隧道
		log.Println("告知服务端准备建立隧道")
		if jsonByte, err := json.Marshal(RedisJsonType{}); err == nil {
			gogo.Redis().SetNx(m_cli_redis_id, m_cli_tun_key, string(jsonByte), m_process_time_out)
		}
	NEXT_CHECK:
		gogo.Utils().TimeSleepSecond(1)
	}

	conn.Close()

	for i := 0; i <= 256 && !c.m_process_stop; i++ {
		c.process_client2(redisJson.ServerIP, redisJson.ServerPort, m_send_data)
	}

	work_pool_end := make(chan int, 1)

	go func() {
		c.m_work_pool.Wait()
		work_pool_end <- 1
	}()

	select {
	case <-c.m_process_chain:
		return c.m_stun_quic_conn
	case <-time.After(m_process_time_out):
		c.m_process_stop = true
	case <-work_pool_end:
		break
	}

	return nil
}

func (c *TunnelClient) process_client() quic.Connection {
	gogo.Redis().Init(&redis.Options{
		Addr:     m_cli_redis_addr,
		Password: m_cli_redis_pass,
		DB:       m_cli_redis_id,
	})

	for {
		if ret := c.process_client1(); ret != nil {
			return ret
		}
		gogo.Utils().TimeSleepSecond(1)
	}
}

func (c *TunnelClient) GetQuicConn() quic.Connection {
	return c.m_stun_quic_conn
}
