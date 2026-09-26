package ui2

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"goodlink/config"
	"goodlink/pro"
	"goodlink/utils"
)

type svcProc struct {
	cmd         *exec.Cmd
	started     bool
	autoRestart bool
}

var (
	procMu  sync.Mutex
	procs   = map[string]*svcProc{}
	startMu sync.Mutex
)

func procOf(id string) *svcProc {
	p, ok := procs[id]
	if !ok {
		p = &svcProc{}
		procs[id] = p
	}
	return p
}

func StopCmdProcess() {
	procMu.Lock()
	ids := make([]string, 0, len(procs))
	for id, p := range procs {
		if p.cmd != nil && p.cmd.Process != nil {
			utils.KillProcess(p.cmd.Process.Pid)
		}
		p.cmd = nil
		p.started = false
		p.autoRestart = false
		ids = append(ids, id)
	}
	procMu.Unlock()
	for _, id := range ids {
		resetProxyListen(id)
		setServiceStatus(id, StatusIdle)
	}
	utils.CleanupOrphanedCmdProcesses()
}

func stopServiceProcess(id string) {
	procMu.Lock()
	p := procOf(id)
	p.autoRestart = false
	cmd := p.cmd
	p.cmd = nil
	p.started = false
	procMu.Unlock()
	if cmd != nil && cmd.Process != nil {
		utils.KillProcess(cmd.Process.Pid)
	}
	utils.RemoveWorkerPid(id)
}

func parseStatusMessage(line string) (int, bool) {
	idx := strings.Index(line, pro.TagStatusPrefix)
	if idx == -1 {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[idx+len(pro.TagStatusPrefix):]))
	if err != nil {
		return 0, false
	}
	switch n {
	case pro.StatusConnecting, pro.StatusConnectingNAT4, pro.StatusConnected, pro.StatusRunning,
		pro.StatusVersionMismatch, pro.StatusTransportMismatch, pro.StatusNeedAdmin, pro.StatusConnectFailed:
		return n, true
	}
	return 0, false
}

func parseProxyMessage(line string) (string, bool) {
	idx := strings.Index(line, pro.TagProxyPrefix)
	if idx == -1 {
		return "", false
	}
	addr := strings.TrimSpace(line[idx+len(pro.TagProxyPrefix):])
	if addr == "" {
		return "", false
	}
	return addr, true
}

func updateConnectionStatus(id string, status int) {
	name := serviceName(id)
	switch status {
	case pro.StatusConnecting:
		setServiceStatus(id, StatusConnecting)
	case pro.StatusConnected:
		setServiceStatus(id, StatusConnected)
	case pro.StatusConnectingNAT4:
		setServiceStatus(id, StatusConnectingNAT4)
		disableAutoRestart(id)
		ServiceLogPrintF(id, "服务[%s] 两端网络都是NAT4，无法直连，已停止连接", name)
		go func() {
			time.Sleep(500 * time.Millisecond)
			stopServiceProcess(id)
		}()
	case pro.StatusConnectFailed:
		setServiceStatus(id, StatusConnectFailed)
		disableAutoRestart(id)
		ServiceLogPrintF(id, "服务[%s] 已尝试15次仍无法连接，已停止", name)
		go func() {
			time.Sleep(500 * time.Millisecond)
			stopServiceProcess(id)
		}()
	case pro.StatusRunning:
		setServiceStatus(id, StatusRunning)
	case pro.StatusVersionMismatch, pro.StatusTransportMismatch:
		disableAutoRestart(id)
		go func() {
			time.Sleep(500 * time.Millisecond)
			stopServiceProcess(id)
		}()
	case pro.StatusNeedAdmin:
		disableAutoRestart(id)
		ServiceLogPrintF(id, "服务[%s] %s", name, utils.NeedAdminRestartMsg)
		setServiceStatus(id, StatusIdle)
	}
}

func disableAutoRestart(id string) {
	procMu.Lock()
	procOf(id).autoRestart = false
	procMu.Unlock()
}

