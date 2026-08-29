package ui2

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"goodlink/config"
	"goodlink/pro"
	"goodlink/utils"
)

var (
	m_start_button_lock  sync.Mutex
	m_start_button_state int

	// 子进程管理
	m_cmd_process *exec.Cmd
	m_cmd_mutex   sync.Mutex

	// 自动重启控制
	m_auto_restart_enabled bool
)

// StopCmdProcess 停止子进程（供外部调用，如窗口关闭时）
func StopCmdProcess() {
	m_cmd_mutex.Lock()
	defer m_cmd_mutex.Unlock()

	if m_cmd_process != nil && m_cmd_process.Process != nil {
		utils.KillProcess(m_cmd_process.Process.Pid)
		m_cmd_process = nil
	}

	// 清理所有遗留的cmd进程
	utils.CleanupOrphanedCmdProcesses()
}

// parseStatusMessage 解析子进程状态行，返回整数状态。
// 支持带时间戳前缀，如 "2024/01/01 12:00:00 [GOODLINK_STATUS]3"
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
		pro.StatusVersionMismatch, pro.StatusTransportMismatch, pro.StatusNeedAdmin:
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

func updateConnectionStatus(status int) {
	switch status {
	case pro.StatusConnecting:
		if GetWorkType() == workTypeLocal {
			setStatus(StatusConnecting)
		}
	case pro.StatusConnected:
		if GetWorkType() == workTypeLocal {
			setStatus(StatusConnected)
		}
	case pro.StatusConnectingNAT4:
		if GetWorkType() == workTypeLocal {
			setStatus(StatusConnectingNAT4)
		}
	case pro.StatusRunning:
		if GetWorkType() == workTypeRemote {
			setStatus(StatusRunning)
		}
	case pro.StatusVersionMismatch, pro.StatusTransportMismatch:
		m_start_button_lock.Lock()
		m_auto_restart_enabled = false
		m_start_button_lock.Unlock()
		go func() {
			time.Sleep(500 * time.Millisecond)
			StopCmdProcess()
		}()
	case pro.StatusNeedAdmin:
		m_start_button_lock.Lock()
		m_auto_restart_enabled = false
		m_start_button_lock.Unlock()
	}
}

// startCmdProcess 启动cmd进程（提取的公共逻辑，用于初始启动和自动重启）
func startCmdProcess() error {
	// 构建命令行参数
	cmdPath := utils.GetCmdExePath()

	// 检查 cmd 程序是否存在
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", filepath.Base(cmdPath))
	}

	// 构建命令行参数
	workType := GetWorkType()
	args := []string{"--fork", "--" + strings.ToLower(workType), "--key=" + getTunKey(), "--ui"}

	// 创建子进程
	m_cmd_mutex.Lock()
	m_cmd_process = exec.Command(cmdPath, args...)

	// 隐藏子进程窗口（仅 Windows）
	hideChildWindow(m_cmd_process)

	// 获取输出管道
	stdout, err := m_cmd_process.StdoutPipe()
	if err != nil {
		m_cmd_mutex.Unlock()
		UILogPrintF("获取stdout失败: %v", err)
		return err
	}
	stderr, err := m_cmd_process.StderrPipe()
	if err != nil {
		m_cmd_mutex.Unlock()
		UILogPrintF("获取stderr失败: %v", err)
		return err
	}

	// 启动子进程
	if err := m_cmd_process.Start(); err != nil {
		m_cmd_mutex.Unlock()
		UILogPrintF("启动失败: %v", err)
		return err
	}
	m_cmd_mutex.Unlock()

	// 处理进程输出的通用函数
	handleProcessOutput := func(scanner *bufio.Scanner) {
		for scanner.Scan() {
			line := scanner.Text()
			if addr, ok := parseProxyMessage(line); ok {
				setProxyListen(addr)
				continue
			}
			if status, ok := parseStatusMessage(line); ok {
				updateConnectionStatus(status)
			} else {
				UILogPrintF(line)
			}
		}
	}

	// 读取 stdout 和 stderr
	go handleProcessOutput(bufio.NewScanner(stdout))
	go handleProcessOutput(bufio.NewScanner(stderr))

	return nil
}

