//go:build windows

package netstack

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func Sudo() {
	// 获取当前进程的令牌
	var token windows.Token
	err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token)
	if err != nil {
		fmt.Println("Failed to open process token:", err)
		return
	}

	// 定义LUID结构体
	var luid windows.LUID
	err = windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeDebugPrivilege"), &luid)
	if err != nil {
		fmt.Println("Failed to lookup privilege value:", err)
		return
	}

	// 创建TOKEN_PRIVILEGES结构体
	var tp windows.Tokenprivileges
	tp.PrivilegeCount = 1
	tp.Privileges[0].Luid = luid
	tp.Privileges[0].Attributes = windows.SE_PRIVILEGE_ENABLED

	// 提升权限
	err = windows.AdjustTokenPrivileges(token, false, &tp, 0, nil, nil)
	if err != nil {
		fmt.Println("Failed to adjust token privileges:", err)
		return
	}

	fmt.Println("Privilege escalation successful")
}
