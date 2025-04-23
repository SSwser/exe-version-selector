package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getlantern/systray"
	"github.com/sqweek/dialog"
	"gopkg.in/yaml.v3"
)

// 处理打开目录菜单项的点击
func openDirHandler(cfg *Config) {
	go func() {
		for range menuOpenDir.ClickedCh {
			// 每次点击都实时获取当前激活应用的路径
			app, ok := cfg.Apps[cfg.Activate]
			if !ok {
				showError("未找到当前激活应用")
				continue
			}
			dir := app.Path
			if idx := strings.LastIndexAny(dir, "\\/"); idx != -1 {
				dir = dir[:idx]
			}
			if dir == "" {
				showError("未能获取目录")
				continue
			}
			cmd := exec.Command("explorer", dir)
			err := cmd.Start()
			if err != nil {
				showError("打开文件夹失败: " + err.Error())
			}
		}
	}()
}

type App struct {
	Path string   `yaml:"path"`
	Args []string `yaml:"args"`
}

type Config struct {
	Activate string         `yaml:"activate"`
	Apps     map[string]App `yaml:"apps"`
	AppOrder []string       `yaml:"-"` // 不写入 yaml，仅用于顺序
}

const configFile = "config.yaml"

func loadConfig() (*Config, error) {
	paths := []string{
		configFile,
	}
	// 尝试上一层目录
	if wd, err := os.Getwd(); err == nil {
		parent := wd
		if idx := strings.LastIndexAny(wd, `/\\`); idx != -1 {
			parent = wd[:idx]
		}
		if parent != wd && parent != "" {
			paths = append(paths, parent+string(os.PathSeparator)+configFile)
		}
	}
	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, fmt.Errorf("配置文件格式错误（%v）: %v", paths, err)
	}
	if cfg.Apps == nil {
		cfg.Apps = make(map[string]App)
	}

	cfg.AppOrder = parseAppOrderNode(data)

	return &cfg, nil
}

// parseAppOrderNode 通过 yaml.v3 解析节点，严格保持 apps 字段顺序
func parseAppOrderNode(yamlData []byte) []string {
	var root yaml.Node
	yaml.Unmarshal(yamlData, &root)
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	// 找到 apps 字段
	m := root.Content[0]
	for i := 0; i < len(m.Content)-1; i += 2 {
		key := m.Content[i]
		if key.Value == "apps" {
			appsNode := m.Content[i+1]
			if appsNode.Kind == yaml.MappingNode {
				order := make([]string, 0, len(appsNode.Content)/2)
				for j := 0; j < len(appsNode.Content)-1; j += 2 {
					name := appsNode.Content[j].Value
					order = append(order, name)
				}
				return order
			}
		}
	}
	return nil
}

func saveConfig(cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
}

func listApps(cfg *Config) {
	fmt.Println("已配置应用：")
	for name, app := range cfg.Apps {
		current := ""
		if cfg.Activate == name {
			current = " (当前)"
		}
		fmt.Printf("- %s: %s%s\n", name, app.Path, current)
	}
}

func addApp(cfg *Config, args []string) {
	if len(args) < 2 {
		fmt.Println("用法: add <appname> <path> [args...]")
		return
	}
	name := args[0]
	path := args[1]
	appArgs := []string{}
	if len(args) > 2 {
		appArgs = args[2:]
	}
	cfg.Apps[name] = App{Path: path, Args: appArgs}
	fmt.Printf("添加应用 %s 成功\n", name)
}

func removeApp(cfg *Config, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: remove <appname>")
		return
	}
	name := args[0]
	if _, ok := cfg.Apps[name]; ok {
		delete(cfg.Apps, name)
		fmt.Printf("已删除应用 %s\n", name)
		if cfg.Activate == name {
			cfg.Activate = ""
		}
	} else {
		fmt.Println("应用不存在")
	}
}

func switchApp(cfg *Config, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: switch <appname>")
		return
	}
	name := args[0]
	if _, ok := cfg.Apps[name]; ok {
		cfg.Activate = name
		fmt.Printf("已切换当前应用为 %s\n", name)
	} else {
		fmt.Println("应用不存在")
	}
}

func parseArgs(args []string) map[string]string {
	result := make(map[string]string)
	var lastKey string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "--") {
			if eq := strings.Index(arg, "="); eq != -1 {
				// --key=value
				key := arg[:eq]
				val := arg[eq+1:]
				result[key] = val
				lastKey = ""
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "--") {
				// --key value
				result[arg] = args[i+1]
				lastKey = ""
				i++
			} else {
				// --flag（布尔）
				result[arg] = ""
				lastKey = arg
			}
		} else if lastKey != "" {
			result[lastKey] = arg
			lastKey = ""
		}
	}
	return result
}

