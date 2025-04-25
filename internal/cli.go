package internal

import "fmt"

// 打印帮助信息
func PrintHelp() {
	fmt.Println("使用方法：")
	fmt.Println("  exe-version-selector <command> [args...]")
	fmt.Println("\n如果不指定命令，将直接运行当前激活的应用")
	fmt.Println("\n可用命令：")
	fmt.Printf("  %-26s %s\n", "list", "列出所有已配置的应用")
	fmt.Printf("  %-26s %s\n", "add <name> <path> [args]", "添加新应用，可选指定默认启动参数")
	fmt.Printf("  %-26s %s\n", "remove <name>", "删除指定应用")
	fmt.Printf("  %-26s %s\n", "switch <name>", "切换到指定应用")
	fmt.Printf("  %-26s %s\n", "info <name>", "显示指定应用的详细信息")
	fmt.Printf("  %-26s %s\n", "help", "显示此帮助信息")
}

// HandleCliCommand 处理主程序的命令行参数
// 返回 true 表示已处理并应直接退出，false 表示未处理（可继续作为参数传递给激活应用）
func HandleCliCommand(args []string, configPath string) bool {
	if len(args) < 1 {
		return false
	}
	switch args[0] {
	case "info":
		cfg, _ := LoadConfig(configPath)
		ShowAppInfo(cfg, args[1:])
		return true
	case "list":
		cfg, _ := LoadConfig(configPath)
		ListApps(cfg)
		return true
	case "add":
		cfg, _ := LoadConfig(configPath)
		AddApp(cfg, args[1:])
		SaveConfig(cfg, configPath)
		return true
	case "remove":
		cfg, _ := LoadConfig(configPath)
		RemoveApp(cfg, args[1:])
		SaveConfig(cfg, configPath)
		return true
	case "switch":
		cfg, _ := LoadConfig(configPath)
		SwitchApp(cfg, args[1:])
		SaveConfig(cfg, configPath)
		return true
	case "help":
		PrintHelp()
		return true
	default:
		return false
	}
}
