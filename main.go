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
		svc, err := config.GetService(config.Arg_service_id)
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
		if err := pro.Init(svc); err != nil {
			log.Println(err)
			os.Exit(1)
		}

		switch strings.ToLower(svc.WorkType) {
		case "local":
			if err := pro.RunLocal(); err != nil {
				log.Println(err)
			}
			os.Exit(0)
		case "remote":
			pro.RunRemote()
			os.Exit(0)
		default:
			log.Println("无效的服务类型")
			os.Exit(1)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	pro.StopLocal()
	pro.StopRemote()
}

func runUI(openBrowser bool) {
	// 必须排在提权之前: 程序已在运行时只唤起已有窗口, 不再弹 UAC
	if utils.InstanceExists() {
		if err := utils.NotifyExistingInstance(); err != nil {
			log.Printf("通知已存在实例失败: %v", err)
		}
		return
	}

	// 启动时一次性申请管理员权限。用户取消授权则以普通权限继续,
	// 转发模式与 Remote 仍可用, TUN 模式在启动服务时直接报错。
	if !utils.IsAdmin() {
		if err := utils.RelaunchAsAdmin("--no-browser"); err != nil {
			log.Println(err)
		} else {
			return
		}
	}

	// 检查单实例，如果不是第一个实例则退出
	// 必须在创建任何UI资源之前检查，避免影响已运行的实例
	if !utils.CheckSingleInstance() {
		// 已有实例运行，直接退出
		return
	}

	// 启动前清理遗留的cmd进程
	utils.CleanupOrphanedCmdProcesses()

	goodlink_config.DeleteLocalConfig()
	if err := goodlink_config.Init(); err != nil {
		log.Println("拉取运行配置失败:", err)
	} else {
		info := goodlink_config.GetConfig()
		ui2.SetRuntimeConf(config.RuntimeConf{
			Redis: config.RuntimeRedis{
				Addr:    info.Redis.Addr,
				TlsAddr: info.Redis.TlsAddr,
				Passwd:  info.Redis.Passwd,
				ID:      info.Redis.Id,
			},
			StunList: info.StunList,
		})
	}

	ui2.InitTrayIcons()
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
		runUI(true)
		return
	}

	if zipPath, waitPid, target, ok := config.ParseSelfUpdateArgs(os.Args[1:]); ok {
		ui2.RunSelfUpdate(zipPath, waitPid, target)
		return
	}

	if openBrowser, ok := config.ParseUILaunchArgs(os.Args[1:]); ok {
		runUI(openBrowser)
		return
	}

	config.Help()

	if config.Arg_stun_svr_ip != "" && config.Arg_stun_svr_port > 0 {
		stun2.StartSvr(config.Arg_stun_svr_ip, config.Arg_stun_svr_port)
		return
	}

	if _, ok := config.LoadRuntimeConfFromEnv(); !ok {
		goodlink_config.Init()
	}

	if !config.FromUI() || config.Arg_service_id == "" {
		flag.Usage()
		os.Exit(0)
	}

	main2()
}
