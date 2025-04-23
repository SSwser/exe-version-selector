# exe-version-selector

一个用于 Windows 的本地可执行程序多版本切换与代理工具，支持任务栏托盘菜单、参数代理、应用热切换等功能。

## 主要功能

- 通过命令行或托盘菜单切换当前激活的可执行程序
- 代理所有参数并启动目标应用
- 支持为每个应用配置默认参数，命令行参数可覆盖默认参数
- 托盘图标显示当前激活应用，支持一键切换、快速打开应用目录
- 支持添加、删除、列举应用配置
- 配置文件热更新

## 配置文件说明

配置文件为 `config.yaml`，结构如下：

```yaml
activate: app1         # 当前激活应用名
apps:
  app1:
    path: C:\Path\To\App1.exe   # 可执行文件绝对路径
    args: []                     # 默认启动参数
  app2:
    path: D:\Another\App2.exe
    args: ["-flag"]
```

- `activate`：当前被代理/激活的应用名
- `apps`：应用列表，每个应用包含 `path` 与 `args`

## 命令行用法

- 直接运行 `evs.exe [应用参数...]` 或 `evs-console.exe [应用参数...]`：均可代理并启动当前激活应用，将所有参数传递给目标应用（推荐用 evs.exe，evs-console.exe 适合命令行调试）
- `list`：列出所有已配置应用
- `add <name> <path> [args...]`：添加新应用，可指定默认参数
- `remove <name>`：删除指定应用
- `switch <name>`：切换当前激活应用
- `help`：显示帮助信息

### 示例

```shell
evs.exe --foo-arg1 --foo-arg2
evs.exe switch app2
evs.exe add app3 C:\Path\To\App3.exe -defaultArg
evs.exe remove app1
evs.exe list
evs.exe help
# 或用控制台版
# evs-console.exe switch app2
```

## 托盘菜单

- 启动后会在任务栏显示托盘图标
- 鼠标右键菜单可切换应用、显示当前应用信息、快速打开应用目录、退出程序
- 切换应用后自动更新配置

## 构建与运行

### 直接编译

```shell
go build -ldflags="-H windowsgui" -o evs.exe main.go   # GUI 版（推荐）
go build -o evs-console.exe main.go                    # 控制台版
```

### 使用 PowerShell 脚本（推荐）

```shell
./scripts/build.ps1
```
- 会自动在 build 目录生成 evs.exe（GUI 版）、evs-console.exe（控制台版）、config.yaml、resources 等，打包为 zip 包
- 依赖 Go 1.18+ 环境
- 无需额外 bat 文件，直接运行 exe 即可

### 资源目录

- 托盘图标文件：`resources/icon.ico`
- 默认配置文件：`config.yaml`

---

如需反馈或扩展功能，欢迎 issue 或 PR。
