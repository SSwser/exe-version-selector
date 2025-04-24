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
var currentAppPid int
var extraArgs []string
var lastFoundArgs []string  // 仅记录 FindProcessByPath 找到的参数（不含exe路径）
var lastFoundAppName string // 记录参数对应的 app 名称

// 获取当前激活应用的路径和参数
func internalGetAppInfo() (string, string) {
	cfg, _ := internal.LoadConfig(configPath)
	app, ok := cfg.Apps[cfg.Activate]
	if !ok {
		return "", ""
	}
	return app.Path, strings.Join(app.Args, " ")
}

func internalGetActivate() string {
	cfg := internal.GetConfig()
	if cfg == nil {
		return ""
	}
	return cfg.Activate
}

func internalSwitchActivate(name string) bool {
	cfg := internal.GetConfig()
	if cfg == nil {
		return false
	}
	if _, ok := cfg.Apps[name]; ok {
		// 切换前自动 kill 旧进程
		if currentAppPid != 0 {
			fmt.Fprintf(os.Stderr, "[切换应用] 兜底强制终止进程树: PID=%d\n", currentAppPid)
			_ = internal.KillProcessTree(currentAppPid)
		}
		_ = killCurrentApp()
		cfg.Activate = name
		internal.SaveConfig(cfg, configPath)
		// 不再清空 lastFoundArgs，保证参数全程跟随
		return true
	}
	return false
}

func internalGetStatus() string {
	// 用 FindProcessByPath 判断真实运行状态
	// if pid, _, found := internal.FindProcessByPath(app.Path); found && pid != 0 {
	// 	return internal.AppRunning.String()
	// }
	return fmt.Sprintf("%s", appStatus)
}

// 下面这些函数需结合你的原有业务实现
func runAppProxy(args []string) {
	cfg := internal.GetConfig()
	if cfg == nil {
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
	fmt.Printf("[DEBUG] runAppProxy args: %v\n", args)
	fmt.Printf("[DEBUG] extraArgs: %v\n", extraArgs)

	// 参数合并规则：
	// 1. app.Args：应用配置文件中的默认参数
	// 2. lastFoundArgs：启动 evs 时检测到的已运行实例参数（不含 exe 路径，且始终合并，无论切换到哪个 app）
	// 3. args：本次 runAppProxy 传入的参数
	// 4. extraArgs：命令行参数（evs.exe 启动时的参数）
	finalArgs := app.Args
	if len(lastFoundArgs) > 0 {
		finalArgs = internal.MergeArgs(finalArgs, lastFoundArgs)
	}
	if args != nil && len(args) > 0 {
		finalArgs = internal.MergeArgs(finalArgs, args)
	}
	if extraArgs != nil && len(extraArgs) > 0 {
		finalArgs = internal.MergeArgs(finalArgs, extraArgs)
	}
	fmt.Printf("[DEBUG] finalArgs: %v\n", finalArgs)
	_, err := internal.StartAppProcess(app.Path, finalArgs, func(status string, pid int, exitErr error) {
		switch status {
		case "running":
			appStatus = fmt.Sprintf("运行中 (PID=%d)", pid)
			currentAppPid = pid
			fmt.Printf("已启动应用: %s (PID=%d)\n", app.Path, pid)
		case "exited":
			appStatus = fmt.Sprintf("已退出 (PID=%d)", pid)
			fmt.Println("应用已正常退出")
			currentAppPid = 0
		case "exit_failed":
			appStatus = fmt.Sprintf("异常退出 (PID=%d)", pid)
			fmt.Printf("应用异常退出，返回码非0: %v\n", exitErr)
			currentAppPid = 0
		case "killed":
			appStatus = fmt.Sprintf("被终止 (PID=%d)", pid)
			fmt.Printf("应用被信号终止: %v\n", exitErr)
			currentAppPid = 0
		case "crashed":
			appStatus = fmt.Sprintf("已崩溃 (PID=%d)", pid)
			fmt.Printf("应用崩溃: %v\n", exitErr)
			currentAppPid = 0
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

// 关闭当前启动的应用进程
func killCurrentApp() error {
	if currentAppPid == 0 {
		return nil
	}
	err := internal.KillProcessTreeAndWait(currentAppPid)
	if err == nil {
		appStatus = "已终止 (PID=" + fmt.Sprint(currentAppPid) + ")"
	} else {
		appStatus = "终止失败 (PID=" + fmt.Sprint(currentAppPid) + ")"
	}
	currentAppPid = 0
	return err
}

// 启动业务 socket 服务
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
	switch args[0] {
	case "exit":
		conn.Write([]byte("OK\n"))
		go func() {
			if currentAppPid != 0 {
				_ = internal.KillProcessTree(currentAppPid)
				currentAppPid = 0
			}
			time.Sleep(50 * time.Millisecond)
			os.Exit(0)
		}()
		return
	case "apporder":
		cfg, _ := internal.LoadConfig(configPath)
		fmt.Fprintf(os.Stderr, "[SOCKET] 收到 apporder, AppOrder=%v\n", cfg.AppOrder)
		for _, name := range cfg.AppOrder {
			conn.Write([]byte(name + "\n"))
		}
		return
	case "list":
		cfg, _ := internal.LoadConfig(configPath)
		for _, name := range cfg.AppOrder {
			app := cfg.Apps[name]
			line := fmt.Sprintf("%s %s %v", name, app.Path, app.Args)
			conn.Write([]byte(line + "\n"))
		}
	case "activate":
		conn.Write([]byte(internalGetActivate() + "\n"))
	case "switch":
		if len(args) < 2 {
			conn.Write([]byte("ERR need app name\n"))
			return
		}
		if internalSwitchActivate(args[1]) {
			go runAppProxy(nil)
			conn.Write([]byte("OK\n"))
		} else {
			conn.Write([]byte("ERR app not found\n"))
		}
	case "run":
		// 运行当前激活应用，参数透传
		go runAppProxy(args[1:])
		conn.Write([]byte("OK\n"))
	case "status":
		conn.Write([]byte(internalGetStatus() + "\n"))
	case "getappinfo":
		path, args := internalGetAppInfo()
		conn.Write([]byte(path + "|||" + args + "\n"))
	case "reload":
		// 重新加载配置（此处为占位，实际已每次操作自动加载）
		conn.Write([]byte("OK\n"))
	case "restart":
		oldPid := currentAppPid // 记录旧 PID
		err := killCurrentApp()
		if err == nil && oldPid != 0 {
			// 轮询确认进程树已完全退出，最多等 4 秒
			for i := 0; i < 40; i++ {
				if !internal.IsProcessTreeAlive(oldPid) {
					fmt.Printf("[restart] 旧进程及其子进程已完全退出\n", oldPid)
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
		fmt.Println("[restart] 启动新进程...")
		go runAppProxy(nil)
		conn.Write([]byte("OK\n"))
		return
	case "stop":
		err := killCurrentApp()
		if err == nil {
			conn.Write([]byte("OK\n"))
		} else {
			conn.Write([]byte("ERR kill failed\n"))
		}
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
	cfg := internal.GetConfig()
	if cfg != nil {
		app, ok := cfg.Apps[cfg.Activate]
		if ok {
			// 通过进程路径查找是否有已运行实例
			if pid, args, found := internal.FindProcessByPath(app.Path); found {
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
				currentAppPid = pid
			}
		}
	}
	if shouldStart {
		go runAppProxy(nil)
	}

	// 启动 socket 服务
	startConsoleServer(configPath)
}
