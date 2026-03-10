//go:build darwin

package netstack

import (
	"fmt"
	"log"
	"os/exec"
)

// CleanupOldAdapter macOS utun 设备在 fd 关闭时自动销毁，无需手动清理
func CleanupOldAdapter(name string) {}

func SetTunIP(wintunEP *Device, ip string, mask int) error {
	devName := (*wintunEP).Name()
	remoteIP := GetRemoteIP()

	// ifconfig utunN 192.17.0.1 192.17.19.1 up
	cmd := exec.Command("ifconfig", devName, fmt.Sprintf("%s/%d", ip, mask), remoteIP, "up")
	log.Printf("SetTunIP: %s", cmd.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ifconfig failed: %w, output: %s", err, out)
	}

	// route add -host 192.17.19.1 -interface utunN
	cmd = exec.Command("route", "add", "-host", remoteIP, "-interface", devName)
	log.Printf("SetTunIP: %s", cmd.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("route add failed: %w, output: %s", err, out)
	}

	return nil
}
