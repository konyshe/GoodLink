//go:build windows

package ui2

import (
	"golang.org/x/sys/windows"
)

func OpenBrowser(url string) {
	if url == "" {
		return
	}
	file, err := windows.UTF16PtrFromString(url)
	if err != nil {
		return
	}
	verb, err := windows.UTF16PtrFromString("open")
	if err != nil {
		return
	}
	_ = windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}
