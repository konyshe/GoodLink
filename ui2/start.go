//go:build windows

package ui2

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go2"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"goodlink/config"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	m_lock_start            sync.Mutex
	m_button_start          *widget.Button
	m_activity_start_button *widget.Activity
	m_stats_start_button    int

	// 子进程管理
	m_cmd_process *exec.Cmd
	m_cmd_mutex   sync.Mutex

	// 自动重启控制
	m_auto_restart_enabled bool
)

// UI组件接口，用于统一管理启用/禁用
type uiComponent interface {
	Enable()
	Disable()
}

// 所有需要控制的UI组件列表
var uiComponents = []uiComponent{}

func init() {
	// 延迟初始化，在 GetMainUI 中设置
}

// setUIComponents 设置需要控制的UI组件列表
func setUIComponents(components []uiComponent) {
	uiComponents = components
}

func disable_other(content string) {
	for _, comp := range uiComponents {
		comp.Disable()
	}
	m_activity_start_button.Start()
	m_activity_start_button.Show()
	m_stats_start_button = 1
}

func enable_other() {
	for _, comp := range uiComponents {
		comp.Enable()
	}
	m_activity_start_button.Stop()
	m_activity_start_button.Hide()
}

// 获取 cmd 版本可执行文件路径
func getCmdExePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return "goodlink-windows-amd64-cmd.exe"
	}
	dir := filepath.Dir(exePath)
	return filepath.Join(dir, "goodlink-windows-amd64-cmd.exe")
}

// killProcess 强制终止进程（使用 taskkill 确保终止）
func killProcess(pid int) {
	// 先尝试用 taskkill /F /T 强制终止进程树
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run()
}

// StopCmdProcess 停止子进程（供外部调用，如窗口关闭时）
func StopCmdProcess() {
	m_cmd_mutex.Lock()
	defer m_cmd_mutex.Unlock()

	if m_cmd_process != nil && m_cmd_process.Process != nil {
		killProcess(m_cmd_process.Process.Pid)
		m_cmd_process = nil
	}
}

const (
	statusPrefix     = "[GOODLINK_STATUS]"
	statusConnecting = "connecting"
	statusConnected  = "connected"
	statusWaiting    = "waiting"
)

// parseStatusMessage 解析状态消息，返回状态值（connecting/connected/waiting）和是否成功解析
// 支持带时间戳前缀的日志格式，如 "2024/01/01 12:00:00 [GOODLINK_STATUS]connected"
func parseStatusMessage(line string) (string, bool) {
	// 查找前缀在行中的位置（可能不在行首，因为有时间戳）
	idx := strings.Index(line, statusPrefix)
	if idx == -1 {
		return "", false
	}
	// 提取状态值（去除前缀后的内容，可能包含空格）
	status := strings.TrimSpace(line[idx+len(statusPrefix):])
	if status == statusConnecting || status == statusConnected || status == statusWaiting {
		return status, true
	}
	return "", false
}

// 按钮状态类型
type buttonState struct {
	text       string
	importance widget.Importance
	icon       fyne.Resource
}

// 预定义的按钮状态
var (
	buttonStateIdle = buttonState{
		text:       "点击启动",
		importance: widget.HighImportance,
		icon:       theme.MediaPlayIcon(),
	}
	buttonStateConnecting = buttonState{
		text:       "连接中",
		importance: widget.WarningImportance,
		icon:       theme.MediaStopIcon(),
	}
	buttonStateConnected = buttonState{
		text:       "连接成功, 点击停止",
		importance: widget.SuccessImportance,
		icon:       theme.MediaStopIcon(),
	}
	buttonStateWaiting = buttonState{
		text:       "当前有其他local端请求连接, 等待中",
		importance: widget.MediumImportance,
		icon:       theme.MediaStopIcon(),
	}
	buttonStateRunning = buttonState{
		text:       "启动成功, 点击停止",
		importance: widget.SuccessImportance,
		icon:       theme.MediaStopIcon(),
	}
)

// updateButtonState 更新启动按钮的状态
func updateButtonState(state buttonState) {
	if m_button_start == nil {
		return
	}
	m_button_start.SetText(state.text)
	m_button_start.Importance = state.importance
	m_button_start.SetIcon(state.icon)
	m_button_start.Refresh()
}

// updateConnectionStatus 根据连接状态更新按钮（仅在Local模式下生效）
func updateConnectionStatus(status string) {
	if GetWorkType() != workTypeLocal {
		return
	}
	switch status {
	case statusConnecting:
		updateButtonState(buttonStateConnecting)
	case statusConnected:
		updateButtonState(buttonStateConnected)
	case statusWaiting:
		updateButtonState(buttonStateWaiting)
	}
}

// startCmdProcess 启动cmd进程（提取的公共逻辑，用于初始启动和自动重启）
func startCmdProcess() error {
	// 构建命令行参数
	cmdPath := getCmdExePath()

	// 检查 cmd 程序是否存在
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		UILogPrintF("未找到: %s", filepath.Base(cmdPath))
		return fmt.Errorf("cmd程序不存在: %s", cmdPath)
	}

	// 构建命令行参数
	workType := GetWorkType()
	args := []string{"--fork", "--" + strings.ToLower(workType), "--key=" + m_validated_key.Text}

	// 创建子进程
	m_cmd_mutex.Lock()
	m_cmd_process = exec.Command(cmdPath, args...)

	// 隐藏子进程窗口
	m_cmd_process.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

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

