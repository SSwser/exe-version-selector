package internal

import (
	"fmt"
	"strings"
)

// AppStatus 表示应用运行状态，用于托盘菜单、主程序等统一判断
type AppStatus int

func (a AppStatus) String() string {
	// 与 ParseAppStatus 的判定顺序保持一致
	switch a {
	case AppRunning:
		return "运行中"
	case AppStopped:
		return "未启动"
	case AppExited:
		return "已退出"
	case AppCrashed:
		return "已崩溃"
	case AppUnknown:
		fallthrough
	default:
		return "未知"
	}
}

const (
	AppUnknown AppStatus = iota
	AppRunning
	AppStopped
	AppExited
	AppCrashed
)

// ParseAppStatus 从描述字符串解析为 AppStatus 枚举
func ParseAppStatus(status string) AppStatus {
	switch {
	case strings.Contains(status, "未启动"):
		return AppStopped
	case strings.Contains(status, "已退出"):
		return AppExited
	case strings.Contains(status, "已崩溃"):
		return AppCrashed
	case strings.Contains(status, "运行中"):
		return AppRunning
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
		fmt.Printf("%s %s%s  %s %v\n", marker, name, Spaces(pad), app.Path, app.Args)
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
