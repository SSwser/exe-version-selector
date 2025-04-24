package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlantern/systray"

	"exe-version-selector/internal"
)

var menuSwitchSubs []*systray.MenuItem
var switchNames []string
var menuSwitch *systray.MenuItem

var evsProcess *os.Process // 仅 launcher 启动的 evs.exe 子进程

func trayOnReady() {
	// 设置托盘图标
	icon, err := os.ReadFile("resources/icon.ico")
	if err == nil {
		systray.SetIcon(icon)
	}

	// 顶部动态状态项
	menuStatus := systray.AddMenuItem("[状态] ...", "应用运行状态")
	menuStatus.Disable()

	systray.AddSeparator()

	// 应用信息只读菜单
	appPath, appArgs := trayGetCurrentAppPathArgs()
	info := systray.AddMenuItem(fmt.Sprintf("应用: %s", trayGetActivate()), "当前运行的应用")
	info.Disable()
	path := systray.AddMenuItem(fmt.Sprintf("路径: %s", appPath), "可执行文件路径")
	path.Disable()
	args := systray.AddMenuItem(fmt.Sprintf("参数: %s", appArgs), "启动参数")
	args.Disable()

	systray.AddSeparator()

	openDirItem := systray.AddMenuItem("打开目录", "在文件资源管理器中打开当前应用所在文件夹")

	// 动态“切换应用”菜单
	menuSwitch = systray.AddMenuItem("切换到", "切换到其他应用")
	buildSwitchSubMenus()

	internal.RegisterMenuRefresher(func() {
		cur := trayGetActivate()
		for i, name := range switchNames {
			if name == cur {
				menuSwitchSubs[i].Check()
			} else {
				menuSwitchSubs[i].Uncheck()
			}
		}
	})

	// 定时刷新状态项
	internal.RegisterMenuRefresher(func() {
		status := trayGetStatus()
		menuStatus.SetTitle(status.String())
	})

	// 菜单分组配置
	menuConfig := []internal.MenuConfig{
		{
			Title:   "启动 / 重启",
			Tooltip: "运行或重启当前激活的应用",
			OnClick: func(item *systray.MenuItem) {
				status := trayGetStatus()
				if status == internal.AppRunning {
					restartAppRemote()
				} else {
					trayRunApp()
				}
			},
		},
	}

	// 递归生成所有菜单
	internal.CreateMenuFromConfig(menuConfig[0])

	closeAppItem := systray.AddMenuItem("关闭", "远程关闭当前激活应用")

	systray.AddSeparator()

	// 系统操作相关菜单
	reloadConfigItem := systray.AddMenuItem("重载配置", "重新加载 config.yaml")
	exitItem := systray.AddMenuItem("退出", "退出应用")

	go func() {
		for {
			<-closeAppItem.ClickedCh
			traySendCommand("stop")
		}
	}()
	go func() {
		for {
			<-openDirItem.ClickedCh
			openCurrentAppDir()
		}
	}()
	go func() {
		for {
			<-reloadConfigItem.ClickedCh
			reloadConfigRemote()
			buildSwitchSubMenus() // 动态刷新切换子菜单
		}
	}()
	go func() {
		for {
			<-exitItem.ClickedCh
			systray.Quit()
		}
	}()

	// 刷新逻辑注册
	internal.RegisterMenuRefresher(func() {
		info.SetTitle(fmt.Sprintf("应用: %s", trayGetActivate()))
		p, a := trayGetCurrentAppPathArgs()
		path.SetTitle(fmt.Sprintf("路径: %s", p))
		args.SetTitle(fmt.Sprintf("参数: %s", a))

	})
	internal.RegisterMenuRefresher(func() {
		internal.UpdateTrayTitle(trayGetActivate())
	})
	internal.StartMenuRefresher()
}

// 获取当前激活应用的路径和参数
func traySendCommand(cmd string) (string, error) {
	conn, err := net.Dial("tcp", "127.0.0.1:50505")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.Write([]byte(cmd + "\n"))
	resp, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(resp)), nil
}

