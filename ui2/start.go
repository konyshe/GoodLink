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
	gen         uint64 // 每次成功 Start 递增; waiter 只处理自己那一代
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
		// 不在这里 Wait: 各代 waiter 负责回收自己的 Cmd, 避免双重 Wait
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
	// 退出兜底: 上面只删了"进程已死"和"镜像匹配"的文件, 这里把本程序剩下的 pid 文件清干净
	utils.RemoveAllWorkerPids()
}

func stopServiceProcess(id string) {
	procMu.Lock()
	p := procOf(id)
	p.autoRestart = false
	cmd := p.cmd
	p.cmd = nil
	p.started = false
	// 在锁内删: 与 startCmdProcess 的"写文件 + gen++"互斥, 避免删掉刚启动的新一代文件
	utils.RemoveWorkerPid(id)
	procMu.Unlock()
	if cmd != nil && cmd.Process != nil {
		utils.KillProcess(cmd.Process.Pid)
	}
	// Wait 由对应代数的 waiter 完成, 这里只杀进程
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
		msg := "两端版本不兼容"
		if status == pro.StatusTransportMismatch {
			msg = "两端传输协议不一致"
		}
		ServiceLogPrintF(id, "服务[%s] %s，已停止连接", name, msg)
		go func() {
			time.Sleep(500 * time.Millisecond)
			stopServiceProcess(id)
		}()
	case pro.StatusNeedAdmin:
		disableAutoRestart(id)
		ServiceLogPrintF(id, "服务[%s] %s", name, utils.NeedAdminRestartMsg)
		go func() {
			time.Sleep(500 * time.Millisecond)
			stopServiceProcess(id)
		}()
	}
}

func disableAutoRestart(id string) {
	procMu.Lock()
	procOf(id).autoRestart = false
	procMu.Unlock()
}

// startCmdProcess 启动子进程并返回该代的 cmd/gen, 供 waiter 精确 Wait。
func startCmdProcess(id string) (cmd *exec.Cmd, gen uint64, err error) {
	cmdPath := utils.GetCmdExePath()
	args := []string{"--ui", "--service-id=" + id}
	cmd = exec.Command(cmdPath, args...)
	cmd.Env = append(os.Environ(), config.ExportRuntimeConfEnv(runtimeConf))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	procMu.Lock()
	p := procOf(id)
	p.gen++
	gen = p.gen
	p.cmd = cmd
	p.started = true
	if cmd.Process != nil {
		// 必须与 gen++ 在同一临界区内: waiter 靠 gen 判断"没有更代"后才会删 pid 文件,
		// 若写文件在锁外, 新代写完文件、旧 waiter 仍判 mine 为真, 会把新 pid 文件删掉。
		utils.WriteWorkerPid(id, cmd.Process.Pid)
	}
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
	return cmd, gen, nil
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

func HandleStart(id, workType, tunKey, localMode, transport string, rules []config.UIForwardRule, allowLocalProxy *bool) error {
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

	// 先拼候选配置并做校验, 通过后再写入内存, 避免密钥重复等错误留下脏状态
	stateMu.RLock()
	cur, ok := lookupService(id)
	if !ok {
		stateMu.RUnlock()
		return fmt.Errorf("服务不存在")
	}
	prev := cur.cfg
	stateMu.RUnlock()

	next := prev
	next.WorkType = strings.TrimSpace(workType)
	if next.WorkType != workTypeLocal && next.WorkType != workTypeRemote {
		next.WorkType = workTypeLocal
	}
	if strings.TrimSpace(tunKey) != "" {
		next.TunKey = strings.TrimSpace(tunKey)
	}
	next.LocalMode = localMode
	next.Transport = transport
	next.ForwardRules = rules
	if allowLocalProxy != nil {
		next.AllowLocalProxy = allowLocalProxy
	}

	if other := duplicateKey(id, next.TunKey); other != "" {
		return fmt.Errorf("连接密钥与服务[%s]重复, 请先修改密钥", other)
	}
	if next.WorkType == workTypeLocal && next.LocalMode == config.LocalModeForward {
		if conflicts := config.ListenConflicts(rules, nil); len(conflicts) > 0 {
			return fmt.Errorf("%s", config.FormatListenConflicts(conflicts))
		}
	}
	if next.WorkType == workTypeLocal && next.LocalMode == config.LocalModeTUN && !utils.IsAdmin() {
		return fmt.Errorf("%s", utils.NeedAdminRestartMsg)
	}

	err := mutateService(id, func(svc *serviceState) error {
		svc.cfg.WorkType = next.WorkType
		svc.cfg.TunKey = next.TunKey
		svc.cfg.LocalMode = next.LocalMode
		svc.cfg.Transport = next.Transport
		svc.cfg.ForwardRules = next.ForwardRules
		svc.cfg.AllowLocalProxy = next.AllowLocalProxy
		return nil
	})
	if err != nil {
		return err
	}
	cfg := allServicesConfig()
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		_ = mutateService(id, func(svc *serviceState) error {
			svc.cfg = prev
			return nil
		})
		return err
	}
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		_ = mutateService(id, func(svc *serviceState) error {
			svc.cfg = prev
			return nil
		})
		return err
	}
	_ = mutateService(id, func(svc *serviceState) error {
		if found, ok := config.FindService(cfg, id); ok {
			svc.cfg = *found
		}
		return nil
	})

	wt := next.WorkType
	resetProxyListen(id)
	if wt == workTypeLocal {
		setServiceStatus(id, StatusConnecting)
	} else {
		setServiceStatus(id, StatusStarting)
	}
	procMu.Lock()
	procOf(id).autoRestart = true
	procMu.Unlock()

	cmd, gen, err := startCmdProcess(id)
	if err != nil {
		ServiceLogPrintF(id, "服务[%s] 启动失败: %v", serviceName(id), err)
		procMu.Lock()
		procOf(id).started = false
		procOf(id).autoRestart = false
		procMu.Unlock()
		resetProxyListen(id)
		setServiceStatus(id, StatusIdle)
		return err
	}
	go waitForProcessAndHandleExit(id, cmd, gen)
	return nil
}

