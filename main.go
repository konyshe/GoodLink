package main

import (
	"flag"
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
	"strings"
	"syscall"
	"time"
)

func main2() {
	log.Println("官方网址: https://gitee.com/konyshe/goodlink")

	go func() {
		if config.Arg_pprof_addr != "" { // 性能监测，开发使用选项
			log.Println(http.ListenAndServe(config.Arg_pprof_addr, nil))
		}
	}()

	// 新增系统级调优
	runtime.GOMAXPROCS(runtime.NumCPU())
	debug.SetGCPercent(10) // 降低GC频率
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered from panic:", r)
			log.Println(string(debug.Stack()))
		}
	}()

	go2pool.Init()

	// 第三方集成, 关注以下代码即可
	go func() {
		if err := pro.Init(config.GetTunKey()); err != nil {
			log.Println(err)
			return
		}

		switch strings.ToLower(config.GetWorkType()) {
		case "local":
			if err := pro.RunLocal(); err != nil {
				log.Println(err)
				os.Exit(0)
			}
		case "remote":
			pro.RunRemote()
		default:
			log.Println("无效的工作端侧")
			os.Exit(0)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	pro.StopLocal()
	pro.StopRemote()
}

func runUI(openBrowser, autoConnect bool, waitPid int) {
	// 提权重启时先等旧进程退出，避免抢不到单实例锁与UI端口
	ui2.WaitForPidExit(waitPid, 15*time.Second)

	// 检查单实例，如果不是第一个实例则退出
	// 必须在创建任何UI资源之前检查，避免影响已运行的实例
	if !utils.CheckSingleInstance() {
		// 已有实例运行，直接退出
		return
	}

	// 启动前清理遗留的cmd进程
	utils.CleanupOrphanedCmdProcesses()

	goodlink_config.DeleteLocalConfig()

	ui2.InitTrayIcons()

	ui2.SetForceAutoConnect(autoConnect)
	ui2.SetPortRetry(waitPid > 0)
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

	if openBrowser {
		ui2.OpenBrowser(uiURL)
	}

	// Windows：主线程运行托盘消息循环；Linux/macOS：等待信号后退出
	ui2.RunLoop(uiURL)
}

func main() {
	attachParentConsoleIfCLI()

	// 无任何参数时启动 UI（Windows/Linux/macOS）
	if len(os.Args) <= 1 {
		runUI(true, false, 0)
		return
	}

	if zipPath, waitPid, target, ok := config.ParseSelfUpdateArgs(os.Args[1:]); ok {
		ui2.RunSelfUpdate(zipPath, waitPid, target)
		return
	}

	if openBrowser, autoConnect, waitPid, ok := config.ParseUILaunchArgs(os.Args[1:]); ok {
		runUI(openBrowser, autoConnect, waitPid)
		return
	}

	config.Help()

	if config.Arg_stun_svr_ip != "" && config.Arg_stun_svr_port > 0 {
		stun2.StartSvr(config.Arg_stun_svr_ip, config.Arg_stun_svr_port)
		return
	}

	goodlink_config.Init()

	if !config.FromUI() || config.GetTunKey() == "" {
		flag.Usage()
		os.Exit(0)
	}

	main2()
}
