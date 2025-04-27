package internal

import (
	"fmt"
	"strings"
	"time"
)

// AppStatus 表示应用运行状态，用于托盘菜单、主程序等统一判断
type AppMainStatus int

const (
	AppNotStarted AppMainStatus = iota // 未启动
	AppRunning                         // 运行中
	AppExited                          // 已退出
	AppCrashed                         // 已崩溃
	AppUnknown                         // 未知
)

func (s AppMainStatus) String() string {
	switch s {
	case AppNotStarted:
		return "未启动"
	case AppRunning:
		return "运行中"
	case AppExited:
		return "已退出"
	case AppCrashed:
		return "已崩溃"
	case AppUnknown:
		return "未知"
	default:
		return "未知"
	}
}

type AppStatus struct {
	Main      AppMainStatus // 主状态
	Pid       int           // 进程PID
	ExitCode  int           // 退出码
	Detail    string        // 详细描述
	Timestamp time.Time     // 状态变更时间
}

// NewAppStatus 构建带当前时间戳的 AppStatus
func NewAppStatus(main AppMainStatus, pid, exitCode int, detail string) AppStatus {
	return AppStatus{
		Main:      main,
		Pid:       pid,
		ExitCode:  exitCode,
		Detail:    detail,
		Timestamp: time.Now(),
	}
}

// ParseAppStatus 从描述字符串解析为 AppMainStatus 枚举
func ParseAppStatus(status string) AppMainStatus {
	switch {
	case strings.Contains(status, "未启动"):
		return AppNotStarted
	case strings.Contains(status, "已退出"):
		return AppExited
	case strings.Contains(status, "已崩溃"):
		return AppCrashed
	case strings.Contains(status, "运行中"):
		return AppRunning
	case strings.Contains(status, "未知"):
		return AppUnknown
	default:
		return AppUnknown
	}
}

// 示例公共业务逻辑（需根据 main.go 实际内容完善）
func ListApps(cfg *Config) {
	// 先计算所有 name 的最大宽度
	maxNameLen := 0
	for _, name := range cfg.AppOrder {
		if l := DisplayWidth(name); l > maxNameLen {
			maxNameLen = l
		}
	}
	for _, name := range cfg.AppOrder {
		app := cfg.Apps[name]
		marker := "   "
		if name == cfg.Activate {
			marker = "[*]"
		}
		pad := maxNameLen - DisplayWidth(name)
		fmt.Printf("%s %s%s  %s\n", marker, name, Spaces(pad), app.Path)
	}
}

func AddApp(cfg *Config, args []string) {
	if len(args) < 2 {
		fmt.Println("用法: add <name> <path> [args...]")
		return
	}
	name := args[0]
	path := args[1]
	appArgs := args[2:]
	if _, exists := cfg.Apps[name]; exists {
		fmt.Printf("应用名已存在: %s\n", name)
		return
	}
	cfg.Apps[name] = App{Path: path, Args: appArgs}
	cfg.AppOrder = append(cfg.AppOrder, name)
	fmt.Printf("已添加应用: %s\n", name)
}

func RemoveApp(cfg *Config, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: remove <name>")
		return
	}
	name := args[0]
	if _, ok := cfg.Apps[name]; !ok {
		fmt.Printf("未找到应用: %s\n", name)
		return
	}
	delete(cfg.Apps, name)
	// 移除顺序
	order := []string{}
	for _, n := range cfg.AppOrder {
		if n != name {
			order = append(order, n)
		}
	}
	cfg.AppOrder = order
	fmt.Printf("已删除应用: %s\n", name)
}

// ShowAppInfo 根据 app name 打印详细信息
func ShowAppInfo(cfg *Config, args []string) {
	name := ""
	if len(args) < 1 || args[0] == "" {
		name = cfg.Activate
		if name == "" {
			fmt.Println("无激活应用且未指定 name")
			return
		}
	} else {
		name = args[0]
	}
	app, ok := cfg.Apps[name]
	if !ok {
		fmt.Printf("未找到应用: %s\n", name)
		return
	}
	fmt.Printf("名称: %s\n", name)
	fmt.Printf("路径: %s\n", app.Path)
	fmt.Printf("参数: %s\n", joinArgs(app.Args))
}

func SwitchApp(cfg *Config, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: switch <name>")
		return
	}
	name := args[0]
	if _, ok := cfg.Apps[name]; !ok {
		fmt.Printf("未找到应用: %s\n", name)
		return
	}
	cfg.Activate = name
	fmt.Printf("已切换到应用: %s\n", name)
}
