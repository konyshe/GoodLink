package utils

import (
	"log"
	"os"
	"path/filepath"
	"time"
)

const (
	NOTIFY_FILE           = "goodlink_notify.tmp"
	NOTIFY_CHECK_INTERVAL = 500 * time.Millisecond
)

var showWindowChan chan struct{} // 用于通知主线程显示窗口的channel

// NotifyExistingInstance 通知已存在的实例显示窗口
func NotifyExistingInstance() error {
	notifyPath, err := notifyFilePath()
	if err != nil {
		return err
	}
	return os.WriteFile(notifyPath, []byte("SHOW_WINDOW"), 0644)
}

// StartInstanceListener 启动实例监听器，监听来自其他实例的显示窗口请求
func StartInstanceListener() {
	go func() {
		notifyPath, err := notifyFilePath()
		if err != nil {
			log.Printf("获取通知文件路径失败: %v", err)
			return
		}

		for {
			if _, err := os.Stat(notifyPath); err == nil {
				data, err := os.ReadFile(notifyPath)
				if err == nil && string(data) == "SHOW_WINDOW" {
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

func exeDir() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Dir(exePath), nil
}
