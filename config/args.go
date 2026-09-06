package config

import (
	"flag"
	"fmt"
	"go2"
	"os"
	"strconv"
	"strings"
)

// TunKeyByteLen 连接密钥固定字节长度（Local/Remote 必须一致）
const (
	TunKeyByteLen = 48
)

var (
	Arg_stun_svr_ip   string
	Arg_stun_svr_port int

	Arg_pprof_addr     string
	Arg_redis_addr     string
	Arg_redis_tls_addr string
	Arg_redis_pass     string
	Arg_redis_id       int

	Arg_ui              *bool
	Arg_self_update     bool
	Arg_upgrade_zip     string
	Arg_upgrade_waitpid int
	Arg_upgrade_target  string
)

func Help() {
	v := flag.Bool("v", false, "查看版本信息")
	flag.StringVar(&Arg_pprof_addr, "pprof_addr", "", "如果CPU/内存/网络异常, 可监测运行, 例如: 0.0.0.0:6060")

	flag.StringVar(&Arg_stun_svr_ip, "stun_svr_ip", "", "启动STUN服务，监听IP地址")
	flag.IntVar(&Arg_stun_svr_port, "stun_svr_port", 3478, "启动STUN服务，监听端口")

	flag.StringVar(&Arg_redis_addr, "redis_addr", "", "Redis服务地址, 例如: 1.2.3.4:6379")
	flag.StringVar(&Arg_redis_tls_addr, "redis_tls_addr", "", "Redis服务TLS地址, 例如: 1.2.3.4:16378")
	flag.StringVar(&Arg_redis_pass, "redis_pass", "", "Redis服务密码, 例如: 123456")
	flag.IntVar(&Arg_redis_id, "redis_id", 15, "Redis服务可使用的表ID")

	Arg_ui = flag.Bool("ui", false, "由UI启动, 从 goodlink.json 读取并监听转发配置")
	flag.BoolVar(&Arg_self_update, "self-update", false, "内部使用: 更新进程, 覆盖程序后重启")
	flag.StringVar(&Arg_upgrade_zip, "zip", "", "内部使用: 待解压的升级压缩包路径")
	flag.IntVar(&Arg_upgrade_waitpid, "wait-pid", 0, "内部使用: 等待退出的主进程PID")
	flag.StringVar(&Arg_upgrade_target, "target", "", "内部使用: 要覆盖的主程序路径")
	flag.Bool("no-browser", false, "启动UI但不自动打开浏览器")
	flag.Bool("auto-connect", false, "内部使用: 提权重启后自动继续本次连接")

	flag.Parse()

	if *v {
		fmt.Printf("Version: %s\n", GetVersion())
		fmt.Print(go2.BuildVersion())
		os.Exit(0)
	}
}

// ParseSelfUpdateArgs 从命令行解析 --self-update 及相关参数, 不走全局 flag。
func ParseSelfUpdateArgs(args []string) (zipPath string, waitPid int, target string, ok bool) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--self-update" || a == "-self-update":
			ok = true
		case strings.HasPrefix(a, "--self-update=") || strings.HasPrefix(a, "-self-update="):
			ok = true
		case a == "--zip" || a == "-zip":
			if i+1 < len(args) {
				i++
				zipPath = args[i]
			}
		case strings.HasPrefix(a, "--zip="):
			zipPath = strings.TrimPrefix(a, "--zip=")
		case strings.HasPrefix(a, "-zip="):
			zipPath = strings.TrimPrefix(a, "-zip=")
		case a == "--wait-pid" || a == "-wait-pid":
			if i+1 < len(args) {
				i++
				waitPid, _ = strconv.Atoi(args[i])
			}
		case strings.HasPrefix(a, "--wait-pid="):
			waitPid, _ = strconv.Atoi(strings.TrimPrefix(a, "--wait-pid="))
		case strings.HasPrefix(a, "-wait-pid="):
			waitPid, _ = strconv.Atoi(strings.TrimPrefix(a, "-wait-pid="))
		case a == "--target" || a == "-target":
			if i+1 < len(args) {
				i++
				target = args[i]
			}
		case strings.HasPrefix(a, "--target="):
			target = strings.TrimPrefix(a, "--target=")
		case strings.HasPrefix(a, "-target="):
			target = strings.TrimPrefix(a, "-target=")
		}
	}
	return
}

// parseBoolFlag 匹配 -name / --name / -name=v / --name=v 形式的布尔参数。
func parseBoolFlag(a, name string) (val bool, matched bool) {
	for _, prefix := range []string{"--", "-"} {
		if a == prefix+name {
			return true, true
		}
		if v, ok := strings.CutPrefix(a, prefix+name+"="); ok {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return false, false
			}
			return b, true
		}
	}
	return false, false
}

// ParseUILaunchArgs 解析仅由 UI 使用的启动参数（--no-browser / --auto-connect / --wait-pid）。
// 出现其它参数时返回 ok=false, 交由常规 CLI 流程处理。
func ParseUILaunchArgs(args []string) (openBrowser, autoConnect bool, waitPid int, ok bool) {
	if len(args) == 0 {
		return true, false, 0, false
	}

	openBrowser = true
	for i := 0; i < len(args); i++ {
		a := args[i]
		if v, matched := parseBoolFlag(a, "no-browser"); matched {
			openBrowser = !v
			continue
		}
		if v, matched := parseBoolFlag(a, "auto-connect"); matched {
			autoConnect = v
			continue
		}
		if a == "--wait-pid" || a == "-wait-pid" {
			if i+1 >= len(args) {
				return true, false, 0, false
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return true, false, 0, false
			}
			waitPid = n
			continue
		}
		if v, cut := strings.CutPrefix(a, "--wait-pid="); cut {
			n, err := strconv.Atoi(v)
			if err != nil {
				return true, false, 0, false
			}
			waitPid = n
			continue
		}
		if v, cut := strings.CutPrefix(a, "-wait-pid="); cut {
			n, err := strconv.Atoi(v)
			if err != nil {
				return true, false, 0, false
			}
			waitPid = n
			continue
		}
		return true, false, 0, false
	}
	return openBrowser, autoConnect, waitPid, true
}
