// ========================
// 启动行为核心规则说明
//
// 1. .\evs.exe
//    启动激活应用（默认参数）+ socket 服务
//
// 2. .\evs.exe 6666
//    启动激活应用（参数为 6666）+ socket 服务
//
// 3. .\evs.exe list
//    只列出应用，不启动应用和 socket 服务
//
// 只有内置命令（list/add/remove/switch/help）会直接执行并退出，其他参数均作为启动参数传递给激活应用。
// ========================

package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/SSwser/exe-version-selector/internal"
)

var configPath = "config.yaml" // 全局可用
var appStatus = "未启动"
var extraArgs []string
var lastFoundArgs []string // 仅记录 FindProcessByPath 找到的参数（不含exe路径）
var currentAppPid int

var (
	idleTimer  *time.Timer
	idleTimerC <-chan time.Time
)

func setCurrentAppPid(pid int) {
	currentAppPid = pid
	if pid == 0 {
		if idleTimer == nil {
			idleTimer = time.NewTimer(2 * time.Minute)
			idleTimerC = idleTimer.C
			go func() {
				<-idleTimerC
				if currentAppPid == 0 {
					fmt.Println("[evs] 2分钟无应用运行，自动退出")
					os.Exit(0)
				}
			}()
		}
	} else {
		if idleTimer != nil {
			if !idleTimer.Stop() {
				<-idleTimer.C // drain
			}
			idleTimer = nil
			idleTimerC = nil
		}
	}
}

func internalGetConfig() (*internal.Config, error) {
	cfg := internal.GetConfig()
	if cfg == nil {
		return nil, fmt.Errorf("ERR config not loaded")
	}
	return cfg, nil
}

func internalGetAppStatus() string {
	// 用 FindProcessByPath 判断真实运行状态
	// if pid, _, found := internal.FindProcessByPath(app.Path); found && pid != 0 {
	// 	return internal.AppRunning.String()
	// }
	return fmt.Sprintf("%s", appStatus)
}

func internalGetActivate() string {
	cfg, err := internalGetConfig()
	if err != nil {
		return ""
	}
	return cfg.Activate
}

func internalSwitchActivate(name string) error {
	cfg, err := internalGetConfig()
	if err != nil {
		return err
	}

	if _, ok := cfg.Apps[name]; !ok {
		return fmt.Errorf("ERR app not found")
	}

	// 切换前自动 kill 旧进程
	// ---
	// 为什么这里要连续两次“杀进程”？
	// 1. 先用 KillProcessTree（taskkill /F /T）暴力兜底，确保主进程和所有子进程都被 Windows 强杀，防止有顽固子进程残留。
	//    但 taskkill 只是发出 kill 命令，主进程可能未必马上消失，或有极端情况未被完全杀掉。
	// 2. 再用 internalKillCurrentApp（KillProcessTreeAndWait）轮询等待主进程彻底退出（最多5秒），保证主进程真的消失，防止竞态问题。
	//    但它只检测主进程存活，可能有子进程残留，所以两者结合最大程度保证彻底清理。
	// 这样做是实际工程中最稳妥的“兜底+确认”组合。
	if currentAppPid != 0 {
		fmt.Fprintf(os.Stderr, "[切换应用] 兜底强制终止进程树: PID=%d\n", currentAppPid)
		_ = internal.KillProcessTree(currentAppPid)
	}
	err = internalKillCurrentApp()
	if err != nil {
		return err
	}

	cfg.Activate = name
	internal.SaveConfig(cfg, configPath)

	// 不要清空 lastFoundArgs，保证参数全程跟随
	go runAppProxy(nil)
	return nil
}

// 参数 name 为空时返回当前激活应用，否则返回指定应用
// 返回: name, path, args, error
func internalGetAppInfo(name string) (string, string, string, error) {
	appName := name
	if name == "" {
		appName = internalGetActivate()
	}

	cfg, err := internalGetConfig()
	if err != nil {
		return "", "", "", err
	}
	app, ok := cfg.Apps[appName]
	if !ok {
		return "", "", "", fmt.Errorf("ERR app not found")
	}
	return appName, app.Path, strings.Join(app.Args, " "), nil
}

