//go:build !windows

package ui2

import (
	"os"
	"os/signal"
	"syscall"
)

func applyTrayIcons(_, _, _, _ []byte) {}

func UpdateTrayIcon(_ int) {}

func refreshTray() {}

func RunLoop(_ string) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	StopCmdProcess()
}
