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

func disable_other(content string) {
	m_btn_local.Disable()
	m_btn_remote.Disable()
	m_validated_key.Disable()
	m_ui_local.Disable()
	m_ui_remote.Disable()
	m_button_key_create.Disable()
	m_button_key_paste.Disable()
	m_activity_start_button.Start()
	m_activity_start_button.Show()
	m_stats_start_button = 1
}

func enable_other() {
	m_btn_local.Enable()
	m_btn_remote.Enable()
	m_validated_key.Enable()
	m_ui_local.Enable()
	m_ui_remote.Enable()
	m_button_key_create.Enable()
	m_button_key_paste.Enable()
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

// parseStatusMessage 解析状态消息，返回状态值（connecting/connected）和是否成功解析
// 支持带时间戳前缀的日志格式，如 "2024/01/01 12:00:00 [GOODLINK_STATUS]connected"
func parseStatusMessage(line string) (string, bool) {
	const prefix = "[GOODLINK_STATUS]"
	// 查找前缀在行中的位置（可能不在行首，因为有时间戳）
	idx := -1
	for i := 0; i <= len(line)-len(prefix); i++ {
		if line[i:i+len(prefix)] == prefix {
			idx = i
			break
		}
	}
	if idx == -1 {
		return "", false
	}
	// 提取状态值（去除前缀后的内容，可能包含空格）
	status := line[idx+len(prefix):]
	// 去除前后空格
	status = strings.TrimSpace(status)
	if status == "connecting" || status == "connected" {
		return status, true
	}
	return "", false
}

// updateConnectionStatus 根据连接状态更新按钮（仅在Local模式下生效）
func updateConnectionStatus(status string) {
	if GetWorkType() != "Local" {
		return
	}
	if m_button_start == nil {
		return
	}

	switch status {
	case "connecting":
		m_button_start.Importance = widget.WarningImportance
		m_button_start.SetText("连接中")
		m_button_start.Refresh()
	case "connected":
		// Fyne没有SuccessImportance，使用HighImportance表示成功状态（主要颜色）
		m_button_start.Importance = widget.SuccessImportance
		m_button_start.SetText("连接成功")
		m_button_start.Refresh()
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

	var args []string
	switch GetWorkType() {
	case "Local":
		args = []string{"--fork", "--local", "--key=" + m_validated_key.Text}
	case "Remote":
		args = []string{"--fork", "--remote", "--key=" + m_validated_key.Text}
	}

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

	// 读取 stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			// 检查是否是状态消息
			if status, ok := parseStatusMessage(line); ok {
				updateConnectionStatus(status)
			} else {
				UILogPrintF(line)
			}
		}
	}()

	// 读取 stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// 检查是否是状态消息
			if status, ok := parseStatusMessage(line); ok {
				updateConnectionStatus(status)
			} else {
				UILogPrintF(line)
			}
		}
	}()

	return nil
}

func start_button_click() {
	m_lock_start.Lock()
	defer m_lock_start.Unlock()

	//先对需要填写的数据进行校验
	switch m_stats_start_button {
	case 0:
		if len(m_validated_key.Text) < 16 {
			SetLogLabel("请输入或点击生成连接密钥!")
			return
		}
		switch GetWorkType() {
		case "Local":
			if m_ui_local.GetLocalPort() == "" {
				//SetLogLabel("请填写访问端口号")
				//return
			}
		case "Remote":
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
		go func() {
			time.Sleep(time.Second * 1)
			if m_stats_start_button != 1 {
				m_activity_start_button.Stop()
				m_activity_start_button.Hide()
				return
			}
			m_button_start.Enable()
			// Local模式：初始状态为"连接中"（黄色）
			// Remote模式：显示"启动成功"（绿色）
			if GetWorkType() == "Local" {
				m_button_start.Importance = widget.WarningImportance
				m_button_start.SetText("连接中")
			} else {
				m_button_start.Importance = widget.SuccessImportance
				m_button_start.SetText("启动成功")
			}
			m_activity_start_button.Stop()
			m_activity_start_button.Hide()

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
				m_button_start.Importance = widget.HighImportance
				m_button_start.SetText("点击启动")
				m_button_start.Enable()
				enable_other()
			}
		}()

	case 1:
		m_button_start.Disable()

		// 停止子进程（在 goroutine 中执行，避免阻塞 UI）
		go func() {
			// 设置自动重启标志为false，防止误触发重启
			m_auto_restart_enabled = false
			StopCmdProcess()

			m_stats_start_button = 0
			enable_other()
			m_button_start.Importance = widget.HighImportance
			m_button_start.SetText("点击启动")
			m_button_start.Enable()
		}()
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
		m_button_start.Importance = widget.HighImportance
		m_button_start.SetText("点击启动")
		m_button_start.Enable()
		enable_other()
		return
	}

	// 根据模式设置按钮状态
	if GetWorkType() == "Local" {
		// Local模式：设置为"连接中"状态
		m_button_start.Importance = widget.WarningImportance
		m_button_start.SetText("连接中")
		m_button_start.Refresh()
	} else {
		// Remote模式：设置为"启动成功"状态
		m_button_start.Importance = widget.SuccessImportance
		m_button_start.SetText("启动成功")
		m_button_start.Refresh()
	}

	// 启动新的等待goroutine
	go func() {
		time.Sleep(time.Second * 1)
		if m_stats_start_button != 1 {
			return
		}
		m_button_start.Enable()

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
			// 异常退出，继续自动重启
			autoRestartProcess()
		} else {
			// 正常停止，恢复 UI
			m_stats_start_button = 0
			m_button_start.Importance = widget.HighImportance
			m_button_start.SetText("点击启动")
			m_button_start.Enable()
			enable_other()
		}
	}()
}