func trayGetCurrentAppPathArgs() (string, string) {
	resp, err := traySendCommand("getappinfo")
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(resp, "|||", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return resp, ""
}

// 打开当前应用目录
func openCurrentAppDir() {
	appPath, _ := trayGetCurrentAppPathArgs()
	if appPath == "(未实现)" || appPath == "" {
		return
	}
	dir := appPath
	if idx := strings.LastIndexAny(appPath, "\\/"); idx > 0 {
		dir = appPath[:idx]
	}
	exec.Command("explorer.exe", dir).Start()
}

// 远程重载配置
func reloadConfigRemote() {
	traySendCommand("reload")
}

// 远程重启应用
func restartAppRemote() {
	traySendCommand("restart")
}

func trayGetStatus() internal.AppStatus {
	resp, err := traySendCommand("status")
	if err != nil {
		return internal.AppUnknown
	}
	return internal.ParseAppStatus(resp)
}

func trayRunApp() {
	_, err := traySendCommand("run")
	if err != nil {
		absPath, errAbs := filepath.Abs("evs.exe")
		if errAbs != nil {
			fmt.Printf("[trayRunApp] 获取 evs.exe 路径失败: %v\n", errAbs)
			return
		}
		cmd := exec.Command(absPath)
		cmd.Dir = "."
		err2 := cmd.Start()
		if err2 == nil {
			evsProcess = cmd.Process
		} else {
			fmt.Printf("[trayRunApp] 启动 evs.exe 失败: %v\n", err2)
		}
	}
}

func traySwitchApp(name string) {
	traySendCommand("switch " + name)

	// 切换后也动态刷新
	internal.UpdateTrayTitle(trayGetActivate())
	buildSwitchSubMenus()
}

func buildSwitchSubMenus() {
	// 清除历史子菜单
	for _, sub := range menuSwitchSubs {
		sub.Hide()
	}
	menuSwitchSubs = nil
	switchNames = nil
	apps := trayGetApps()
	for _, name := range apps {
		appName := name
		sub := menuSwitch.AddSubMenuItem(appName, "切换到 "+appName)
		menuSwitchSubs = append(menuSwitchSubs, sub)
		switchNames = append(switchNames, appName)
		go func(n string, m *systray.MenuItem) {
			for {
				<-m.ClickedCh
				if n != trayGetActivate() {
					traySwitchApp(n)
				}
			}
		}(appName, sub)
	}
}

func trayGetApps() []string {
	resp, err := traySendCommand("apporder")
	if err != nil || strings.TrimSpace(resp) == "" {
		resp, err = traySendCommand("list")
		if err != nil {
			return nil
		}
	}
	lines := strings.Split(resp, "\n")
	var apps []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			apps = append(apps, line)
		}
	}
	return apps
}

func trayGetActivate() string {
	resp, err := traySendCommand("activate")
	if err != nil {
		return ""
	}
	return resp
}

func trayOnExit() {
	if evsProcess == nil {
		return
	}

	err := evsProcess.Signal(os.Interrupt)
	if err != nil {
		fmt.Printf("[trayOnExit] 发送 Interrupt 信号失败: %v\n", err)
	}
	done := make(chan error, 1)
	go func() {
		_, werr := evsProcess.Wait()
		done <- werr
	}()
	select {
	case <-done:
		// evs.exe 已退出
	case <-time.After(3 * time.Second):
		// 超时后强制 kill 进程树，保证代理 app 一起退出
		err := internal.KillProcessTree(evsProcess.Pid)
		if err != nil {
			fmt.Printf("[trayOnExit] 强制 kill evs.exe 及子进程失败: %v\n", err)
		} else {
			fmt.Printf("[trayOnExit] 已强制 kill evs.exe 及所有子进程\n")
		}
	}
}

func main() {
	systray.Run(trayOnReady, trayOnExit)
}
