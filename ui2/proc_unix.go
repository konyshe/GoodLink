//go:build !windows

package ui2

import "os/exec"

func hideChildWindow(cmd *exec.Cmd) {}