func internalKillCurrentApp() error {
	if currentAppPid == 0 {
		return nil
	}
	err := internal.KillProcessTreeAndWait(currentAppPid)
	if err == nil {
		appStatus = "已终止 (PID=" + fmt.Sprint(currentAppPid) + ")"
		setCurrentAppPid(0)
	} else {
		appStatus = "终止失败 (PID=" + fmt.Sprint(currentAppPid) + ")"
	}
	return err
}

func runAppProxy(args []string) {
	cfg, err := internalGetConfig()
	if err != nil {
		fmt.Println("配置未加载")
		return
	}
	app, ok := cfg.Apps[cfg.Activate]
	if !ok {
		fmt.Println("未找到激活应用")
		return
	}

	fmt.Printf("[DEBUG] app.Args: %v\n", app.Args)
	fmt.Printf("[DEBUG] lastFoundArgs: %v\n", lastFoundArgs)
	fmt.Printf("[DEBUG] extraArgs: %v\n", extraArgs)
	fmt.Printf("[DEBUG] runAppProxy args: %v\n", args)

	// 参数合并规则：
	// 1. app.Args：应用配置文件中的默认参数
	// 2. lastFoundArgs：启动 evs 时检测到的已运行实例参数（不含 exe 路径，且始终合并，无论切换到哪个 app）
	// 3. extraArgs：命令行参数（evs.exe 启动时的参数）
	// 4. args：本次 runAppProxy 传入的参数
	finalArgs := app.Args
	for _, group := range [][]string{lastFoundArgs, extraArgs, args} {
		if len(group) > 0 {
			finalArgs = internal.MergeArgs(finalArgs, group)
		}
	}
	fmt.Printf("[DEBUG] finalArgs: %v\n", finalArgs)

	_, err = internal.StartAppProcess(app.Path, finalArgs, func(status string, pid int, exitErr error) {
		switch status {
		case "running":
			appStatus = fmt.Sprintf("运行中 (PID=%d)", pid)
			setCurrentAppPid(pid)
			fmt.Printf("已启动应用: %s (PID=%d)\n", app.Path, pid)
		case "exited":
			appStatus = fmt.Sprintf("已退出 (PID=%d)", pid)
			fmt.Println("应用已正常退出")
			setCurrentAppPid(0)
		case "exit_failed":
			appStatus = fmt.Sprintf("异常退出 (PID=%d)", pid)
			fmt.Printf("应用异常退出，返回码非0: %v\n", exitErr)
			setCurrentAppPid(0)
		case "killed":
			appStatus = fmt.Sprintf("被终止 (PID=%d)", pid)
			fmt.Printf("应用被信号终止: %v\n", exitErr)
			setCurrentAppPid(0)
		case "crashed":
			appStatus = fmt.Sprintf("已崩溃 (PID=%d)", pid)
			fmt.Printf("应用崩溃: %v\n", exitErr)
			setCurrentAppPid(0)
		case "start_failed":
			appStatus = "启动失败"
			fmt.Printf("启动应用失败: %v\n", exitErr)
		}
	})
	if err != nil {
		appStatus = "启动失败"
		fmt.Printf("启动应用失败: %v\n", err)
		return
	}
}

func startConsoleServer(configPath string) {
	ln, err := net.Listen("tcp", "127.0.0.1:50505")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[console] 启动 socket 失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("[console] 业务服务已启动，监听 127.0.0.1:50505")
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConsoleConn(conn, configPath)
	}
}