func startCmdProcess(id string) error {
	cmdPath := utils.GetCmdExePath()
	args := []string{"--ui", "--service-id=" + id}
	cmd := exec.Command(cmdPath, args...)
	cmd.Env = append(os.Environ(), config.ExportRuntimeConfEnv(runtimeConf))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	if cmd.Process != nil {
		utils.WriteWorkerPid(id, cmd.Process.Pid)
	}
	procMu.Lock()
	p := procOf(id)
	p.cmd = cmd
	p.started = true
	procMu.Unlock()

	handle := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			line := scanner.Text()
			if addr, ok := parseProxyMessage(line); ok {
				setProxyListen(id, addr)
				continue
			}
			if status, ok := parseStatusMessage(line); ok {
				updateConnectionStatus(id, status)
			} else {
				ServiceLogPrintF(id, line)
			}
		}
	}
	go handle(bufio.NewScanner(stdout))
	go handle(bufio.NewScanner(stderr))
	return nil
}

func saveAllConfig() error {
	return config.SaveUIConfig(config.UIConfigPath(), allServicesConfig())
}

func duplicateKey(id, tunKey string) string {
	tunKey = strings.TrimSpace(tunKey)
	if tunKey == "" {
		return ""
	}
	stateMu.RLock()
	defer stateMu.RUnlock()
	for _, svc := range m_services {
		if svc.cfg.ID == id {
			continue
		}
		if strings.TrimSpace(svc.cfg.TunKey) == tunKey {
			return svc.cfg.Name
		}
	}
	return ""
}

func HandleStart(id, workType, tunKey, localMode, transport string, rules []config.UIForwardRule) error {
	startMu.Lock()
	defer startMu.Unlock()

	procMu.Lock()
	started := procOf(id).started
	procMu.Unlock()
	if started {
		return fmt.Errorf("already started")
	}
	st := getServiceStatus(id)
	if st == StatusDetecting || st == StatusStopping {
		return fmt.Errorf("busy")
	}

	var svcCopy config.UIService
	err := mutateService(id, func(svc *serviceState) error {
		svc.cfg.WorkType = strings.TrimSpace(workType)
		if svc.cfg.WorkType != workTypeLocal && svc.cfg.WorkType != workTypeRemote {
			svc.cfg.WorkType = workTypeLocal
		}
		if strings.TrimSpace(tunKey) != "" {
			svc.cfg.TunKey = strings.TrimSpace(tunKey)
		}
		svc.cfg.LocalMode = localMode
		svc.cfg.Transport = transport
		svc.cfg.ForwardRules = rules
		svcCopy = svc.cfg
		return nil
	})
	if err != nil {
		return err
	}
	if other := duplicateKey(id, svcCopy.TunKey); other != "" {
		return fmt.Errorf("连接密钥与服务[%s]重复, 请先修改密钥", other)
	}
	cfg := allServicesConfig()
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		return err
	}
	_ = mutateService(id, func(svc *serviceState) error {
		if found, ok := config.FindService(cfg, id); ok {
			svc.cfg = *found
		}
		return nil
	})

	stateMu.RLock()
	cur, ok := lookupService(id)
	var mode, wt string
	if ok {
		mode = cur.cfg.LocalMode
		wt = cur.cfg.WorkType
	}
	stateMu.RUnlock()
	if !ok {
		return fmt.Errorf("服务不存在")
	}
	if wt == workTypeLocal && mode == config.LocalModeForward {
		if conflicts := config.ListenConflicts(rules, nil); len(conflicts) > 0 {
			return fmt.Errorf("%s", config.FormatListenConflicts(conflicts))
		}
	}
	if wt == workTypeLocal && mode == config.LocalModeTUN && !utils.IsAdmin() {
		return fmt.Errorf("%s", utils.NeedAdminRestartMsg)
	}

	resetProxyListen(id)
	if wt == workTypeLocal {
		setServiceStatus(id, StatusConnecting)
	} else {
		setServiceStatus(id, StatusStarting)
	}
	procMu.Lock()
	procOf(id).autoRestart = true
	procMu.Unlock()

	if err := startCmdProcess(id); err != nil {
		ServiceLogPrintF(id, "服务[%s] 启动失败: %v", serviceName(id), err)
		procMu.Lock()
		procOf(id).started = false
		procOf(id).autoRestart = false
		procMu.Unlock()
		resetProxyListen(id)
		setServiceStatus(id, StatusIdle)
		return err
	}
	go waitForProcessAndHandleExit(id)
	return nil
}

