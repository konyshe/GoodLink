package utils

import "os"

// GetCmdExePath 返回当前可执行文件路径，UI 通过带参数 fork 自身走 CLI
func GetCmdExePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	return exePath
}