func HandleStop(id string) {
	procMu.Lock()
	started := procOf(id).started
	procMu.Unlock()
	if !started {
		// 进程已退出但状态仍停在 NAT4/连接失败等终端态时, 允许点停止清回 Idle
		st := getServiceStatus(id)
		if st == StatusConnectFailed || st == StatusConnectingNAT4 {
			resetProxyListen(id)
			setServiceStatus(id, StatusIdle)
		}
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

// waitForProcessAndHandleExit 只 Wait 启动时拿到的那份 cmd。
// 代数不匹配说明已有更新的一代在跑, 本 waiter 只负责回收僵尸, 不改状态、不重启。
func waitForProcessAndHandleExit(id string, cmd *exec.Cmd, gen uint64) {
	if cmd != nil {
		_ = cmd.Wait()
	}
	procMu.Lock()
	p := procOf(id)
	mine := p.gen == gen
	abnormal := mine && p.started && p.autoRestart
	if mine {
		p.started = false
		if p.cmd == cmd {
			p.cmd = nil
		}
		// 子进程已经退出, pid 文件不再有意义, 在锁内删以与 startCmdProcess 的
		// "写文件 + gen++"互斥, 保证删的是自己这一代的文件。
		// 顺序上必须先于 autoRestartProcess: 自动重启会重新写一份, 先删后写才不留旧进程号。
		utils.RemoveWorkerPid(id)
	}
	procMu.Unlock()
	if !mine {
		return
	}
	if abnormal {
		autoRestartProcess(id)
		return
	}
	resetProxyListen(id)
	st := getServiceStatus(id)
	if st != StatusConnectFailed && st != StatusConnectingNAT4 && st != StatusStopping {
		setServiceStatus(id, StatusIdle)
	}
}

func autoRestartProcess(id string) {
	time.Sleep(500 * time.Millisecond)
	// 与 HandleStart 串行, 避免崩溃后 500ms 内用户手动启动又叠一层自动重启
	startMu.Lock()
	defer startMu.Unlock()

	procMu.Lock()
	p := procOf(id)
	ok := p.autoRestart && !p.started
	procMu.Unlock()
	if !ok {
		return
	}
	ServiceLogPrintF(id, "检测到服务[%s]进程异常退出，正在自动重启...", serviceName(id))
	resetProxyListen(id)
	cmd, gen, err := startCmdProcess(id)
	if err != nil {
		ServiceLogPrintF(id, "服务[%s] 自动重启失败: %v", serviceName(id), err)
		procMu.Lock()
		procOf(id).started = false
		procOf(id).autoRestart = false
		procMu.Unlock()
		setServiceStatus(id, StatusIdle)
		return
	}
	go waitForProcessAndHandleExit(id, cmd, gen)
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
		var allowLocalProxy *bool
		if svc != nil {
			wt = svc.cfg.WorkType
			key = svc.cfg.TunKey
			mode = svc.cfg.LocalMode
			transport = svc.cfg.Transport
			rules = append([]config.UIForwardRule(nil), svc.cfg.ForwardRules...)
			allowLocalProxy = svc.cfg.AllowLocalProxy
		}
		stateMu.RUnlock()
		if err := HandleStart(id, wt, key, mode, transport, rules, allowLocalProxy); err != nil {
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

func HandleSetAutoStart(id string, autoStart bool, workType, tunKey, localMode, transport string, rules []config.UIForwardRule, allowLocalProxy *bool) error {
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
			if allowLocalProxy != nil {
				svc.cfg.AllowLocalProxy = allowLocalProxy
			}
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
	removed, ok := lookupService(id)
	if !ok {
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
	stateMu.Unlock()

	// 注意: saveAllConfig 内部会取 stateMu 读锁, 因此必须在释放写锁之后调用
	if err := saveAllConfig(); err != nil {
		// 保存失败则回滚内存, 避免内存已删除而磁盘未变导致重启后"复活"
		stateMu.Lock()
		m_services = append(m_services, removed)
		if _, exist := m_index[id]; !exist {
			m_index[id] = removed
		}
		stateMu.Unlock()
		return err
	}

	// procs 由 procMu 保护, 在 stateMu 下 delete 会与其它 goroutine 的 procOf 形成并发读写 map
	procMu.Lock()
	delete(procs, id)
	procMu.Unlock()

	// 服务已经不存在了, 它的 pid 文件不该继续留在磁盘上
	utils.RemoveWorkerPid(id)

	dropServiceLogs(id)
	broadcastState()
	return nil
}

func HandleUpdateConfig(id, workType, tunKey, localMode, transport string, rules []config.UIForwardRule, autoStart bool, allowLocalProxy *bool) error {
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
		if allowLocalProxy != nil {
			svc.cfg.AllowLocalProxy = allowLocalProxy
		}
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
