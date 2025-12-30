package upnp

import (
	// "fmt"
	"errors"
	"log"
	"sync"
)

/*
 * 得到网关
 */

// 对所有的端口进行管理
type MappingPortStruct struct {
	lock *sync.Mutex
}

type Upnp struct {
	Active             bool              //这个upnp协议是否可用
	LocalHost          string            //本机ip地址
	GatewayInsideIP    string            //局域网网关ip
	GatewayOutsideIP   string            //网关公网ip
	OutsideMappingPort map[string]int    //映射外部端口
	InsideMappingPort  map[string]int    //映射本机端口
	Gateway            *Gateway          //网关信息
	CtrlUrl            string            //控制请求url
	MappingPort        MappingPortStruct //已经添加了的映射 {"TCP":[1990],"UDP":[1991]}
}

// 得到本地联网的ip地址
// 得到局域网网关ip
func (this *Upnp) SearchGateway() (err error) {
	if this.LocalHost == "" {
		this.MappingPort = MappingPortStruct{
			lock: new(sync.Mutex),
			// mappingPorts: map[string][][]int{},
		}
		this.LocalHost = GetLocalIntenetIp()
	}
	searchGateway := SearchGateway{upnp: this}
	if searchGateway.Send() {
		return nil
	}
	return errors.New("未发现网关设备")
}

func (this *Upnp) deviceStatus() {

}

// 查看设备描述，得到控制请求url
func (this *Upnp) deviceDesc() (err error) {
	device := DeviceDesc{upnp: this}
	device.Send()
	this.Active = true
	return
}

// 查看公网ip地址
func (this *Upnp) ExternalIPAddr() (err error) {
	if this.GatewayOutsideIP != "" {
		return nil
	}
	if this.GatewayInsideIP == "" {
		if err := this.SearchGateway(); err != nil {
			return err
		}
	}
	if this.CtrlUrl == "" {
		if err := this.deviceDesc(); err != nil {
			return err
		}
		log.Println("获得控制请求url:", this.CtrlUrl)
	}
	eia := ExternalIPAddress{upnp: this}
	eia.Send()
	log.Println("获得公网ip地址为：", this.GatewayOutsideIP)

	return nil
}

func (this *Upnp) Init() (err error) {
	defer func(err error) {
		if errTemp := recover(); errTemp != nil {
			log.Println("upnp模块报错了", errTemp)
			err = errTemp.(error)
		}
	}(err)

	log.Println("upnp模块初始化中")

	if err := this.ExternalIPAddr(); err != nil {
		return err
	}

	if err := this.CleanMappings(0); err != nil {
		return err
	}

	return nil
}

// 添加一个端口映射
func (this *Upnp) AddPortMapping(localPort, remotePort int, protocol string) (err error) {
	if this.GatewayOutsideIP == "" {
		return errors.New("无Upnp设备")
	}
	addPort := AddPortMapping{upnp: this}
	if issuccess := addPort.Send(localPort, remotePort, protocol); issuccess {
		// log.Println("添加一个端口映射：protocol:", protocol, "local:", localPort, "remote:", remotePort)
		return nil
	} else {
		this.Active = false
		// log.Println("添加一个端口映射失败")
		return errors.New("添加一个端口映射失败")
	}
}

func (this *Upnp) DelPortMapping(remotePort int, protocol string) bool {
	if this.GatewayOutsideIP == "" {
		return false
	}
	delMapping := DelPortMapping{upnp: this}
	issuccess := delMapping.Send(remotePort, protocol)
	if issuccess {
		//log.Println("删除了一个端口映射： remote:", remotePort)
	}
	return issuccess
}

// CleanMappings 清理之前添加的端口映射
// 通过枚举路由器上所有端口映射，筛选描述为 "goodlink" 的映射并删除
func (this *Upnp) CleanMappings(externalPort int) error {
	if this.GatewayOutsideIP == "" {
		return errors.New("无Upnp设备")
	}

	// 先收集所有需要删除的映射
	toDelete := make([]PortMappingEntry, 0)
	getter := GetPortMappingEntry{upnp: this}

	for index := 0; ; index++ {
		entry, ok := getter.Send(index)
		if !ok {
			// 没有更多映射了
			break
		}
		if entry.Description == "goodlink" && entry.ExternalPort != externalPort {
			toDelete = append(toDelete, *entry)
		}
	}

	// 删除所有 goodlink 的映射
	for _, entry := range toDelete {
		this.DelPortMapping(entry.ExternalPort, entry.Protocol)
	}

	return nil
}
