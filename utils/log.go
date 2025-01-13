package utils

import (
	"fmt"
	"log"
)

var (
	mp_log        *Log_Class
	m_debug_state int
)

func Log() *Log_Class {
	if mp_log == nil {
		mp_log = &Log_Class{}
	}
	return mp_log
}

type LogProcFunc func(content string)

type Log_Class struct {
	ErrorFunc   LogProcFunc
	DebugFunc   LogProcFunc
	LogInfoFunc LogProcFunc
}

func (c *Log_Class) RegistError(handle LogProcFunc) {
	c.ErrorFunc = handle
}

func (c *Log_Class) RegistDebug(handle LogProcFunc) {
	c.DebugFunc = handle
}

func (c *Log_Class) RegistInfo(handle LogProcFunc) {
	c.LogInfoFunc = handle
}

func (c *Log_Class) SetDebugSate(state int) {
	m_debug_state = state
}

// Debug 日志输出和官方fmt.Print、fmt.Printf使用一致
func (c *Log_Class) Debug(content string) {
	content2 := fmt.Sprintf("%d: %s", m_debug_state, content)
	if c.DebugFunc != nil {
		c.DebugFunc(content2)
		return
	}
	log.Println(content2)
}

// DebugF 日志输出和官方fmt.Print、fmt.Printf使用一致
func (c *Log_Class) DebugF(format string, a ...interface{}) {
	c.Debug(fmt.Sprintf(format, a...))
}

// LogInfo 日志输出和官方fmt.Print、fmt.Printf使用一致
func (c *Log_Class) Info(content string) {
	if c.DebugFunc != nil {
		c.DebugFunc(content)
		return
	}
	log.Println(content)
}

// LogInfoF 日志输出和官方fmt.Print、fmt.Printf使用一致
func (c *Log_Class) InfoF(format string, a ...interface{}) {
	c.Info(fmt.Sprintf(format, a...))
}

// Error 日志输出和官方fmt.Print、fmt.Printf使用一致
func (c *Log_Class) Error(content string) {
	if c.DebugFunc != nil {
		c.DebugFunc(content)
		return
	}
	log.Println(content)
}

// ErrorF 日志输出和官方fmt.Print、fmt.Printf使用一致
func (c *Log_Class) ErrorF(format string, a ...interface{}) {
	c.Error(fmt.Sprintf(format, a...))
}
