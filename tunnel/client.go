package tunnel

import (
	"context"
	"encoding/json"
	"gogo"
	"gogo/workpool"
	"goodlink/proxy"
	"goodlink/tools"
	"log"
	"net"
	"sync"
	"time"

	"github.com/go-redis/redis"
	"github.com/quic-go/quic-go"
)

type TunnelClient struct {
	m_stun_quic_conn     quic.Connection
	m_stun_health_stream quic.Stream
	m_process_stop       bool
	m_process_lock       sync.Mutex
	m_process_chain      chan quic.Connection
	m_work_pool          *workpool.WorkPool
}

func (c *TunnelClient) process_client3(conn *net.UDPConn, remoteAddr *net.UDPAddr, send_data, recv_data []byte) {
	c.m_process_lock.Lock()
	defer c.m_process_lock.Unlock()

	if c.m_process_stop {
		return
	}
	c.m_process_stop = true

	conn.SetDeadline(time.Time{})

	log.Printf("process_client3 conn.WriteToUDP: %v==>%v\n", conn.LocalAddr(), remoteAddr)
	_, err := conn.WriteToUDP(send_data, remoteAddr)
	tools.AssertErrorToNilf("process_client3 conn.WriteToUDP: %v", err)

	time.Sleep(1 * time.Second)

	log.Printf("process_client3 quic.Dial: %v==>%v\n", conn.LocalAddr(), remoteAddr)
	new_quic_conn, err := quic.Dial(context.Background(), conn, remoteAddr, getClientTLSConfig(), nil)
	tools.AssertErrorToNilf("process_client3 quic.Dial: %v", err)

	log.Printf("process_client3 new_quic_conn.OpenStreamSync: %v==>%v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	new_quic_stream, err := new_quic_conn.OpenStreamSync(context.Background())
	tools.AssertErrorToNilf("process_server5 quic_conn.OpenStreamSync: %v", err)

	log.Printf("process_server5 new_quic_stream.Write: %v==>%v\n", new_quic_conn.LocalAddr(), new_quic_conn.RemoteAddr())
	for {
		if n, err := new_quic_stream.Write([]byte(send_data)); n > 0 && err == nil {
			c.m_stun_health_stream = new_quic_stream
			c.m_stun_quic_conn = new_quic_conn
			c.m_process_chain <- new_quic_conn
			break
		}
	}
}

func (c *TunnelClient) process_client2(ip string, port int, send_data, recv_data []byte) {
	conn, err := net.ListenUDP("udp4", nil)
	tools.AssertErrorToNilf("process_server2 net.ListenUDP: %v", err)

	conn.SetReadDeadline(time.Now().Add(6 * time.Second))

	c.m_work_pool.Do(func() error {
		for !c.m_process_stop {
			if n, remoteAddr, err := conn.ReadFromUDP(recv_data); err == nil && n > 0 {
				log.Printf("process_client2 udp local:%v remote:%v recv:%v... count:%v\n", conn.LocalAddr(), remoteAddr, string(recv_data[:10]), n)
				c.process_client3(conn, remoteAddr, send_data, recv_data)
				return nil
			}
		}
		conn.Close()
		return nil
	})

	c.m_work_pool.Do(func() error {
		process_send(conn, ip, port, send_data, &c.m_process_stop)
		return nil
	})
}

type RedisJsonType struct {
	ServerIP   string `bson:"server_ip" json:"server_ip"`
	ServerPort int    `bson:"server_port" json:"server_port"`
	ClientIP   string `bson:"client_ip" json:"client_ip"`
	ClientPort int    `bson:"client_port" json:"client_port"`
}

func (c *TunnelClient) process_client1(radis_id int, redis_key string, time_out time.Duration, send_data, recv_data []byte) quic.Connection {
	var redisJson RedisJsonType
	var conn *net.UDPConn

	c.m_process_stop = false
	c.m_process_chain = make(chan quic.Connection, 1)
	c.m_work_pool = workpool.NewWorkPool(10240)

	for {
		if res, err := gogo.Redis().GetDB(radis_id).Get(redis_key).Bytes(); err == nil && res != nil && len(res) > 0 {
			if err = json.Unmarshal(res, &redisJson); err == nil {
				if redisJson.ServerPort == 0 && redisJson.ClientPort == 0 { //等待服务器响应
					log.Println("等待服务器响应")
					goto NEXT_CHECK

				} else if redisJson.ServerPort > 0 && redisJson.ClientPort == 0 { //服务器已返回IPPORT
					log.Println("收到服务器返回的IPPORT")
					conn, err = net.ListenUDP("udp4", nil)
					tools.AssertErrorToNilf("main net.ListenUDP: %v", err)
					redisJson.ClientIP, redisJson.ClientPort = getWanIpPort(conn)
					if jsonByte, err := json.Marshal(redisJson); err == nil {
						gogo.Redis().Set(radis_id, redis_key, string(jsonByte), time_out)
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
			gogo.Redis().SetNx(radis_id, redis_key, string(jsonByte), time_out)
		}
	NEXT_CHECK:
		time.Sleep(1 * time.Second)
	}

	conn.Close()

	for i := 0; i <= 256 && !c.m_process_stop; i++ {
		c.process_client2(redisJson.ServerIP, redisJson.ServerPort, send_data, recv_data)
	}

	work_pool_end := make(chan int, 1)

	go func() {
		c.m_work_pool.Wait()
		work_pool_end <- 1
	}()

	select {
	case <-c.m_process_chain:
		return c.m_stun_quic_conn
	case <-time.After(time_out):
		c.m_process_stop = true
	case <-work_pool_end:
		break
	}

	return nil
}

func ProcessClient(m_cli_tun_local_addr, redis_addr, redis_pass string, radis_id int, redis_key string) quic.Connection {
	gogo.Redis().Init(&redis.Options{
		Addr:     redis_addr,
		Password: redis_pass,
		DB:       radis_id,
	})

	recv_data := make([]byte, 1600)
	send_data := []byte(tools.RandomString(9))

	for {
		var tunnelClient TunnelClient
		go proxy.ProcessProxyClient(m_cli_tun_local_addr, tunnelClient.process_client1(radis_id, redis_key, 15*time.Second, send_data, recv_data))
		process_health(tunnelClient.m_stun_health_stream, send_data, recv_data)
		tunnelClient.m_stun_quic_conn.CloseWithError(0, "0")
		time.Sleep(5 * time.Second)
	}
}

func (c *TunnelClient) GetQuicConn() quic.Connection {
	return c.m_stun_quic_conn
}
