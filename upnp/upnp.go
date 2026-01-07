package upnp

import (
	"errors"
	"log"
	"sync"
)

type Upnp struct {
	Active             bool           //这个upnp协议是否可用
	LocalHost          string         //本机ip地址
	GatewayInsideIP    string         //局域网网关ip
	GatewayOutsideIP   string         //网关公网ip
	OutsideMappingPort map[string]int //映射外部端口
	InsideMappingPort  map[string]int //映射本机端口
	Gateway            *Gateway       //网关信息
	CtrlUrl            string         //控制请求url
	KeepPorts          map[int]bool   //需要保留映射的端口号
	lock               sync.Mutex
}

// 查看设备描述，得到控制请求url
func (this *Upnp) deviceDesc() (err error) {
	device := DeviceDesc{upnp: this}
	device.Send()
	this.Active = true
	return
}

func (this *Upnp) Init() (err error) {
	this.lock.Lock()
	defer this.lock.Unlock()

	this.KeepPorts = make(map[int]bool)

	defer func(err error) {
		if errTemp := recover(); errTemp != nil {
			log.Printf("upnp: %v", errTemp)
			err = errTemp.(error)
		}
	}(err)

	log.Println("upnp: 初始化中...")

	if this.LocalHost == "" {
		this.LocalHost = GetLocalIntenetIp()
		log.Printf("upnp: LocalHost: %s", this.LocalHost)
	}

	if this.CtrlUrl == "" {
		searchGateway := SearchGateway{upnp: this}
		if searchGateway.Send() {
			log.Printf("upnp: Gateway.ServiceType: %s", this.Gateway.ServiceType)
		}

		if err := this.deviceDesc(); err != nil {
			return err
		}
		log.Printf("upnp: CtrlUrl: %s", this.CtrlUrl)
	}

	if this.GatewayOutsideIP == "" {
		eia := ExternalIPAddress{upnp: this}
		eia.Send()
		log.Printf("upnp: GatewayOutsideIP: %s", this.GatewayOutsideIP)
	}

	return nil
}

// 添加一个端口映射
func (this *Upnp) AddPortMapping(localPort, remotePort int, protocol string) (err error) {
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.GatewayOutsideIP == "" {
		return errors.New("upnp: 无Upnp设备")
	}
	addPort := AddPortMapping{upnp: this, http_client: nil}
	if issuccess := addPort.Send(localPort, remotePort, protocol); issuccess {
		// log.Println("添加一个端口映射：protocol:", protocol, "local:", localPort, "remote:", remotePort)
		return nil
	} else {
		this.Active = false
		// log.Println("添加一个端口映射失败")
		return errors.New("upnp: 添加一个端口映射失败")
	}
}

func (this *Upnp) delPortMapping(remotePort int, protocol string) bool {
	if this.GatewayOutsideIP == "" {
		return false
	}
	delMapping := DelPortMapping{upnp: this, http_client: nil}
	issuccess := delMapping.Send(remotePort, protocol)
	if issuccess {
		//log.Println("删除了一个端口映射： remote:", remotePort)
	}
	return issuccess
}

func (this *Upnp) AddKeepPort(port int) {
	this.lock.Lock()
	defer this.lock.Unlock()

	this.KeepPorts[port] = true
}

func (this *Upnp) RemoveKeepPort(port int) {
	this.lock.Lock()
	defer this.lock.Unlock()

	delete(this.KeepPorts, port)
}

// CleanMappings 清理之前添加的端口映射
// 通过枚举路由器上所有端口映射，筛选描述为 "goodlink" 的映射
// keepPorts 为需要保留的端口号 map，不在该 map 中的端口映射将被删除
func (this *Upnp) CleanMappings() error {
	this.lock.Lock()
	defer this.lock.Unlock()

	log.Println("upnp: CleanMappings")

	if this.GatewayOutsideIP == "" {
		return errors.New("upnp: 无Upnp设备")
	}

	// 先收集所有需要删除的映射
	toDelete := make([]PortMappingEntry, 0)
	getter := GetPortMappingEntry{upnp: this}

	index := 0
	for ; ; index++ {
		entry, ok := getter.Send(index)
		if !ok {
			// 没有更多映射了
			break
		}
		// 如果是 goodlink 的映射，且端口号不在保留列表中，则标记为删除
		if entry.Description == "goodlink" && !this.KeepPorts[entry.ExternalPort] {
			toDelete = append(toDelete, *entry)
		}
	}

	log.Printf("upnp: CleanMappings %d/%d", len(toDelete), index)

	// 删除所有标记为删除的映射
	for _, entry := range toDelete {
		this.delPortMapping(entry.ExternalPort, entry.Protocol)
	}

	log.Println("upnp: CleanMappings done")

	return nil
}
