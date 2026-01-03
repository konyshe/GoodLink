//go:build windows

package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	MUTEX_NAME            = "Global\\GoodLinkSingleInstance"
	NOTIFY_FILE           = "goodlink_notify.tmp"
	NOTIFY_CHECK_INTERVAL = 500 * time.Millisecond
)

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")

	mutexHandle    uintptr
	showWindowChan chan struct{} // 用于通知主线程显示窗口的channel
)

// CreateMutex 创建互斥锁
func CreateMutex(name string) (uintptr, error) {
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return 0, err
	}

	ret, _, err := procCreateMutex.Call(
		0,
		0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	if ret == 0 {
		return 0, fmt.Errorf("创建互斥锁失败: %v", err)
	}

	// 检查是否已存在（ERROR_ALREADY_EXISTS = 183）
	if err.(syscall.Errno) == 183 {
		return ret, fmt.Errorf("实例已存在")
	}

	return ret, nil
}

// CloseHandle 关闭句柄
func CloseHandle(handle uintptr) error {
	ret, _, _ := procCloseHandle.Call(handle)
	if ret == 0 {
		return fmt.Errorf("关闭句柄失败")
	}
	return nil
}

// NotifyExistingInstance 通知已存在的实例显示窗口
func NotifyExistingInstance() error {
	// 创建通知文件
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}
	dir := filepath.Dir(exePath)
	notifyPath := filepath.Join(dir, NOTIFY_FILE)

	// 写入通知文件
	err = os.WriteFile(notifyPath, []byte("SHOW_WINDOW"), 0644)
	if err != nil {
		return fmt.Errorf("写入通知文件失败: %v", err)
	}

	return nil
}

// StartInstanceListener 启动实例监听器，监听来自其他实例的显示窗口请求
func StartInstanceListener() {
	go func() {
		exePath, err := os.Executable()
		if err != nil {
			log.Printf("获取可执行文件路径失败: %v", err)
			return
		}
		dir := filepath.Dir(exePath)
		notifyPath := filepath.Join(dir, NOTIFY_FILE)

		for {
			// 检查通知文件是否存在
			if _, err := os.Stat(notifyPath); err == nil {
				// 文件存在，读取内容
				data, err := os.ReadFile(notifyPath)
				if err == nil && string(data) == "SHOW_WINDOW" {
					// 删除通知文件
					os.Remove(notifyPath)

					// 通过channel通知主线程显示窗口（非阻塞）
					select {
					case showWindowChan <- struct{}{}:
					default:
						// channel已满，跳过本次通知
					}
				}
			}

			time.Sleep(NOTIFY_CHECK_INTERVAL)
		}
	}()
}

// GetShowWindowChan 获取显示窗口通知channel
func GetShowWindowChan() <-chan struct{} {
	return showWindowChan
}

// CheckSingleInstance 检查是否为单实例，如果不是第一个实例则返回false
func CheckSingleInstance() bool {
	handle, err := CreateMutex(MUTEX_NAME)
	if err != nil {
		// 实例已存在，通知第一个实例显示窗口
		if notifyErr := NotifyExistingInstance(); notifyErr != nil {
			log.Printf("通知已存在实例失败: %v", notifyErr)
		}
		return false
	}

	// 保存互斥锁句柄，程序退出时系统会自动释放
	mutexHandle = handle

	// 初始化channel（带缓冲，避免阻塞）
	showWindowChan = make(chan struct{}, 1)

	// 启动监听器，等待其他实例的请求
	StartInstanceListener()

	return true
}