func HandleStop(id string) {
	procMu.Lock()
	started := procOf(id).started
	procMu.Unlock()
	if !started {
		return
	}
	setServiceStatus(id, StatusStopping)
	procMu.Lock()
	procOf(id).autoRestart = false
	procMu.Unlock()
	go func() {
		stopServiceProcess(id)
		resetProxyListen(id)
		setServiceStatus(id, StatusIdle)
	}()
}

func waitForProcessAndHandleExit(id string) {
	time.Sleep(time.Second)
	procMu.Lock()
	p := procOf(id)
	if !p.started {
		procMu.Unlock()
		return
	}
	cmd := p.cmd
	procMu.Unlock()
	if cmd != nil {
		_ = cmd.Wait()
	}
	procMu.Lock()
	p = procOf(id)
	abnormal := p.started && p.autoRestart
	procMu.Unlock()
	if abnormal {
		autoRestartProcess(id)
		return
	}
	procMu.Lock()
	procOf(id).started = false
	procMu.Unlock()
	resetProxyListen(id)
	if getServiceStatus(id) != StatusConnectFailed && getServiceStatus(id) != StatusConnectingNAT4 {
		setServiceStatus(id, StatusIdle)
	}
}

func autoRestartProcess(id string) {
	time.Sleep(500 * time.Millisecond)
	procMu.Lock()
	p := procOf(id)
	ok := p.started && p.autoRestart
	procMu.Unlock()
	if !ok {
		return
	}
	ServiceLogPrintF(id, "检测到服务[%s]进程异常退出，正在自动重启...", serviceName(id))
	resetProxyListen(id)
	if err := startCmdProcess(id); err != nil {
		ServiceLogPrintF(id, "服务[%s] 自动重启失败: %v", serviceName(id), err)
		procMu.Lock()
		procOf(id).started = false
		procOf(id).autoRestart = false
		procMu.Unlock()
		setServiceStatus(id, StatusIdle)
		return
	}
	go waitForProcessAndHandleExit(id)
}

func tryAutoStart() {
	stateMu.RLock()
	var ids []string
	for _, svc := range m_services {
		if svc.cfg.AutoStart {
			ids = append(ids, svc.cfg.ID)
		}
	}
	stateMu.RUnlock()
	for _, id := range ids {
		name := serviceName(id)
		ServiceLogPrintF(id, "已配置自动连接, 正在启动服务[%s]...", name)
		stateMu.RLock()
		svc := m_index[id]
		var wt, key, mode, transport string
		var rules []config.UIForwardRule
		if svc != nil {
			wt = svc.cfg.WorkType
			key = svc.cfg.TunKey
			mode = svc.cfg.LocalMode
			transport = svc.cfg.Transport
			rules = append([]config.UIForwardRule(nil), svc.cfg.ForwardRules...)
		}
		stateMu.RUnlock()
		if err := HandleStart(id, wt, key, mode, transport, rules); err != nil {
			ServiceLogPrintF(id, "服务[%s] 自动启动失败: %v", name, err)
		}
	}
}

func HandleForwards(id, localMode string, rules []config.UIForwardRule) error {
	err := mutateService(id, func(svc *serviceState) error {
		if !serviceIdle(svc) && svc.status != StatusConnected && svc.status != StatusConnecting && svc.status != StatusRunning && svc.status != StatusStarting {
			return fmt.Errorf("busy")
		}
		svc.cfg.LocalMode = localMode
		svc.cfg.ForwardRules = rules
		return nil
	})
	if err != nil {
		return err
	}
	cfg := allServicesConfig()
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	return config.SaveUIConfig(config.UIConfigPath(), cfg)
}

// clearOtherAutoStart 关掉除 keepID 以外的自启动。调用方必须已持有 stateMu。
func clearOtherAutoStart(keepID string) {
	for _, other := range m_services {
		if other.cfg.ID != keepID {
			other.cfg.AutoStart = false
		}
	}
}

