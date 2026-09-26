package upnp

import (
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	httpClientTimeout   = 3 * time.Second
	ssdpSearchTimeout   = 3 * time.Second
	ssdpMX              = 2
	maxPortMappingIndex = 256

	// cleanMappingsTimeout 是单轮 CleanMappings 的整体时间上限,
	// 保证清理即使异常也不会长时间占用 Upnp.lock
	cleanMappingsTimeout = 20 * time.Second
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

// normalizeProtocol 把协议名归一化为 SOAP 规范要求的大写形式并限定取值,
// 无法识别时返回空串, 由调用方跳过而不是把非法值发往路由器。
// 部分调用方传入小写 "udp", 路由器会返回 402, 导致删除永久失败且被泛化错误信息掩盖。
func normalizeProtocol(protocol string) string {
	switch strings.ToUpper(strings.TrimSpace(protocol)) {
	case "TCP":
		return "TCP"
	case "UDP":
		return "UDP"
	default:
		return ""
	}
}

// validPort 端口必须落在 1..65535 之间
func validPort(port int) bool {
	return port > 0 && port <= 65535
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

	// 每条请求最多耗时 httpClientTimeout, 最坏情况下 256 条叠加会阻塞十几分钟。
	// 调用方往往持有其它锁(如 TUN 会话锁), 因此这里必须有整体上界。
	deadline := time.Now().Add(cleanMappingsTimeout)

	toDelete := make([]PortMappingEntry, 0)
	getter := GetPortMappingEntry{upnp: this}

	goodlinkPortCount := 0
	index := 0
	for ; index < maxPortMappingIndex; index++ {
		if time.Now().After(deadline) {
			log.Printf("upnp: CleanMappings 枚举超时, 已检查%d条", index)
			break
		}

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

	failCount := 0
	for i, entry := range toDelete {
		if time.Now().After(deadline) {
			failCount = failCount + (len(toDelete) - i)
			log.Printf("upnp: CleanMappings 删除超时, 剩余%d条未处理", len(toDelete)-i)
			break
		}

		proto := normalizeProtocol(entry.Protocol)
		if proto == "" || !validPort(entry.ExternalPort) {
			failCount++
			log.Printf("upnp: 端口映射条目非法, 跳过删除: port=%d proto=%s", entry.ExternalPort, entry.Protocol)
			continue
		}

		// 此处持锁, 必须走 DelPortMapping(false, ...) 的免加锁分支避免重入
		if err := this.DelPortMapping(false, entry.ExternalPort, proto); err != nil {
			failCount++
			log.Printf("upnp: 删除端口映射失败: port=%d proto=%s err=%v", entry.ExternalPort, proto, err)
		}
	}
	if failCount > 0 {
		log.Printf("upnp: CleanMappings 有%d条未能删除", failCount)
	}

	log.Println("upnp: CleanMappings done")
}
