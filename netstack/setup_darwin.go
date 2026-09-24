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

	// ifconfig utunN 192.17.N.1/32 up
	cmd := exec.Command("ifconfig", devName, fmt.Sprintf("%s/%d", ip, mask), "up")
	log.Printf("SetTunIP: %s", cmd.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ifconfig failed: %w, output: %s", err, out)
	}

	// ifconfig 可能把 CreateTUN 设的 MTU 改回默认，这里再钉成 TunMTU。
	cmd = exec.Command("ifconfig", devName, "mtu", fmt.Sprintf("%d", TunMTU))
	log.Printf("SetTunIP: %s", cmd.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ifconfig mtu failed: %w, output: %s", err, out)
	}

	// route add -host 192.17.N.1 -interface utunN
	cmd = exec.Command("route", "add", "-host", ip, "-interface", devName)
	log.Printf("SetTunIP: %s", cmd.String())
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("route add failed: %w, output: %s", err, out)
	}

	return nil
}
