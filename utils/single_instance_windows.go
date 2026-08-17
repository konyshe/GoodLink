//go:build windows

package utils

import (
	"fmt"
	"log"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const MUTEX_NAME = "Global\\GoodLinkSingleInstance"

var (
	kernel32        = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutex = kernel32.NewProc("CreateMutexW")
	procCloseHandle = kernel32.NewProc("CloseHandle")

	mutexHandle uintptr
)

func notifyFilePath() (string, error) {
	dir, err := exeDir()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败: %v", err)
	}
	return filepath.Join(dir, NOTIFY_FILE), nil
}

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

func CloseHandle(handle uintptr) error {
	ret, _, _ := procCloseHandle.Call(handle)
	if ret == 0 {
		return fmt.Errorf("关闭句柄失败")
	}
	return nil
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