func HandleSetAutoStart(id string, autoStart bool, workType, tunKey, localMode, transport string, rules []config.UIForwardRule) error {
	err := mutateService(id, func(svc *serviceState) error {
		svc.cfg.AutoStart = autoStart
		if autoStart {
			clearOtherAutoStart(id)
		}
		if serviceIdle(svc) {
			if workType == workTypeLocal || workType == workTypeRemote {
				svc.cfg.WorkType = workType
			}
			if strings.TrimSpace(tunKey) != "" {
				svc.cfg.TunKey = strings.TrimSpace(tunKey)
			}
			svc.cfg.LocalMode = localMode
			svc.cfg.Transport = transport
			svc.cfg.ForwardRules = rules
		}
		return nil
	})
	if err != nil {
		return err
	}
	cfg := allServicesConfig()
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		return err
	}
	broadcastState()
	if autoStart {
		ServiceLogPrintF(id, "已开启自动连接, 程序下次启动时自动连接服务[%s]", serviceName(id))
	}
	return nil
}

func HandleRename(id, name string) error {
	err := mutateService(id, func(svc *serviceState) error {
		svc.cfg.Name = name
		return nil
	})
	if err != nil {
		return err
	}
	cfg := allServicesConfig()
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		return err
	}
	_ = mutateService(id, func(svc *serviceState) error {
		if found, ok := config.FindService(cfg, id); ok {
			svc.cfg.Name = found.Name
		}
		return nil
	})
	broadcastState()
	ServiceLogPrintF(id, "服务已改名为[%s]", serviceName(id))
	return nil
}

func HandleCreateService() (ServiceView, error) {
	startMu.Lock()
	defer startMu.Unlock()
	cfg := allServicesConfig()
	svc := config.UIService{
		Name:         config.AllocateServiceName(cfg),
		WorkType:     workTypeLocal,
		LocalMode:    config.LocalModeTUN,
		Transport:    config.TransportKCP,
		TunIP:        config.AllocateTunIP(cfg),
		ForwardRules: []config.UIForwardRule{},
	}
	if svc.TunIP == "" {
		return ServiceView{}, fmt.Errorf("没有可用的网段")
	}
	cfg.Services = append(cfg.Services, svc)
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return ServiceView{}, err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		return ServiceView{}, err
	}
	created := cfg.Services[len(cfg.Services)-1]
	stateMu.Lock()
	st := &serviceState{cfg: created, status: StatusIdle}
	m_services = append(m_services, st)
	m_index[created.ID] = st
	view := serviceViewLocked(st, false)
	stateMu.Unlock()
	broadcastState()
	return view, nil
}

func HandleDeleteService(id string) error {
	procMu.Lock()
	started := false
	if p, ok := procs[id]; ok {
		started = p.started
	}
	procMu.Unlock()
	if started || (getServiceStatus(id) != StatusIdle && getServiceStatus(id) != StatusDetecting && getServiceStatus(id) != StatusConnectFailed && getServiceStatus(id) != StatusConnectingNAT4) {
		return fmt.Errorf("请先停止服务[%s]", serviceName(id))
	}
	stateMu.Lock()
	if _, ok := lookupService(id); !ok {
		stateMu.Unlock()
		return fmt.Errorf("服务不存在")
	}
	next := make([]*serviceState, 0, len(m_services)-1)
	for _, svc := range m_services {
		if svc.cfg.ID == id {
			continue
		}
		next = append(next, svc)
	}
	m_services = next
	delete(m_index, id)
	delete(procs, id)
	stateMu.Unlock()
	dropServiceLogs(id)
	if err := saveAllConfig(); err != nil {
		return err
	}
	broadcastState()
	return nil
}

func HandleUpdateConfig(id, workType, tunKey, localMode, transport string, rules []config.UIForwardRule, autoStart bool) error {
	err := mutateService(id, func(svc *serviceState) error {
		if !serviceIdle(svc) {
			return fmt.Errorf("busy")
		}
		if workType == workTypeLocal || workType == workTypeRemote {
			svc.cfg.WorkType = workType
		}
		if strings.TrimSpace(tunKey) != "" {
			svc.cfg.TunKey = strings.TrimSpace(tunKey)
		}
		svc.cfg.LocalMode = localMode
		svc.cfg.Transport = transport
		svc.cfg.ForwardRules = rules
		svc.cfg.AutoStart = autoStart
		if autoStart {
			clearOtherAutoStart(id)
		}
		return nil
	})
	if err != nil {
		return err
	}
	cfg := allServicesConfig()
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		return err
	}
	broadcastState()
	return nil
}
