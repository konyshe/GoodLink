//go:build unix

package utils

import (
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

var lockFile *os.File

func notifyFilePath() (string, error) {
	return filepath.Join(os.TempDir(), NOTIFY_FILE), nil
}

// CheckSingleInstance 检查是否为单实例，如果不是第一个实例则返回false
func CheckSingleInstance() bool {
	lockPath := filepath.Join(os.TempDir(), "goodlink.single.lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		log.Printf("创建单实例锁文件失败: %v", err)
		return true
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if notifyErr := NotifyExistingInstance(); notifyErr != nil {
			log.Printf("通知已存在实例失败: %v", notifyErr)
		}
		return false
	}

	lockFile = f
	showWindowChan = make(chan struct{}, 1)
	StartInstanceListener()
	return true
}
