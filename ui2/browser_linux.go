//go:build linux

package ui2

import "os/exec"

func OpenBrowser(url string) {
	_ = exec.Command("xdg-open", url).Start()
}
