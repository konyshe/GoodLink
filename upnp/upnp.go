package upnp

import (
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	httpClientTimeout   = 3 * time.Second
	ssdpSearchTimeout   = 3 * time.Second
	ssdpMX              = 2
	maxPortMappingIndex = 256
)

type Upnp struct {
	Active           bool         //这个upnp协议是否可用
	LocalHost        string       //本机ip地址
	GatewayOutsideIP string       //网关公网ip
	Gateway          *Gateway     //网关信息
	CtrlUrl          string       //控制请求url
	KeepPorts        map[int]bool //需要保留映射的端口号
	Description      string       //端口映射描述, 用于多服务互不删除
	lock             sync.Mutex
	http_client      *http.Client
}

func (this *Upnp) mappingDescription() string {
	if this != nil && this.Description != "" {
		return this.Description
	}
	return "goodlink"
}

func (this *Upnp) IsValid() bool {
	if this == nil || this.Gateway == nil {
		return false
	}
	return this.Gateway.ServiceType != "" && this.GatewayOutsideIP != "" && this.CtrlUrl != "" && this.LocalHost != ""
}

func (this *Upnp) deviceDesc() error {
	device := DeviceDesc{upnp: this}
	if !device.Send() {
		this.Active = false
		return errors.New("upnp: 获取设备描述失败")
	}
	if this.CtrlUrl == "" {
		this.Active = false
		return errors.New("upnp: 无控制URL")
	}
	this.Active = true
	return nil
}

func (this *Upnp) Init() error {
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.IsValid() {
		return nil
	}

	if this.KeepPorts == nil {
		this.KeepPorts = make(map[int]bool)
	}

	if this.http_client == nil {
		this.http_client = &http.Client{
			Timeout: httpClientTimeout,
		}
	}

	log.Println("upnp: 初始化中...")

	if this.LocalHost == "" {
		localHost, err := GetLocalInternetIp()
		if err != nil {
			this.Active = false
			return err
		}
		this.LocalHost = localHost
		log.Printf("upnp: LocalHost: %s", this.LocalHost)
	}

	if this.CtrlUrl == "" {
		searchGateway := SearchGateway{upnp: this}
		if !searchGateway.Send() {
			this.Active = false
			return errors.New("upnp: 未发现网关")
		}
		log.Printf("upnp: Gateway.ServiceType: %s", this.Gateway.ServiceType)

		if err := this.deviceDesc(); err != nil {
			return err
		}
		log.Printf("upnp: CtrlUrl: %s", this.CtrlUrl)
	}

	if this.GatewayOutsideIP == "" {
		eia := ExternalIPAddress{upnp: this}
		if !eia.Send() {
			this.Active = false
			return errors.New("upnp: 获取外网IP失败")
		}
		log.Printf("upnp: GatewayOutsideIP: %s", this.GatewayOutsideIP)
	}

	return nil
}

func (this *Upnp) AddPortMapping(localPort, remotePort int, protocol string) error {
	this.lock.Lock()
	defer this.lock.Unlock()

	if !this.IsValid() {
		return errors.New("upnp: 无Upnp设备")
	}

	addPort := AddPortMapping{upnp: this}
	if addPort.Send(localPort, remotePort, protocol) {
		return nil
	}

	this.Active = false
	return errors.New("upnp: 添加一个端口映射失败")
}

func (this *Upnp) DelPortMapping(lock bool, remotePort int, protocol string) error {
	if lock {
		this.lock.Lock()
		defer this.lock.Unlock()
	}

	if !this.IsValid() {
		return errors.New("upnp: 无Upnp设备")
	}

	delMapping := DelPortMapping{upnp: this}
	if delMapping.Send(remotePort, protocol) {
		return nil
	}
	return errors.New("upnp: 删除一个端口映射失败")
}

func (this *Upnp) AddKeepPort(port int) {
	this.lock.Lock()
	defer this.lock.Unlock()

	if this.KeepPorts == nil {
		this.KeepPorts = make(map[int]bool)
	}
	this.KeepPorts[port] = true
}

func (this *Upnp) RemoveKeepPort(port int) {
	this.lock.Lock()
	defer this.lock.Unlock()

	delete(this.KeepPorts, port)
}

// CleanMappings 清理之前添加的端口映射
// 通过枚举路由器上所有端口映射，筛选描述为 "goodlink" 的映射
// KeepPorts 中的端口映射将被保留
func (this *Upnp) CleanMappings() {
	this.lock.Lock()
	defer this.lock.Unlock()

	if !this.IsValid() {
		return
	}

	log.Println("upnp: CleanMappings start")

	toDelete := make([]PortMappingEntry, 0)
	getter := GetPortMappingEntry{upnp: this}

	goodlinkPortCount := 0
	index := 0
	for ; index < maxPortMappingIndex; index++ {
		entry, ok := getter.Send(index)
		if !ok {
			break
		}
		if entry.Description == this.mappingDescription() {
			goodlinkPortCount++
			if !this.KeepPorts[entry.ExternalPort] {
				toDelete = append(toDelete, *entry)
			}
		}
	}

	log.Printf("upnp: CleanMappings %d/%d/%d", len(toDelete), goodlinkPortCount, index)

	for _, entry := range toDelete {
		this.DelPortMapping(false, entry.ExternalPort, entry.Protocol)
	}

	log.Println("upnp: CleanMappings done")
}