func saveConfig() error {
	cfg := config.UIConfig{
		WorkType:     GetWorkType(),
		TunKey:       getTunKey(),
		LocalMode:    getLocalMode(),
		Transport:    getTransport(),
		AutoStart:    getAutoStart(),
		ForwardRules: getForwardRules(),
	}
	log.Println(cfg)
	if err := config.SaveUIConfig(config.UIConfigPath(), cfg); err != nil {
		log.Println("保存 goodlink.json 失败:", err)
		return err
	}
	return nil
}

func HandleForwards(localMode string, rules []config.UIForwardRule) error {
	m_start_button_lock.Lock()
	running := m_start_button_state == 1
	m_start_button_lock.Unlock()
	if running {
		localMode = getLocalMode()
	}

	cfg := config.UIConfig{
		LocalMode:    localMode,
		ForwardRules: rules,
	}
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}

	var skip []config.UIForwardRule
	if running {
		skip = getForwardRules()
	}
	if conflicts := config.ListenConflicts(cfg.ForwardRules, skip); len(conflicts) > 0 {
		return fmt.Errorf("%s", config.FormatListenConflicts(conflicts))
	}

	if err := setLocalModeAndRules(cfg.LocalMode, cfg.ForwardRules); err != nil {
		return err
	}
	if err := saveConfig(); err != nil {
		return err
	}
	broadcastState()
	UILogPrintF("端口映射已保存")
	return nil
}

func autoConnectVerb() string {
	if GetWorkType() == workTypeRemote {
		return "启动"
	}
	return "连接"
}

func HandleSetAutoStart(autoStart bool, workType, tunKey, localMode, transport string, rules []config.UIForwardRule) error {
	if othersEnabled() {
		wt := strings.TrimSpace(workType)
		if wt == "" {
			wt = GetWorkType()
		}
		if wt != workTypeLocal && wt != workTypeRemote {
			return fmt.Errorf("无效的工作端侧: %s", wt)
		}
		cfg := config.UIConfig{
			LocalMode:    localMode,
			Transport:    transport,
			ForwardRules: rules,
		}
		if err := config.NormalizeAndValidate(&cfg); err != nil {
			return err
		}
		setWorkTypeAndKey(wt, tunKey)
		if err := setLocalModeAndRules(cfg.LocalMode, cfg.ForwardRules); err != nil {
			return err
		}
		setTransport(cfg.Transport)
	}
	setAutoStart(autoStart)
	if err := saveConfig(); err != nil {
		return err
	}
	broadcastState()

	if !autoStart {
		return nil
	}
	if getStatus() != StatusIdle {
		return nil
	}
	UILogPrintF("已开启自动%s, 正在启动%s端...", autoConnectVerb(), GetWorkType())
	if err := HandleStart(GetWorkType(), getTunKey(), getLocalMode(), getTransport(), getForwardRules()); err != nil {
		msg := err.Error()
		if msg == "already started" || msg == "busy" {
			return nil
		}
		return err
	}
	return nil
}

func tryAutoStart() {
	if !getAutoStart() {
		return
	}
	UILogPrintF("已配置自动%s, 正在启动%s端...", autoConnectVerb(), GetWorkType())
	if err := HandleStart(GetWorkType(), getTunKey(), getLocalMode(), getTransport(), getForwardRules()); err != nil {
		UILogPrintF("自动%s失败: %v", autoConnectVerb(), err)
	}
}

