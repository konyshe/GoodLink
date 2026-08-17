package main

import (
	"embed"
	"flag"
	go2log "go2/log"
	go2pool "go2/pool"
	"goodlink/config"
	"goodlink/pro"
	_ "goodlink/pro"
	"goodlink/stun2"
	"goodlink/ui2"
	"goodlink/utils"
	goodlink_config "goodlink3/config"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"
)

//go:embed assert/tray_idle.ico assert/tray_warning.ico assert/tray_danger.ico assert/tray_success.ico
var trayIcons embed.FS

func main2() {
	log.Println("官方网址: https://gitee.com/konyshe/goodlink")

	go func() {
		if config.Arg_pprof_addr != "" { // 性能监测，开发使用选项
			log.Println(http.ListenAndServe(config.Arg_pprof_addr, nil))
		}
	}()

	// 初始化日志文件输出
	if err := utils.InitLogFile(); err != nil {
		log.Printf("初始化日志文件失败: %v", err)
	}

	// 新增系统级调优
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetGCPercent(10) // 降低GC频率
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
			log.Println(string(debug.Stack()))
		}
	}()

	config.SetVersion(GetVersionFromAppConfig())

	go2pool.Init()

	// 第三方集成, 关注以下代码即可
	go func() {
		if err := pro.Init(config.Arg_tun_key); err != nil {
			log.Println(err)
			return
		}

		if *config.Arg_tun_local {
			if err := pro.RunLocal(); err != nil {
				log.Println(err)
				os.Exit(0)
			}
		} else if *config.Arg_tun_remote {
			pro.RunRemote()
		} else {
			log.Println("参数错误")
			os.Exit(0)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	pro.StopLocal()
	pro.StopRemote()
}

func runUI() {
	// 隐藏 UI 主进程控制台窗口（仅 Windows；从已有终端启动时只脱离本进程）
	hideUIConsole()

	// 检查单实例，如果不是第一个实例则退出
	// 必须在创建任何UI资源之前检查，避免影响已运行的实例
	if !utils.CheckSingleInstance() {
		// 已有实例运行，直接退出
		return
	}

	// 启动前清理遗留的cmd进程
	utils.CleanupOrphanedCmdProcesses()

	goodlink_config.DeleteLocalConfig()

	config.SetVersion(GetVersionFromAppConfig())

	idle, _ := trayIcons.ReadFile("assert/tray_idle.ico")
	warning, _ := trayIcons.ReadFile("assert/tray_warning.ico")
	danger, _ := trayIcons.ReadFile("assert/tray_danger.ico")
	success, _ := trayIcons.ReadFile("assert/tray_success.ico")
	ui2.InitTrayIcons(idle, warning, danger, success)

	ui2.Init()

	uiURL, err := ui2.StartServer()
	if err != nil {
		return
	}

	// 监听显示窗口请求（二次启动时重新打开浏览器）
	go func() {
		for range utils.GetShowWindowChan() {
			ui2.OpenBrowser(uiURL)
		}
	}()

	ui2.OpenBrowser(uiURL)

	// Windows：主线程运行托盘消息循环；Linux/macOS：等待信号后退出
	ui2.RunLoop(uiURL)
}

func main() {
	// 无任何参数时启动 UI（Windows/Linux/macOS）
	if len(os.Args) <= 1 {
		runUI()
		return
	}

	config.SetVersion(GetVersionFromAppConfig())
	config.Help()

	if config.Arg_stun_svr_ip != "" && config.Arg_stun_svr_port > 0 {
		stun2.StartSvr(config.Arg_stun_svr_ip, config.Arg_stun_svr_port)
		return
	}

	if !*config.Arg_local_config {
		goodlink_config.DeleteLocalConfig()
	}

	goodlink_config.Init()

	if *config.Arg_stun_test { // 测试stun节点，开发使用选项
		stun2.TestStun()
		os.Exit(0)
	}

	if config.Arg_tun_key == "" {
		flag.Usage()
		os.Exit(0)
	}

	utils.GuardStart(main2, 500*time.Millisecond, func(err error) {
		// if 0: err==nil; -1: err==255; -2: err==254; err==1: 1; err==2
		if err != nil {
			log.Printf("异常退出: %v", err)
			go2log.Dingf("error: %v", err)
		}
	})
}