func mergeArgs(defaults, overrides []string) []string {
	defMap := parseArgs(defaults)
	overMap := parseArgs(overrides)

	// 覆盖默认参数
	for k, v := range overMap {
		defMap[k] = v
	}

	// 保证顺序：先输出覆盖后的默认参数，再输出命令行中未出现在默认参数里的参数
	used := make(map[string]bool)
	result := []string{}
	for _, arg := range defaults {
		if strings.HasPrefix(arg, "--") {
			key := arg
			if eq := strings.Index(arg, "="); eq != -1 {
				key = arg[:eq]
			}
			if val, ok := defMap[key]; ok {
				if val == "" {
					result = append(result, key)
				} else {
					result = append(result, key+"="+val)
				}
				used[key] = true
			}
		} else {
			result = append(result, arg)
		}
	}

	// 添加命令行中新增的参数
	for _, arg := range overrides {
		if strings.HasPrefix(arg, "--") {
			key := arg
			if eq := strings.Index(arg, "="); eq != -1 {
				key = arg[:eq]
			}
			if !used[key] {
				if val, ok := overMap[key]; ok {
					if val == "" {
						result = append(result, key)
					} else {
						result = append(result, key+"="+val)
					}
					used[key] = true
				}
			}
		} else {
			result = append(result, arg)
		}
	}
	return result
}

// 终止进程树
func killProcessTree(pid int) error {
	// 使用 taskkill 命令终止进程树
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("终止进程树失败: %v, 输出: %s", err, output)
	}
	return nil
}

var trayQuitCh = make(chan struct{})
var switchAppCh = make(chan string)

// 全局：切换应用主菜单及子菜单项列表
var menuSwitch *systray.MenuItem
var menuSwitchSubs []*systray.MenuItem

var menuAppInfo *systray.MenuItem
var menuPath *systray.MenuItem
var menuArgs *systray.MenuItem
var menuOpenDir *systray.MenuItem

// 全局：启动时传入的参数（不含 os.Args[0]）
var initialArgs []string

func onReady(cfg *Config, app App, allArgs []string) {
	// 设置托盘图标和标题
	systray.SetIcon(getIcon())
	systray.SetTitle("EVS: " + cfg.Activate)
	systray.SetTooltip(fmt.Sprintf("当前应用: %s", cfg.Activate))

	// 添加切换应用父菜单
	menuSwitch = systray.AddMenuItem("切换应用", "切换到其他应用")
	menuSwitchSubs = []*systray.MenuItem{}
	for name := range cfg.Apps {
		sub := menuSwitch.AddSubMenuItem(name, fmt.Sprintf("切换到 %s", name))
		if name == cfg.Activate {
			sub.Check()
		} else {
			sub.Uncheck()
		}
		menuSwitchSubs = append(menuSwitchSubs, sub)
		go func(n string, m *systray.MenuItem) {
			for {
				<-m.ClickedCh
				if n != cfg.Activate {
					switchAppCh <- n
				}
			}
		}(name, sub)
	}

	// 添加“打开目录”菜单项（全局变量）
	menuOpenDir = systray.AddMenuItem("打开目录", "在文件资源管理器中打开当前应用所在文件夹")
	go openDirHandler(cfg)

	systray.AddSeparator()

	// 添加应用信息
	menuAppInfo = systray.AddMenuItem(fmt.Sprintf("应用: %s", cfg.Activate), "当前运行的应用")
	menuAppInfo.Disable()
	menuPath = systray.AddMenuItem(fmt.Sprintf("路径: %s", app.Path), "可执行文件路径")
	menuPath.Disable()
	menuArgs = systray.AddMenuItem(fmt.Sprintf("参数: %s", strings.Join(allArgs, " ")), "启动参数")
	menuArgs.Disable()
	systray.AddSeparator()

	// 添加退出菜单
	menuQuit := systray.AddMenuItem("退出", "退出应用")
	go func() {
		<-menuQuit.ClickedCh
		close(trayQuitCh)
	}()

	// 不再在 onReady 监听 switchAppCh，菜单内容更新全部在 runApp 中处理
}

func onExit() {
	// 清理托盘图标
	systray.Quit()
	// 保险起见也通知主goroutine
	select {
	case <-trayQuitCh:
		// 已经通知过了
	default:
		close(trayQuitCh)
	}
}

// 获取托盘图标数据
func getIcon() []byte {
	data, err := os.ReadFile("resources/icon.ico")
	if err != nil {
		fmt.Printf("读取托盘图标失败: %v\n", err)
		return []byte{}
	}
	return data
}

func showError(msg string) {
	dialog.Message("%s", msg).Title("错误").Error()
}

var childPid int