func HandleStart(workType, tunKey, localMode, transport string, rules []config.UIForwardRule) error {
	m_start_button_lock.Lock()
	defer m_start_button_lock.Unlock()

	if m_start_button_state != 0 {
		return fmt.Errorf("already started")
	}

	s := getStatus()
	if s == StatusDetecting || s == StatusStopping {
		return fmt.Errorf("busy")
	}

	cfg := config.UIConfig{
		LocalMode:    localMode,
		Transport:    transport,
		ForwardRules: rules,
	}
	if err := config.NormalizeAndValidate(&cfg); err != nil {
		return err
	}
	wt := workType
	if wt != workTypeLocal && wt != workTypeRemote {
		wt = GetWorkType()
	}
	if wt == workTypeLocal && cfg.LocalMode == config.LocalModeForward {
		if conflicts := config.ListenConflicts(cfg.ForwardRules, nil); len(conflicts) > 0 {
			return fmt.Errorf("%s", config.FormatListenConflicts(conflicts))
		}
	}

	setWorkTypeAndKey(workType, tunKey)
	if err := setLocalModeAndRules(cfg.LocalMode, cfg.ForwardRules); err != nil {
		return err
	}
	setTransport(cfg.Transport)
	if err := saveConfig(); err != nil {
		return err
	}
	if GetWorkType() == workTypeLocal && getLocalMode() == config.LocalModeTUN && !utils.IsAdmin() {
		return fmt.Errorf("%s", utils.NeedAdminRestartMsg)
	}

	resetProxyListen()
	if GetWorkType() == workTypeLocal {
		setStatus(StatusConnecting)
	} else {
		setStatus(StatusStarting)
	}

	m_start_button_state = 1

	// 设置自动重启标志
	m_auto_restart_enabled = true

	// 启动进程
	if err := startCmdProcess(); err != nil {
		UILogPrintF("启动失败: %v", err)
		m_start_button_state = 0
		m_auto_restart_enabled = false
		resetProxyListen()
		setStatus(StatusIdle)
		return err
	}

	// 更新按钮状态并等待进程结束
	go waitForProcessAndHandleExit()
	return nil
}

func HandleStop() {
	m_start_button_lock.Lock()
	defer m_start_button_lock.Unlock()

	if m_start_button_state != 1 {
		return
	}

	setStatus(StatusStopping)

	// 设置自动重启标志为false，防止误触发重启
	m_auto_restart_enabled = false

	// 停止子进程（在 goroutine 中执行，避免阻塞 UI）
	go func() {
		StopCmdProcess()
		m_start_button_state = 0
		resetProxyListen()
		setStatus(StatusIdle)
	}()
}

// waitForProcessAndHandleExit 等待进程结束并处理退出逻辑（在 goroutine 中运行，状态通过 SSE 推送到浏览器）
func waitForProcessAndHandleExit() {
	time.Sleep(time.Second * 1)
	if m_start_button_state != 1 {
		return
	}

	// 等待子进程结束
	m_cmd_mutex.Lock()
	proc := m_cmd_process
	m_cmd_mutex.Unlock()

	if proc != nil {
		proc.Wait()
	}

	// 检查是否为异常退出（需要自动重启）
	m_start_button_lock.Lock()
	isAbnormalExit := m_start_button_state == 1 && m_auto_restart_enabled
	m_start_button_lock.Unlock()

	if isAbnormalExit {
		// 异常退出，自动重启
		autoRestartProcess()
	} else {
		// 正常停止，恢复 UI
		m_start_button_state = 0
		resetProxyListen()
		setStatus(StatusIdle)
	}
}

// autoRestartProcess 自动重启进程（当进程异常退出时调用，在 goroutine 中运行）
func autoRestartProcess() {
	// 短暂延迟，避免频繁重启
	time.Sleep(500 * time.Millisecond)

	// 检查是否仍然需要重启（用户可能在此期间手动停止了）
	m_start_button_lock.Lock()
	if m_start_button_state != 1 || !m_auto_restart_enabled {
		m_start_button_lock.Unlock()
		return
	}
	m_start_button_lock.Unlock()

	UILogPrintF("检测到进程异常退出，正在自动重启...")

	resetProxyListen()
	// 重启进程
	if err := startCmdProcess(); err != nil {
		UILogPrintF("启动失败: %v", err)
		m_start_button_state = 0
		resetProxyListen()
		setStatus(StatusIdle)
		return
	}

	// 启动新的等待goroutine
	go waitForProcessAndHandleExit()
}