func start_button_click() {
	m_lock_start.Lock()
	defer m_lock_start.Unlock()

	//先对需要填写的数据进行校验
	switch m_stats_start_button {
	case 0:
		switch GetWorkType() {
		case workTypeLocal:
			if m_ui_local.GetLocalPort() == "" {
				//SetLogLabel("请填写访问端口号")
				//return
			}
		case workTypeRemote:
			switch m_ui_remote.GetRemoteType() {
			case "代理模式":
			case "转发模式":
			}
		}

		// 保存配置文件, 下次启动加载
		configByte, _ := json.Marshal(&config.ConfigInfo{
			WorkType:   GetWorkType(),
			TunKey:     m_validated_key.Text,
			ConnType:   m_ui_local.GetConnType2(),
			LocalIP:    m_ui_local.GetLocalIP(),
			LocalPort:  m_ui_local.GetLocalPort(),
			RemoteType: m_ui_remote.GetRemoteType(),
			RemoteIP:   m_ui_remote.GetRemoteIP(),
			RemotePort: m_ui_remote.GetRemotePort(),
		})
		log.Println(string(configByte))
		os.Remove("goodlink.json")
		go2.FileAppend("goodlink.json", configByte)
	}

	switch m_stats_start_button {
	case 0:
		m_button_start.Disable()
		// 强制刷新工作端侧按钮高亮，确保选中项明显显示
		updateWorkTypeButtons(GetWorkType())
		disable_other("正在启动...")

		// 设置自动重启标志
		m_auto_restart_enabled = true

		// 启动进程
		if err := startCmdProcess(); err != nil {
			enable_other()
			m_button_start.Enable()
			m_stats_start_button = 0
			m_auto_restart_enabled = false
			return
		}

		// 更新按钮状态并等待进程结束
		go waitForProcessAndHandleExit(false)

	case 1:
		m_button_start.Disable()

		// 停止子进程（在 goroutine 中执行，避免阻塞 UI）
		go func() {
			// 设置自动重启标志为false，防止误触发重启
			m_auto_restart_enabled = false
			StopCmdProcess()

			m_stats_start_button = 0
			enable_other()
			updateButtonState(buttonStateIdle)
			m_button_start.Enable()
		}()
	}
}

// waitForProcessAndHandleExit 等待进程结束并处理退出逻辑
func waitForProcessAndHandleExit(isRestart bool) {
	time.Sleep(time.Second * 1)
	if m_stats_start_button != 1 {
		if !isRestart {
			m_activity_start_button.Stop()
			m_activity_start_button.Hide()
		}
		return
	}
	m_button_start.Enable()

	// 根据模式设置初始按钮状态
	if GetWorkType() == workTypeLocal {
		updateButtonState(buttonStateConnecting)
	} else {
		updateButtonState(buttonStateRunning)
	}

	if !isRestart {
		m_activity_start_button.Stop()
		m_activity_start_button.Hide()
	}

	// 等待子进程结束
	m_cmd_mutex.Lock()
	proc := m_cmd_process
	m_cmd_mutex.Unlock()

	if proc != nil {
		proc.Wait()
	}

	// 检查是否为异常退出（需要自动重启）
	m_lock_start.Lock()
	isAbnormalExit := m_stats_start_button == 1 && m_auto_restart_enabled
	m_lock_start.Unlock()

	if isAbnormalExit {
		// 异常退出，自动重启
		autoRestartProcess()
	} else {
		// 正常停止，恢复 UI
		m_stats_start_button = 0
		updateButtonState(buttonStateIdle)
		m_button_start.Enable()
		enable_other()
	}
}

// autoRestartProcess 自动重启进程（当进程异常退出时调用）
func autoRestartProcess() {
	// 短暂延迟，避免频繁重启
	time.Sleep(500 * time.Millisecond)

	// 检查是否仍然需要重启（用户可能在此期间手动停止了）
	m_lock_start.Lock()
	if m_stats_start_button != 1 || !m_auto_restart_enabled {
		m_lock_start.Unlock()
		return
	}
	m_lock_start.Unlock()

	UILogPrintF("检测到进程异常退出，正在自动重启...")

	// 重启进程
	if err := startCmdProcess(); err != nil {
		UILogPrintF("自动重启失败: %v", err)
		// 重启失败，恢复UI
		m_stats_start_button = 0
		updateButtonState(buttonStateIdle)
		m_button_start.Enable()
		enable_other()
		return
	}

	// 根据模式设置按钮状态
	if GetWorkType() == workTypeLocal {
		updateButtonState(buttonStateConnecting)
	} else {
		updateButtonState(buttonStateRunning)
	}

	// 启动新的等待goroutine
	go waitForProcessAndHandleExit(true)
}
