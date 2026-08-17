//go:build darwin

package ui2

import "os/exec"

func OpenBrowser(url string) {
	_ = exec.Command("open", url).Start()
}
