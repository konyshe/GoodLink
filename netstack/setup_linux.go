//go:build linux

package netstack

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"unsafe"

	"golang.org/x/sys/unix"
)

func setUnixIP(name string, ip net.IP, mask net.IPMask) error {
	log.Printf("setUnixIP: %s %s %s", name, ip, mask)

	// 新增接口存在性检查
	if err := checkInterfaceExists(name); err != nil {
		return err
	}

	// 修改标志位设置流程（先关闭再开启）
	ifreq, _ := unix.NewIfreq(name)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(unix.AF_INET), unix.SIOCGIFFLAGS, uintptr(unsafe.Pointer(ifreq)))
	if errno != 0 {
		return fmt.Errorf("SIOCGIFFLAGS(pre-check) failed: %s", unix.ErrnoName(errno))
	}
	originalFlags := ifreq.Uint16()

	// 临时关闭接口（关键步骤）
	ifreq.SetUint16(originalFlags &^ unix.IFF_UP)
	_, _, errno = unix.Syscall(unix.SYS_IOCTL, uintptr(unix.AF_INET), unix.SIOCSIFFLAGS, uintptr(unsafe.Pointer(ifreq)))
	if errno != 0 {
		return fmt.Errorf("SIOCSIFFLAGS(down) failed: %s", unix.ErrnoName(errno))
	}

	// 设置IP地址结构体（修复掩码处理）
	addr, _ := unix.NewIfreq(name)
	addr.SetInet4Addr(ip.To4())
	_, _, errno = unix.Syscall(unix.SYS_IOCTL, uintptr(unix.AF_INET), unix.SIOCSIFADDR, uintptr(unsafe.Pointer(addr)))
	if errno != 0 {
		return fmt.Errorf("SIOCSIFADDR failed: %s", unix.ErrnoName(errno)) // 显示具体错误名称
	}

	// 设置子网掩码（修复变量名错误）
	maskAddr, _ := unix.NewIfreq(name)
	maskAddr.SetInet4Addr(net.IP(mask).To4()) // 显式转换掩码类型
	_, _, errno = unix.Syscall(unix.SYS_IOCTL, uintptr(unix.AF_INET), unix.SIOCSIFNETMASK, uintptr(unsafe.Pointer(maskAddr)))
	if errno != 0 {
		return fmt.Errorf("SIOCSIFNETMASK failed: %s", unix.ErrnoName(errno))
	}

	// 修改接口激活逻辑（关键修复）
	ifreq, _ = unix.NewIfreq(name)
	ifreq.SetUint16((originalFlags | unix.IFF_UP) &^ unix.IFF_NOARP) // 保留原始标志位并添加UP标志
	_, _, errno = unix.Syscall(unix.SYS_IOCTL, uintptr(unix.AF_INET), unix.SIOCSIFFLAGS, uintptr(unsafe.Pointer(ifreq)))
	if errno != 0 {
		return fmt.Errorf("SIOCSIFFLAGS failed: %s", unix.ErrnoName(errno))
	}

	log.Printf("Configuration applied successfully")
	return nil
}

// 新增辅助函数
func checkInterfaceExists(name string) error {
	ifreq, _ := unix.NewIfreq(name)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(unix.AF_INET), unix.SIOCGIFFLAGS, uintptr(unsafe.Pointer(ifreq)))
	if errno != 0 {
		return fmt.Errorf("interface %s not exists: %s", name, unix.ErrnoName(errno))
	}
	return nil
}

// 修改调用端错误处理
func SetTunIP(wintunEP *Device, ip string, mask int) error {
	// ip addr add 192.17.0.1/32 dev GoodLink
	// ip link set GoodLink up
	// ip route add 192.17.19.1 dev GoodLink

	// 设置网卡eth0的IP地址为192.168.1.10/24
	cmd := exec.Command("ip", "addr", "add", "192.17.0.1/32", "dev", GetName())
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("ip", "link", "set", GetName(), "up")
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = exec.Command("ip", "route", "add", GetRemoteIP(), "dev", GetName())
	if err := cmd.Run(); err != nil {
		return err
	}

	/*if err := setUnixIP(GetName(), net.ParseIP(ip), net.CIDRMask(mask, 32)); err != nil {
		return fmt.Errorf("setUnixIP failed: %w", err)
	}*/
	return nil
}