func runApp(cfg *Config, extraArgs []string) {
	if cfg.Activate == "" {
		showError("未设置当前应用，请先用 switch 命令切换")
		return
	}

	// 启动托盘（只调用一次）
	go systray.Run(func() {
		onReady(cfg, cfg.Apps[cfg.Activate], mergeArgs(cfg.Apps[cfg.Activate].Args, extraArgs))
	}, onExit)

	// 管理子进程及菜单切换
	for {
		app := cfg.Apps[cfg.Activate]
		allArgs := mergeArgs(app.Args, initialArgs)

		// 打印应用启动信息
		fmt.Printf("\n启动应用配置信息:\n")
		fmt.Printf("- 应用名称: %s\n", cfg.Activate)
		fmt.Printf("- 可执行文件: %s\n", app.Path)
		fmt.Printf("- 默认参数: %v\n", app.Args)
		fmt.Printf("- 附加参数: %v\n", initialArgs)
		fmt.Printf("- 启动参数: %s\n", strings.Join(allArgs, " "))

		cmdLine := fmt.Sprintf("\"%s\" %s", app.Path, strings.Join(allArgs, " "))
		fmt.Printf("- 最终命令行: %s\n", cmdLine)

		cmd := exec.Command(app.Path, allArgs...)
		childPid = 0
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
		if err := cmd.Start(); err != nil {
			errMsg := fmt.Sprintf("启动应用失败:\n%v", err)
			fmt.Println(errMsg)
			showError(errMsg)
			return
		}

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case newApp := <-switchAppCh:
			fmt.Printf("切换到应用: %s\n", newApp)

			// 先优雅退出当前进程
			cmd.Process.Signal(syscall.SIGTERM)
			time.Sleep(time.Second)
			killProcessTree(cmd.Process.Pid)
			killProcessTree(childPid)
			cfg.Activate = newApp
			saveConfig(cfg)
			systray.SetTitle("EVS: " + cfg.Activate)
			systray.SetTooltip(fmt.Sprintf("当前应用: %s", cfg.Activate))

			// 更新应用信息菜单项内容
			appNew := cfg.Apps[cfg.Activate]
			allArgsNew := mergeArgs(appNew.Args, initialArgs)
			if menuAppInfo != nil {
				menuAppInfo.SetTitle(fmt.Sprintf("应用: %s", cfg.Activate))
			}
			if menuPath != nil {
				menuPath.SetTitle(fmt.Sprintf("路径: %s", appNew.Path))
			}
			if menuArgs != nil {
				menuArgs.SetTitle(fmt.Sprintf("参数: %s", strings.Join(allArgsNew, " ")))
			}

			// 更新切换应用子菜单的 Checked 状态
			for i, name := range getAppNames(cfg) {
				if name == cfg.Activate {
					menuSwitchSubs[i].Check()
				} else {
					menuSwitchSubs[i].Uncheck()
				}
			}
			continue
		case <-trayQuitCh:
			// 处理托盘退出，终止进程树并输出日志
			fmt.Printf("\n收到托盘退出，正在关闭应用...\n")
			if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
				fmt.Printf("发送 SIGTERM 失败: %v，尝试终止进程树\n", err)
			}
			time.Sleep(time.Second)
			killProcessTree(cmd.Process.Pid)
			killProcessTree(childPid)

			// 等待子进程退出
			select {
			case <-done:
				fmt.Println("应用已成功终止")
			case <-time.After(5 * time.Second):
				fmt.Println("应用未在预期时间内终止，强制退出")
			}
			os.Exit(0)
		case err := <-done:
			if err != nil {
				fmt.Printf("应用退出，错误: %v\n", err)
			}
			killProcessTree(cmd.Process.Pid)
			killProcessTree(childPid)
			return
		}
	}
}

func printHelp() {
	fmt.Println("使用方法：")
	fmt.Println("  exe-version-selector <command> [args...]")
	fmt.Println("\n可用命令：")
	fmt.Printf("  %-26s %s\n", "list", "列出所有已配置的应用")
	fmt.Printf("  %-26s %s\n", "add <name> <path> [args]", "添加新应用，可选指定默认启动参数")
	fmt.Printf("  %-26s %s\n", "remove <name>", "删除指定应用")
	fmt.Printf("  %-26s %s\n", "switch <name>", "切换到指定应用")
	fmt.Printf("  %-26s %s\n", "help", "显示此帮助信息")
	fmt.Println("\n如果不指定命令，将直接运行当前选中的应用")
}

// 返回所有应用名，顺序与 config.yaml apps 字段顺序一致
func getAppNames(cfg *Config) []string {
	return cfg.AppOrder
}

func main() {
	// 捕获 Ctrl+C、关闭窗口等信号，确保退出时清理子进程
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		if childPid != 0 {
			killProcessTree(childPid)
		}
		os.Exit(0)
	}()

	// 初始化 initialArgs
	if len(os.Args) > 1 {
		initialArgs = os.Args[1:]
	} else {
		initialArgs = []string{}
	}

	if len(os.Args) >= 2 {
		cmd := os.Args[1]
		if cmd == "help" || cmd == "-h" || cmd == "--help" {
			printHelp()
			return
		}
		if cmd == "list" || cmd == "add" || cmd == "remove" || cmd == "switch" {
			cfg, err := loadConfig()
			if err != nil {
				fmt.Printf("读取配置文件失败: %v\n", err)
				return
			}
			switch cmd {
			case "list":
				listApps(cfg)
			case "add":
				addApp(cfg, os.Args[2:])
				saveConfig(cfg)
			case "remove":
				removeApp(cfg, os.Args[2:])
				saveConfig(cfg)
			case "switch":
				switchApp(cfg, os.Args[2:])
				saveConfig(cfg)
			}
			return
		}
	}

	// 默认行为：代理当前激活应用，并转发所有参数
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("读取配置文件失败: %v\n", err)
		return
	}
	runApp(cfg, initialArgs)
}
