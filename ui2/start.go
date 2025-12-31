//go:build windows

package ui2

import (
	"bufio"
	"context"
	"encoding/json"
	"go2"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"goodlink/config"

	_ "embed"
	_ "net/http/pprof"

	"fyne.io/fyne/v2/widget"
)

var (
	m_mg_start              sync.WaitGroup
	m_lock_start            sync.Mutex
	m_button_start          *widget.Button
	m_activity_start_button *widget.Activity
	m_stats_start_button    int

	// 子进程管理
	m_cmd_process *exec.Cmd
	m_cmd_cancel  context.CancelFunc
)

func disable_other(content string) {
	m_radio_work_type.Disable()
	m_validated_key.Disable()
	m_ui_local.Disable()
	m_ui_remote.Disable()
	m_button_key_create.Disable()
	m_button_key_paste.Disable()
	m_activity_start_button.Start()
	m_activity_start_button.Show()
	UILogPrintF(content)
	m_stats_start_button = 1
}

func enable_other() {
	m_radio_work_type.Enable()
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
		switch m_radio_work_type.Selected {
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
			WorkType:   m_radio_work_type.Selected,
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

		// 构建命令行参数
		cmdPath := getCmdExePath()

		// 检查 cmd 程序是否存在
		if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
			UILogPrintF("未找到: %s", filepath.Base(cmdPath))
			enable_other()
			m_button_start.Enable()
			m_stats_start_button = 0
			return
		}

		var args []string
		switch m_radio_work_type.Selected {
		case "Local":
			args = []string{"-local", "-key=" + m_validated_key.Text}
		case "Remote":
			args = []string{"-remote", "-key=" + m_validated_key.Text}
		}

		// 创建带取消的 context
		ctx, cancel := context.WithCancel(context.Background())
		m_cmd_cancel = cancel
		m_cmd_process = exec.CommandContext(ctx, cmdPath, args...)

		// 隐藏子进程窗口
		m_cmd_process.SysProcAttr = &syscall.SysProcAttr{
			HideWindow:    true,
			CreationFlags: 0x08000000, // CREATE_NO_WINDOW
		}

		// 获取输出管道
		stdout, err := m_cmd_process.StdoutPipe()
		if err != nil {
			UILogPrintF("获取stdout失败: %v", err)
			enable_other()
			m_button_start.Enable()
			m_stats_start_button = 0
			return
		}
		stderr, err := m_cmd_process.StderrPipe()
		if err != nil {
			UILogPrintF("获取stderr失败: %v", err)
			enable_other()
			m_button_start.Enable()
			m_stats_start_button = 0
			return
		}

		// 启动子进程
		if err := m_cmd_process.Start(); err != nil {
			UILogPrintF("启动失败: %v", err)
			enable_other()
			m_button_start.Enable()
			m_stats_start_button = 0
			return
		}

		// 读取 stdout
		m_mg_start.Add(1)
		go func() {
			defer m_mg_start.Done()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				UILogPrintF(scanner.Text())
			}
		}()

		// 读取 stderr
		m_mg_start.Add(1)
		go func() {
			defer m_mg_start.Done()
			scanner := bufio.NewScanner(stderr)
			for scanner.Scan() {
				UILogPrintF(scanner.Text())
			}
		}()

		// 更新按钮状态
		m_mg_start.Add(1)
		go func() {
			defer func() {
				m_activity_start_button.Stop()
				m_activity_start_button.Hide()
				m_mg_start.Done()
			}()

			time.Sleep(time.Second * 1)
			if m_stats_start_button != 1 {
				return
			}
			m_button_start.Enable()
			m_button_start.Importance = widget.WarningImportance
			m_button_start.SetText("关闭连接")

			// 等待子进程结束
			m_cmd_process.Wait()

			m_stats_start_button = 0
			m_button_start.Importance = widget.HighImportance
			m_button_start.SetText("点击启动")
			m_button_start.Enable()
			enable_other()
		}()

	case 1:
		m_button_start.Disable()
		UILogPrintF("正在停止...")
		m_stats_start_button = 0

		// 杀掉子进程（在 goroutine 中执行，避免阻塞 UI）
		go func() {
			if m_cmd_cancel != nil {
				m_cmd_cancel()
			}
			if m_cmd_process != nil && m_cmd_process.Process != nil {
				m_cmd_process.Process.Kill()
			}

			m_mg_start.Wait()
			enable_other()
			UILogPrintF("等待启动")
			m_button_start.Importance = widget.HighImportance
			m_button_start.SetText("点击启动")
			m_button_start.Enable()
		}()
	}
}
