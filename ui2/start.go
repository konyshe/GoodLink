//go:build windows

package ui2

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go2"
	"image/color"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"goodlink/config"
	"goodlink/utils"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var (
	m_start_button_lock     sync.Mutex
	m_button_start          *widget.Button
	m_activity_start_button *widget.Activity
	m_start_button_state    int

	// 子进程管理
	m_cmd_process *exec.Cmd
	m_cmd_mutex   sync.Mutex

	// 自动重启控制
	m_auto_restart_enabled bool
)

const (
	TagStatusPrefix          = "[GOODLINK_STATUS]"
	TagStatusConnecting      = "connecting"
	TagStatusConnectingNAT4  = "connecting_nat4"
	TagStatusConnected       = "connected"
	TagStatusRunning         = "running"
	TagStatusVersionMismatch = "version_mismatch"
)

// LogStatus 输出带 TagStatusPrefix 的状态行，供 UI 等解析
func UpdateStartButtonStatue(status string) {
	log.Printf("%s%s", TagStatusPrefix, status)
}

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
	idx := strings.Index(line, TagStatusPrefix)
	if idx == -1 {
		return "", false
	}
	// 提取状态值（去除前缀后的内容，可能包含空格）
	status := strings.TrimSpace(line[idx+len(TagStatusPrefix):])
	if status == TagStatusConnecting || status == TagStatusConnected || status == TagStatusRunning || status == TagStatusConnectingNAT4 || status == TagStatusVersionMismatch {
		return status, true
	}
	return "", false
}

// 按钮状态类型
type buttonState struct {
	text          string
	importance    widget.Importance
	icon          fyne.Resource
	dotColor      color.NRGBA
	enabled       bool
	activity      bool
	other_enabled bool
}

