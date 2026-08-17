//go:build windows

package main

import "golang.org/x/sys/windows"

func hideUIConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	procFreeConsole := kernel32.NewProc("FreeConsole")
	_, _, _ = procFreeConsole.Call()
}
