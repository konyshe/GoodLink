package ui2

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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

// parseStatusMessage 解析状态消息，返回状态值（connecting/connected/waiting）和是否成功解析
// 支持带时间戳前缀的日志格式，如 "2024/01/01 12:00:00 [GOODLINK_STATUS]connected"
func parseStatusMessage(line string) (string, bool) {
	// 查找前缀在行中的位置（可能不在行首，因为有时间戳）
	idx := strings.Index(line, pro.TagStatusPrefix)
	if idx == -1 {
		return "", false
	}
	// 提取状态值（去除前缀后的内容，可能包含空格）
	status := strings.TrimSpace(line[idx+len(pro.TagStatusPrefix):])
	if status == pro.TagStatusConnecting || status == pro.TagStatusConnected || status == pro.TagStatusRunning || status == pro.TagStatusConnectingNAT4 || status == pro.TagStatusVersionMismatch || status == pro.TagStatusNeedAdmin {
		return status, true
	}
	return "", false
}

// updateConnectionStatus 根据连接状态更新按钮（Local端直接映射，Remote端在连接成功后才切换为运行状态）
// 由 handleProcessOutput goroutine 调用，状态通过 SSE 推送到浏览器
func updateConnectionStatus(status string) {
	switch GetWorkType() {
	case workTypeLocal:
		switch status {
		case pro.TagStatusConnecting:
			updateButtonState(buttonStateConnecting)
		case pro.TagStatusConnected:
			updateButtonState(buttonStateConnected)
		case pro.TagStatusConnectingNAT4:
			updateButtonState(buttonStateConnectingNat4ToNat4)
		case pro.TagStatusVersionMismatch:
			// 版本不一致，禁用自动重启并停止进程
			m_start_button_lock.Lock()
			m_auto_restart_enabled = false
			m_start_button_lock.Unlock()
			go func() {
				time.Sleep(500 * time.Millisecond) // 短暂延迟，确保日志已输出
				StopCmdProcess()
			}()
		case pro.TagStatusNeedAdmin:
			// 需要管理员权限，禁用自动重启（进程会自行退出）
			m_start_button_lock.Lock()
			m_auto_restart_enabled = false
			m_start_button_lock.Unlock()
		}
	case workTypeRemote:
		switch status {
		case pro.TagStatusRunning:
			updateButtonState(buttonStateRunning)
		}
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
	args := []string{"--fork", "--" + strings.ToLower(workType), "--key=" + getTunKey(), "--local_config", "--ui"}

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
			// 检查是否是状态消息
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
		ForwardRules: getForwardRules(),
	}
	log.Println(cfg)
	if err := config.SaveUIConfig(goodlinkFileName, cfg); err != nil {
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
	if err := setLocalModeAndRules(localMode, rules); err != nil {
		return err
	}
	if err := saveConfig(); err != nil {
		return err
	}
	broadcastState()
	UILogPrintF("端口映射已保存")
	return nil
}

func HandleStart(workType, tunKey, localMode string, rules []config.UIForwardRule) error {
	m_start_button_lock.Lock()
	defer m_start_button_lock.Unlock()

	if m_start_button_state != 0 {
		return fmt.Errorf("already started")
	}

	stateMu.RLock()
	busy := currentButton.kind == kindInitializing || currentButton.kind == kindStopping
	stateMu.RUnlock()
	if busy {
		return fmt.Errorf("busy")
	}

	setWorkTypeAndKey(workType, tunKey)
	if err := setLocalModeAndRules(localMode, rules); err != nil {
		return err
	}
	if err := saveConfig(); err != nil {
		return err
	}

	if GetWorkType() == workTypeLocal {
		updateButtonState(buttonStateConnecting)
	} else {
		updateButtonState(buttonStateStarting)
	}

	m_start_button_state = 1

	// 设置自动重启标志
	m_auto_restart_enabled = true

	// 启动进程
	if err := startCmdProcess(); err != nil {
		UILogPrintF("启动失败: %v", err)
		m_start_button_state = 0
		m_auto_restart_enabled = false
		updateButtonState(buttonStateIdle)
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

	updateButtonState(buttonStateStopping)

	// 设置自动重启标志为false，防止误触发重启
	m_auto_restart_enabled = false

	// 停止子进程（在 goroutine 中执行，避免阻塞 UI）
	go func() {
		StopCmdProcess()
		m_start_button_state = 0
		updateButtonState(buttonStateIdle)
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
		updateButtonState(buttonStateIdle)
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

	// 重启进程
	if err := startCmdProcess(); err != nil {
		UILogPrintF("启动失败: %v", err)
		m_start_button_state = 0
		updateButtonState(buttonStateIdle)
		return
	}

	// 启动新的等待goroutine
	go waitForProcessAndHandleExit()
}
