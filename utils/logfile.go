//go:build cmd

package utils

import (
	"io"
	"log"
	"os"
	"sync"
)

const (
	// MaxLogSize 日志文件最大大小（1MB）
	MaxLogSize = 1 * 1024 * 1024
	// LogFileName 日志文件名
	LogFileName = "goodlink.log"
)

// RotatingFileWriter 支持大小限制的日志文件写入器
type RotatingFileWriter struct {
	file     *os.File
	filename string
	mutex    sync.Mutex
	maxSize  int64
}

// NewRotatingFileWriter 创建新的日志文件写入器
func NewRotatingFileWriter(filename string, maxSize int64) (*RotatingFileWriter, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	writer := &RotatingFileWriter{
		file:     file,
		filename: filename,
		maxSize:  maxSize,
	}

	return writer, nil
}

// Write 实现 io.Writer 接口
func (w *RotatingFileWriter) Write(p []byte) (n int, err error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	// 检查文件大小
	info, err := w.file.Stat()
	if err != nil {
		return 0, err
	}

	// 如果写入后超过限制，先截断文件
	if info.Size()+int64(len(p)) > w.maxSize {
		// 关闭当前文件
		w.file.Close()

		// 读取现有文件内容（保留最后一部分）
		keepSize := w.maxSize / 2 // 保留一半大小
		if keepSize > info.Size() {
			keepSize = info.Size()
		}

		var keepData []byte
		if keepSize > 0 {
			// 读取文件末尾的内容
			oldFile, err := os.Open(w.filename)
			if err == nil {
				// 定位到要保留的位置
				offset := info.Size() - keepSize
				if offset < 0 {
					offset = 0
				}
				oldFile.Seek(offset, 0)
				keepData = make([]byte, keepSize)
				if n, err := oldFile.Read(keepData); err == nil && n > 0 {
					keepData = keepData[:n]
				} else {
					keepData = nil
				}
				oldFile.Close()
			}
		}

		// 重新创建文件
		w.file, err = os.Create(w.filename)
		if err != nil {
			return 0, err
		}

		// 写入保留的内容
		if len(keepData) > 0 {
			if _, err := w.file.Write(keepData); err != nil {
				// 如果写入保留内容失败，继续写入新数据
			}
		}
	}

	// 写入新数据
	n, err = w.file.Write(p)
	// 实时刷新到磁盘，确保日志实时写入
	if err == nil {
		w.file.Sync()
	}
	return n, err
}

// Close 关闭文件
func (w *RotatingFileWriter) Close() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.file.Close()
}

// InitLogFile 初始化日志文件输出
func InitLogFile() error {
	writer, err := NewRotatingFileWriter(LogFileName, MaxLogSize)
	if err != nil {
		return err
	}

	// 设置日志同时输出到文件和控制台
	multiWriter := io.MultiWriter(os.Stdout, writer)
	log.SetOutput(multiWriter)
	// 添加时间前缀
	log.SetFlags(log.Ltime)

	return nil
}
