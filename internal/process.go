package internal

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/StackExchange/wmi"
)

// ExtractExitCode 提取 error 中的退出码（如有），否则返回 false
func ExtractExitCode(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		// 优先使用 Go1.12+ ExitCode()
		if code := exitErr.ExitCode(); code != -1 {
			return code, true
		}
		// 尝试 syscall.WaitStatus
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus(), true
		}
	}
	return 0, false
}

// 检查进程是否存活（windows 适用）
func IsProcessAlive(pid int) bool {
	if pid == 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

// 终止进程树
func KillProcessTree(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} // 隐藏控制台窗口
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("终止进程树失败: %v, 输出: %s", err, output)
	}
	return nil
}

// 终止进程树并等待主进程彻底退出（注意：只检测主进程存活，可能有子进程残留）
// 推荐结合 IsProcessTreeAlive 检查进程树是否完全退出
func KillProcessTreeAndWait(pid int) error {
	err := KillProcessTree(pid)
	if err != nil {
		return err
	}

	for i := 0; i < 50; i++ { // 最多等5秒
		if !IsProcessTreeAlive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if IsProcessTreeAlive(pid) {
		return fmt.Errorf("进程树未完全退出")
	}
	return nil
}

// 启动应用进程并异步监控退出，所有状态通过回调返回
func StartAppProcess(path string, args []string, onStatus func(status string, pid int, exitErr error)) (int, error) {
	cmd := exec.Command(path, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		onStatus("start_failed", 0, err)
		return 0, err
	}
	pid := cmd.Process.Pid
	onStatus("running", pid, nil)
	go func() {
		err := cmd.Wait()
		if err == nil {
			onStatus("exited", pid, nil)
			return
		}

		exitErr, ok := err.(*exec.ExitError)
		if ok {
			// Windows: exitErr.ExitCode()，Unix: exitErr.Sys().(syscall.WaitStatus)
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				if status.Signaled() {
					onStatus("killed", pid, err)
					return
				}
				if code := status.ExitStatus(); code != 0 {
					onStatus("exit_failed", pid, err)
					return
				}
			}
			// fallback: 非 0 退出码
			onStatus("exit_failed", pid, err)
			return
		}
		// 其它异常
		onStatus("crashed", pid, err)
	}()
	return pid, nil
}

type wmiProc struct {
	ProcessId      uint32  `wmi:"ProcessId"`
	ExecutablePath *string `wmi:"ExecutablePath"`
	CommandLine    *string `wmi:"CommandLine"`
}

// FindProcessByPath 在系统进程中查找与指定路径匹配的进程（仅查首个匹配，区分大小写）。
// 返回 pid、命令行参数、是否找到。
// Windows 需用 WMI 查询进程信息。
func FindProcessByPath(path string) (pid int, args []string, found bool) {
	var (
		wmiQuery = "SELECT ProcessId, ExecutablePath, CommandLine FROM Win32_Process"
	)

	var procs []wmiProc
	if err := wmiQueryAll(wmiQuery, &procs); err != nil {
		return 0, nil, false
	}
	for _, p := range procs {
		if p.ExecutablePath != nil {
			// fmt.Printf("[DEBUG] PID=%d, ExecutablePath=%q\n", p.ProcessId, *p.ExecutablePath)
			// 路径归一化并大小写不敏感比较
			normExe, err2 := filepath.Abs(filepath.Clean(*p.ExecutablePath))
			normTarget, err1 := filepath.Abs(filepath.Clean(path))
			if err1 == nil && err2 == nil && strings.EqualFold(normExe, normTarget) {
				fmt.Printf("[DEBUG] MATCH: PID=%d\n", p.ProcessId)
				pid := int(p.ProcessId)
				args := parseCmdlineWin32(p.CommandLine)
				fmt.Printf("[DEBUG] MATCHED CMDLINE: %v\n", args)
				return pid, args, true
			}
		}
	}
	fmt.Println("[DEBUG] No matching process found.")
	return 0, nil, false
}

// wmiQueryAll 封装 WMI 查询
func wmiQueryAll(query string, dst interface{}) error {
	return wmi.Query(query, dst)
}

// parseCmdlineWin32 将 Win32_Process.CommandLine 拆分为参数
func parseCmdlineWin32(cmd *string) []string {
	if cmd == nil {
		return nil
	}
	argv, err := CommandLineToArgv(*cmd)
	if err == nil {
		return argv
	}
	return strings.Fields(*cmd)
}

// CommandLineToArgv 封装 Windows API
func CommandLineToArgv(cmd string) ([]string, error) {
	shell32 := syscall.NewLazyDLL("shell32.dll")
	proc := shell32.NewProc("CommandLineToArgvW")
	cmd16, _ := syscall.UTF16PtrFromString(cmd)
	var argc int32
	argv, _, err := proc.Call(uintptr(unsafe.Pointer(cmd16)), uintptr(unsafe.Pointer(&argc)))
	if argv == 0 {
		return nil, err
	}
	defer syscall.LocalFree(syscall.Handle(argv))
	var args []string
	for i := 0; i < int(argc); i++ {
		p := (*[1 << 16]*uint16)(unsafe.Pointer(argv))[i]
		args = append(args, syscall.UTF16ToString((*[1 << 16]uint16)(unsafe.Pointer(p))[:]))
	}
	return args, nil
}

// FindAllDescendantPids 递归查找所有子进程 PID（含自身）
type wmiProcTree struct {
	ProcessId       uint32 `wmi:"ProcessId"`
	ParentProcessId uint32 `wmi:"ParentProcessId"`
}

func FindAllDescendantPids(rootPid int) []int {
	var procs []wmiProcTree
	_ = wmi.Query("SELECT ProcessId, ParentProcessId FROM Win32_Process", &procs)
	pidSet := map[int]struct{}{rootPid: {}}
	changed := true
	for changed {
		changed = false
		for _, p := range procs {
			if _, ok := pidSet[int(p.ParentProcessId)]; ok {
				if _, exist := pidSet[int(p.ProcessId)]; !exist {
					pidSet[int(p.ProcessId)] = struct{}{}
					changed = true
				}
			}
		}
	}
	var pids []int
	for pid := range pidSet {
		pids = append(pids, pid)
	}
	return pids
}

// IsProcessTreeAlive 检查进程树是否有存活
func IsProcessTreeAlive(rootPid int) bool {
	for _, pid := range FindAllDescendantPids(rootPid) {
		if IsProcessAlive(pid) {
			return true
		}
	}
	return false
}
