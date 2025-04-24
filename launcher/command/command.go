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
	"time"

	"github.com/SSwser/exe-version-selector/internal"
)

// evsProcess 用于 RunApp 启动本地 evs.exe 进程
var evsProcess *os.Process

// SendCommand sends a line command to the socket server and returns trimmed response.
func SendCommand(cmd string) (string, error) {
	conn, err := net.Dial("tcp", "127.0.0.1:50505")
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

// GetApps returns the list of apps by "apporder" or fallback to "list".
func GetApps() []string {
	resp, err := SendCommand("apporder")
	if err != nil || strings.TrimSpace(resp) == "" {
		resp, err = SendCommand("list")
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

// GetActivate returns the current activated app name.
func GetActivate() string {
	resp, err := SendCommand("activate")
	if err != nil {
		return ""
	}
	return resp
}

// GetAppStatus returns AppStatus from "status".
func GetAppStatus() internal.AppStatus {
	resp, err := SendCommand("status")
	if err != nil {
		return internal.AppUnknown
	}
	return internal.ParseAppStatus(resp)
}

// GetCurrentAppPathArgs returns current app path and args from "getappinfo".
func GetCurrentAppPathArgs() (string, string) {
	resp, err := SendCommand("getappinfo")
	if err != nil {
		return "", ""
	}
	parts := strings.SplitN(resp, "|||", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return resp, ""
}

// RestartApp sends restart command.
func RestartApp() {
	SendCommand("restart")
}

// StopApp sends stop command.
func StopApp() {
	SendCommand("stop")
}

// RunApp sends run command.
func RunApp() {
	_, err := SendCommand("run")
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

// ReloadConfig sends reload command and waits briefly
func ReloadConfig() {
	SendCommand("reload")
	time.Sleep(100 * time.Millisecond)
}

// GetProcess returns the current local evs.exe process.
func GetProcess() *os.Process {
    return evsProcess
}
