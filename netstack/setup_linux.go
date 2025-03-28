//go:build linux

package netstack

import (
	"github.com/quic-go/quic-go"
)

// setupNetstack 初始化并配置网络栈
// 该函数负责创建协议栈、设置网络接口、配置IP地址和路由表
// 返回:
//   - *stack.Stack: 配置好的网络栈实例
//   - error: 初始化过程中的错误信息
func Start() error {
	return nil
}

func SetForWarder(stun_quic_conn quic.Connection) {
}

func GetRemoteIP() string {
	return "Linux暂不支持Local端"
}

func Stop() {
}
