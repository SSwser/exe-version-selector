package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"

	"github.com/getlantern/systray"

	"github.com/SSwser/exe-version-selector/internal"
	"github.com/SSwser/exe-version-selector/launcher/command"
)

var menuSwitch *systray.MenuItem
var menuSwitchSubs []*systray.MenuItem
var switchNames []string

// 缓存上一次的 app 列表用于防抖和变更检测
var lastSwitchAppNames []string

func trayOnReady() {
	// 设置托盘图标
	icon, err := os.ReadFile("resources/icon.ico")
	if err == nil {
		systray.SetIcon(icon)
	}

	// 菜单分组配置（全部声明式）
	menuConfig := []internal.MenuItemData{
		{
			Title:     "[未连接]",
			Tooltip:   "与 EVS core 的 socket 连接状态",
			Separator: true,
			OnRefresh: func(item *systray.MenuItem) {
				ok, timeout := command.Ping()
				if ok {
					item.SetTitle("[已连接]")
				} else if timeout {
					item.SetTitle("[连接超时]")
				} else {
					item.SetTitle("[未连接]")
				}
			},
		},
		{
			Title:   "应用: ",
			Tooltip: "当前运行的应用",
			Disable: true,
			OnRefresh: func(item *systray.MenuItem) {
				item.SetTitle("应用: " + command.GetActivate())
			},
		},
		{
			Title:   "状态：",
			Tooltip: "应用运行状态",
			Disable: true,
			OnRefresh: func(item *systray.MenuItem) {
				item.SetTitle("状态: " + command.GetAppStatus().String())
			},
		},
		{
			Title:   "路径: ",
			Tooltip: "可执行文件路径",
			Disable: true,
			OnRefresh: func(item *systray.MenuItem) {
				_, p, _ := command.GetAppInfo("")
				item.SetTitle("路径: " + p)
			},
		},
		{
			Title:     "参数: ",
			Tooltip:   "启动参数",
			Disable:   true,
			Separator: true,
			OnRefresh: func(item *systray.MenuItem) {
				_, _, a := command.GetAppInfo("")
				item.SetTitle("参数: " + a)
			},
		},
		{
			Title:   "打开目录",
			Tooltip: "在文件资源管理器中打开当前应用所在文件夹",
			OnClick: func(item *systray.MenuItem) {
				_, appPath, _ := command.GetAppInfo("")
				if appPath != "" {
					dir := filepath.Dir(appPath)
					exec.Command("explorer.exe", dir).Start()
				}
			},
		},
		{
			Title:   "切换到",
			Tooltip: "切换到其他应用",
		},
		{
			Title:   "启动 / 重启",
			Tooltip: "运行或重启当前激活的应用",
			OnClick: func(item *systray.MenuItem) {
				status := command.GetAppStatus()
				if status == internal.AppRunning {
					command.RestartApp()
				} else {
					command.RunApp()
				}
			},
		},
		{
			Title:     "关闭",
			Tooltip:   "远程关闭当前激活应用",
			Separator: true,
			OnClick: func(item *systray.MenuItem) {
				command.StopApp()
			},
		},
		{
			Title:   "重载配置",
			Tooltip: "重新加载 config.yaml",
			OnClick: func(item *systray.MenuItem) {
				command.ReloadConfig()
				if menuSwitch != nil {
					buildSwitchSubMenus()
				}
			},
		},
		{
			Title:   "退出",
			Tooltip: "退出应用",
			OnClick: func(item *systray.MenuItem) {
				systray.Quit()
			},
		},
	}
	for _, cfg := range menuConfig {
		entry := internal.CreateMenuFromConfig(cfg)
		if cfg.Title == "切换到" {
			menuSwitch = entry.Item
		}
		// 收集根菜单项，便于刷新
		internal.RootMenuEntries = append(internal.RootMenuEntries, entry)
	}
	// 初始化后立即同步一次切换子菜单
	buildSwitchSubMenus()

	// 注册菜单刷新器
	internal.RegisterMenuRefresher(func() {
		internal.UpdateTrayTitle(command.GetActivate())
	})

	// 启动菜单刷新器
	internal.StartMenuRefresher()
}

// 动态生成“切换到”子菜单配置
func buildSwitchSubMenus() {
	if menuSwitch == nil {
		return
	}

	apps := command.GetApps()
	changed := !reflect.DeepEqual(apps, lastSwitchAppNames)
	if changed {
		lastSwitchAppNames = make([]string, len(apps))
		copy(lastSwitchAppNames, apps)
		// 彻底隐藏和释放所有旧子项，避免子项残留
		// systray.MenuItem 无法彻底销毁旧子项和 goroutine；不要再建议使用 Disable！
		for _, sub := range menuSwitchSubs {
			sub.Hide()
		}
		menuSwitchSubs = nil
		switchNames = nil
		for _, name := range apps {
			appName := name
			sub := menuSwitch.AddSubMenuItem(appName, "切换到 "+appName)
			menuSwitchSubs = append(menuSwitchSubs, sub)
			switchNames = append(switchNames, appName)
			go func(n string, m *systray.MenuItem) {
				for {
					<-m.ClickedCh
					if n != command.GetActivate() {
						command.SwitchApp(n)
					}
				}
			}(appName, sub)
		}
	}

	// 每次都刷新 checked 状态
	activate := command.GetActivate()
	for i, sub := range menuSwitchSubs {
		if switchNames[i] == activate {
			sub.Check()
		} else {
			sub.Uncheck()
		}
	}
}

func trayOnExit() {
	evsProc := command.GetEVSProcess()
	if evsProc == nil {
		return
	}

	// 1. 优雅通知 evs
	command.SendCommand("exit")
	// 2. 最多等待 3 秒
	done := make(chan error, 1)
	go func() { _, err := evsProc.Wait(); done <- err }()
	select {
	case <-done:
		fmt.Println("[trayOnExit] evs 正常退出")
	case <-time.After(3 * time.Second):
		if err := internal.KillProcessTree(evsProc.Pid); err != nil {
			fmt.Printf("[trayOnExit] kill evs 失败: %v\n", err)
		} else {
			fmt.Println("[trayOnExit] evs 已被强制终止")
		}
	}
}

func main() {
	systray.Run(trayOnReady, trayOnExit)
}

func init() {
	command.OnEVSRun = buildSwitchSubMenus
	command.OnAppSwitch = buildSwitchSubMenus
}
