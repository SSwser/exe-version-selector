package command

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/SSwser/exe-version-selector/internal"
)

var localSocketAddr = "127.0.0.1:50505"

// Ping checks if the socket server is reachable.
// Returns (ok, timeout): ok=true 表示连接成功，timeout=true 表示超时未响应，二者都为 false 表示连接被拒绝或其它错误。
func Ping() (ok bool, timeout bool) {
	conn, err := net.DialTimeout("tcp", localSocketAddr, 300*time.Millisecond)
	if err == nil {
		conn.Close()
		return true, false
	}
	if nerr, ok := err.(net.Error); ok && nerr.Timeout() {
		return false, true
	}
	return false, false
}

// SendCommand sends a line command to the socket server and returns trimmed response.
func SendCommand(cmd string) (string, error) {
	conn, err := net.Dial("tcp", localSocketAddr)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	writer := bufio.NewWriter(conn)
	writer.WriteString(cmd + "\n")
	writer.Flush()
	resp, err := io.ReadAll(conn)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(resp)), nil
}

// GetApps returns the list of apps by "list" (每行一个 name)。
func GetApps() []string {
	resp, err := SendCommand("list")
	if err != nil {
		return nil
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

// GetActivate returns the current activated app name.
func GetActivate() string {
	name, _, _ := GetAppInfo("")
	return name
}

// GetAppStatus returns AppStatus from "status".
func GetAppStatus() internal.AppStatus {
	resp, err := SendCommand("status")
	if err != nil {
		return internal.AppUnknown
	}
	return internal.ParseAppStatus(resp)
}

// GetAppInfo 获取应用信息，name 为空则为当前激活 app，否则为指定 app。
func GetAppInfo(name string) (string, string, string) {
	cmd := "info"
	if name != "" {
		cmd = "info:" + name
	}
	resp, err := SendCommand(cmd)
	if err != nil {
		return "", "", ""
	}
	parts := strings.SplitN(resp, "|||", 3)
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

// ReloadConfig sends reload command and waits briefly
func ReloadConfig() {
	SendCommand("reload")
	time.Sleep(100 * time.Millisecond)
}

// RestartApp sends restart command.
func RestartApp() {
	SendCommand("restart")
}

// StopApp sends stop command.
func StopApp() {
	SendCommand("stop")
}

// SwitchApp sends switch command.
var OnAppSwitch func()

func SwitchApp(name string) {
	SendCommand("switch:" + name)

	// 切换后也动态刷新
	internal.UpdateTrayTitle(GetActivate())
	if OnAppSwitch != nil {
		OnAppSwitch()
	}
}

// RunApp sends run command.
// OnEVSRun is a callback for menu refresh after running evs.exe.
var OnEVSRun func()

func RunApp(args ...string) {
	cmd := "run"
	if len(args) > 0 {
		cmd = "run:" + strings.Join(args, " ")
	}
	_, err := SendCommand(cmd)
	if err != nil {
		absPath, errAbs := filepath.Abs("evs.exe")
		if errAbs != nil {
			fmt.Printf("[trayRunApp] 获取 evs.exe 路径失败: %v\n", errAbs)
			return
		}

		cmdObj := exec.Command(absPath)
		cmdObj.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // 隐藏控制台窗口
		cmdObj.Dir = "."
		err2 := cmdObj.Start()
		if err2 != nil {
			fmt.Printf("[trayRunApp] 启动 evs.exe 失败: %v\n", err2)
			return
		}

		evsProcess = cmdObj.Process
		if OnEVSRun != nil {
			go func() {
				const (
					maxWait  = 10 * time.Second
					interval = 300 * time.Millisecond
				)
				waited := time.Duration(0)
				for waited < maxWait {
					apps := GetApps()
					if len(apps) > 0 {
						break
					}
					time.Sleep(interval)
					waited += interval
				}
				OnEVSRun()
			}()
		}
	}
}

// GetEVSProcess returns the current local evs.exe process.
var evsProcess *os.Process

func GetEVSProcess() *os.Process {
	return evsProcess
}