func handleConsoleConn(conn net.Conn, configPath string) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	cmdLine, err := r.ReadString('\n')
	if err != nil {
		return
	}
	cmdLine = strings.TrimSpace(cmdLine)
	args := strings.Split(cmdLine, " ")
	if len(args) == 0 {
		conn.Write([]byte("ERR empty command\n"))
		return
	}
	fmt.Printf("[SOCKET] 收到命令: %v\n", args)

	// 支持 command:args 格式
	cmd := args[0]
	cmdArg := ""
	if idx := strings.Index(cmd, ":"); idx != -1 {
		cmdArg = cmd[idx+1:]
		cmd = cmd[:idx]
	}

	switch cmd {
	case "activate":
		conn.Write([]byte(internalGetActivate()))
	case "status":
		conn.Write([]byte(internalGetAppStatus()))
	case "list":
		cfg, err := internalGetConfig()
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}
		conn.Write([]byte(strings.Join(cfg.AppOrder, "\n")))
	case "info":
		appName, appPath, appArgs, err := internalGetAppInfo(cmdArg)
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}
		appInfoStr := fmt.Sprintf("%s|||%s|||%s\n", appName, appPath, appArgs)
		conn.Write([]byte(appInfoStr))
	case "reload":
		fmt.Println("[reload]")
		err := internal.ReloadConfig(configPath)
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}
		internal.RefreshAllMenuItems() // 刷新托盘菜单（递归 OnRefresh）
		conn.Write([]byte("OK\n"))
	case "run":
		var runArgs []string // 运行当前激活应用，参数透传
		if cmdArg != "" {
			runArgs = strings.Fields(cmdArg)
		}

		go runAppProxy(runArgs)
		conn.Write([]byte("OK\n"))
	case "switch":
		if cmdArg == "" {
			conn.Write([]byte("ERR need app name\n"))
			return
		}

		err := internalSwitchActivate(cmdArg)
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}
		conn.Write([]byte("OK\n"))
	case "restart":
		err := internalKillCurrentApp()
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}

		fmt.Println("[restart] 启动新进程...")
		go runAppProxy(nil)
		conn.Write([]byte("OK\n"))
	case "stop":
		err := internalKillCurrentApp()
		if err != nil {
			conn.Write([]byte(err.Error()))
			return
		}
		fmt.Println("[stop] 已终止")
		conn.Write([]byte("OK\n"))
	case "exit":
		if currentAppPid != 0 {
			_ = internalKillCurrentApp()
			setCurrentAppPid(0)
		}
		conn.Write([]byte("OK\n"))
		time.Sleep(50 * time.Millisecond)
		os.Exit(0)
	default:
		conn.Write([]byte("ERR unknown command\n"))
	}
}

func main() {
	if err := internal.ReloadConfig(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "加载配置失败: %v\n", err)
		os.Exit(1)
	}

	// 捕捉 SIGINT/SIGTERM，主进程退出时自动 kill 子进程
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		if currentAppPid != 0 {
			_ = internal.KillProcessTree(currentAppPid)
		}
		os.Exit(0)
	}()

	if len(os.Args) > 1 {
		if internal.HandleCliCommand(os.Args[1:], configPath) {
			return
		}

		extraArgs = os.Args[1:]
	} else {
		extraArgs = nil
	}

	// 启动应用前判断是否已启动
	shouldStart := true

	appName, appPath, _, err := internalGetAppInfo("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "获取应用信息失败: %v\n", err)
		os.Exit(1)
	}
	if appName != "" {
		// 通过进程路径查找是否有已运行实例
		if pid, args, found := internal.FindProcessByPath(appPath); found {
			shouldStart = false
			appStatus = fmt.Sprintf("运行中 (PID=%d)", pid)
			fmt.Printf("[DEBUG] FindProcessByPath 原始args: %v\n", args)
			if len(args) > 1 {
				fmt.Printf("[DEBUG] FindProcessByPath 参数部分: %v\n", args[1:])
				lastFoundArgs = args[1:] // 只记录参数部分，不含exe路径
			} else {
				fmt.Printf("[DEBUG] FindProcessByPath 参数部分: []\n")
				lastFoundArgs = nil
			}
			fmt.Printf("[DEBUG] lastFoundArgs 赋值后: %v\n", lastFoundArgs)
			setCurrentAppPid(pid)
		}

		if shouldStart {
			go runAppProxy(nil)
		}

		// 启动 soc
		startConsoleServer(configPath)
	}
}