// 预定义的按钮状态
var (
	buttonStateInitializing = buttonState{
		text:          "检测网络中...",
		importance:    widget.HighImportance,
		icon:          theme.MediaPlayIcon(),
		enabled:       false,
		activity:      false,
		other_enabled: true,
	}
	buttonStateIdle = buttonState{
		text:          "点击启动",
		importance:    widget.HighImportance,
		icon:          theme.MediaPlayIcon(),
		enabled:       true,
		activity:      false,
		other_enabled: true,
	}
	buttonStateStarting = buttonState{
		text:          "启动中...",
		importance:    widget.WarningImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnecting = buttonState{
		text:          "连接中...",
		importance:    widget.WarningImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnectingNat4 = buttonState{
		text:          "当前网络是NAT4, 连接中...",
		importance:    widget.WarningImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnectingNat4ToNat4 = buttonState{
		text:          "两端网络都是NAT4, 连接中...",
		importance:    widget.DangerImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       true,
		activity:      true,
		other_enabled: false,
	}
	buttonStateConnected = buttonState{
		text:          "连接成功, 点击停止",
		importance:    widget.SuccessImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       true,
		activity:      false,
		other_enabled: false,
	}
	buttonStateRunning = buttonState{
		text:          "启动成功, 点击停止",
		importance:    widget.SuccessImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       true,
		activity:      false,
		other_enabled: false,
	}
	buttonStateStopping = buttonState{
		text:          "停止中...",
		importance:    widget.WarningImportance,
		icon:          theme.MediaStopIcon(),
		enabled:       false,
		activity:      false,
		other_enabled: false,
	}
)

// updateButtonState 更新启动按钮的状态，同时同步托盘图标小圆点颜色
func updateButtonState(state buttonState) {
	if m_button_start == nil {
		return
	}

	if state.enabled {
		m_button_start.Enable()
	} else {
		m_button_start.Disable()
	}

	if state.activity {
		m_activity_start_button.Start()
		m_activity_start_button.Show()
	} else {
		m_activity_start_button.Stop()
		m_activity_start_button.Hide()
	}

	if state.other_enabled {
		for _, comp := range uiComponents {
			comp.Enable()
		}
	} else {
		for _, comp := range uiComponents {
			comp.Disable()
		}
	}

	m_button_start.Importance = state.importance
	m_button_start.SetText(state.text)
	m_button_start.SetIcon(state.icon)
	m_button_start.Refresh()

	UpdateTrayIcon(state)
}

// updateConnectionStatus 根据连接状态更新按钮（Local端直接映射，Remote端在连接成功后才切换为运行状态）
// 由 handleProcessOutput goroutine 调用，UI 更新通过 fyne.Do 调度到主线程
func updateConnectionStatus(status string) {
	switch GetWorkType() {
	case workTypeLocal:
		switch status {
		case TagStatusConnecting:
			fyne.Do(func() { updateButtonState(buttonStateConnecting) })
		case TagStatusConnected:
			fyne.Do(func() { updateButtonState(buttonStateConnected) })
		case TagStatusConnectingNAT4:
			fyne.Do(func() { updateButtonState(buttonStateConnectingNat4ToNat4) })
		case TagStatusVersionMismatch:
			// 版本不一致，禁用自动重启并停止进程
			m_start_button_lock.Lock()
			m_auto_restart_enabled = false
			m_start_button_lock.Unlock()
			go func() {
				time.Sleep(500 * time.Millisecond) // 短暂延迟，确保日志已输出
				StopCmdProcess()
			}()
		}
	case workTypeRemote:
		switch status {
		case TagStatusRunning:
			fyne.Do(func() { updateButtonState(buttonStateRunning) })
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
	args := []string{"--fork", "--" + strings.ToLower(workType), "--key=" + m_validated_key.Text, "--local_config"}

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

// 保存配置文件, 下次启动加载
func saveConfig() {
	//先对需要填写的数据进行校验
	configByte, _ := json.Marshal(&config.ConfigInfo{
		WorkType: GetWorkType(),
		TunKey:   m_validated_key.Text,
	})
	log.Println(string(configByte))
	os.Remove(goodlinkFileName)
	go2.FileAppend(goodlinkFileName, configByte)
}

func start_button_click() {
	m_start_button_lock.Lock()
	defer m_start_button_lock.Unlock()

	switch m_start_button_state {
	case 0:
		if GetWorkType() == workTypeLocal {
			updateButtonState(buttonStateConnecting)
		} else {
			updateButtonState(buttonStateStarting)
		}

		saveConfig()

		// 强制刷新工作端侧按钮高亮，确保选中项明显显示
		updateWorkTypeButtons(GetWorkType())

		m_start_button_state = 1

		// 设置自动重启标志
		m_auto_restart_enabled = true

		// 启动进程
		if err := startCmdProcess(); err != nil {
			UILogPrintF("启动失败: %v", err)
			m_start_button_state = 0
			m_auto_restart_enabled = false
			updateButtonState(buttonStateIdle)
			return
		}

		// 更新按钮状态并等待进程结束
		go waitForProcessAndHandleExit(false)

	case 1:
		updateButtonState(buttonStateStopping)

		// 停止子进程（在 goroutine 中执行，避免阻塞 UI）
		go func() {
			// 设置自动重启标志为false，防止误触发重启
			m_auto_restart_enabled = false
			StopCmdProcess()

			fyne.Do(func() {
				m_start_button_state = 0
				updateButtonState(buttonStateIdle)
			})
		}()
	}
}

// waitForProcessAndHandleExit 等待进程结束并处理退出逻辑（在 goroutine 中运行，UI 更新通过 fyne.Do 调度到主线程）
func waitForProcessAndHandleExit(isRestart bool) {
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
		fyne.Do(func() {
			m_start_button_state = 0
			updateButtonState(buttonStateIdle)
		})
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
		fyne.Do(func() {
			m_start_button_state = 0
			updateButtonState(buttonStateIdle)
		})
		return
	}

	// 启动新的等待goroutine
	go waitForProcessAndHandleExit(true)
}
