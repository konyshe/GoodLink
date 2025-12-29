package upnp

import (
	"log"
	"net"
	"strings"
	"time"
	// "net/http"
)

type Gateway struct {
	GatewayName   string //网关名称
	Host          string //网关ip和端口
	DeviceDescUrl string //网关设备描述路径
	Cache         string //cache
	ST            string
	USN           string
	deviceType    string //设备的urn   "urn:schemas-upnp-org:service:WANIPConnection:1"
	ControlURL    string //设备端口映射请求路径
	ServiceType   string //提供upnp服务的服务类型
}

type SearchGateway struct {
	upnp *Upnp
}

func (this *SearchGateway) buildRequest(serviceType string) string {
	return "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 6\r\n" +
		"ST: urn:schemas-upnp-org:service:" + serviceType + ":1\r\n\r\n"
}

func (this *SearchGateway) Send() bool {
	c := make(chan string)

	for _, serviceType := range []string{"WANPPPConnection", "WANIPConnection"} {
		go this.send("239.255.255.250:1900", this.buildRequest(serviceType), c)
	}

	result := <-c
	if result == "" {
		//超时了
		this.upnp.Active = false
		return this.upnp.Active
	}
	this.resolve(result)

	this.upnp.Gateway.ServiceType = this.upnp.Gateway.ST //"urn:schemas-upnp-org:service:WANIPConnection:1"
	this.upnp.Active = true
	return this.upnp.Active
}

func (this *SearchGateway) send(remoteAddr, searchMessage string, c chan string) {
	//发送组播消息，remoteAddr要带上端口，格式如："239.255.255.250:1900"
	var conn *net.UDPConn
	defer func() {
		if r := recover(); r != nil {
			//超时了
		}
	}()
	go func(conn *net.UDPConn) {
		defer func() {
			if r := recover(); r != nil {
				//没超时
			}
		}()
		//超时时间为3秒
		time.Sleep(time.Second * 3)
		c <- ""
		conn.Close()
	}(conn)
	remotAddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	if err != nil {
		log.Println("组播地址格式不正确")
	}
	locaAddr, err := net.ResolveUDPAddr("udp", this.upnp.LocalHost+":")

	if err != nil {
		log.Println("本地ip地址格式不正确")
	}
	conn, err = net.ListenUDP("udp", locaAddr)
	if conn == nil || err != nil {
		log.Println("监听udp出错")
	}
	defer conn.Close()

	_, err = conn.WriteToUDP([]byte(searchMessage), remotAddr)
	if err != nil {
		log.Println("发送msg到组播地址出错")
	}
	buf := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		log.Println("从组播地址接搜消息出错")
	}

	result := string(buf[:n])
	c <- result
}

func (this *SearchGateway) resolve(result string) {
	this.upnp.Gateway = &Gateway{}

	lines := strings.Split(result, "\r\n")
	for _, line := range lines {
		//按照第一个冒号分为两个字符串
		nameValues := strings.SplitAfterN(line, ":", 2)
		if len(nameValues) < 2 {
			continue
		}
		switch strings.ToUpper(strings.Trim(strings.Split(nameValues[0], ":")[0], " ")) {
		case "ST":
			this.upnp.Gateway.ST = nameValues[1]
		case "CACHE-CONTROL":
			this.upnp.Gateway.Cache = nameValues[1]
		case "LOCATION":
			urls := strings.Split(strings.Split(nameValues[1], "//")[1], "/")
			this.upnp.Gateway.Host = urls[0]
			log.Println("this.upnp.Gateway.Host:", this.upnp.Gateway.Host)
			this.upnp.Gateway.DeviceDescUrl = "/" + urls[1]
			log.Println("this.upnp.Gateway.DeviceDescUrl:", this.upnp.Gateway.DeviceDescUrl)
		case "SERVER":
			this.upnp.Gateway.GatewayName = nameValues[1]
		case "USN":
			this.upnp.Gateway.USN = nameValues[1]
			log.Println("this.upnp.Gateway.USN:", this.upnp.Gateway.USN)
		default:
		}
	}
}
