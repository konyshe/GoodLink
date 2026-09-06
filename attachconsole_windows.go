//go:build windows

package main

import (
	"log"
	"os"
	"strings"

	"golang.org/x/sys/windows"
	"goodlink/config"
)

func attachParentConsoleIfCLI() {
	args := os.Args[1:]
	if len(args) == 0 {
		return
	}
	if _, _, _, ok := config.ParseSelfUpdateArgs(args); ok {
		return
	}
	if _, _, _, ok := config.ParseUILaunchArgs(args); ok {
		return
	}
	for _, a := range args {
		if a == "--ui" || a == "-ui" || strings.HasPrefix(a, "--ui=") || strings.HasPrefix(a, "-ui=") {
			return
		}
	}
	attachParentConsole()
}

func attachParentConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procAttachConsole := kernel32.NewProc("AttachConsole")
	r, _, _ := procAttachConsole.Call(uintptr(^uint32(0))) // ATTACH_PARENT_PROCESS
	if r == 0 {
		return
	}
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 {
		os.Stdout = os.NewFile(uintptr(h), "stdout")
	}
	if h, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE); err == nil && h != 0 {
		os.Stderr = os.NewFile(uintptr(h), "stderr")
	}
	if h, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE); err == nil && h != 0 {
		os.Stdin = os.NewFile(uintptr(h), "stdin")
	}
	log.SetOutput(os.Stderr)
}
