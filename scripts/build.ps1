# 清空 build 目录内容（保留 build 目录本身）
if (Test-Path "build") {
    Get-ChildItem -Path "build" -Recurse -Force | Remove-Item -Recurse -Force
}

# 创建必要的目录
foreach ($dir in @("build", "dist")) {
    if (-not (Test-Path -Path $dir)) {
        New-Item -ItemType Directory -Path $dir
    }
}

# 设置输出路径
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# 构建控制台版（调试用）
go build -o build/evs-console.exe main.go

# 构建 GUI 版（发布用，无黑窗）
go build -ldflags="-H windowsgui" -o build/evs.exe main.go

if ($LASTEXITCODE -eq 0) {
    # 拷贝配置文件和资源到 bin 目录
    if (-not (Test-Path -Path "build/config.yaml")) {
        Copy-Item -Path "config.yaml" -Destination "build/config.yaml"
        Write-Host "配置文件已复制到 build 目录" -ForegroundColor Green
    }
    if (Test-Path "resources") {
        Copy-Item -Path "resources" -Destination "build/resources" -Recurse -Force
        Write-Host "已复制 resources 到 build 目录" -ForegroundColor Green
    }


    # 创建带时间戳的zip文件名
    $timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
    $zipName = "dist/evs_${timestamp}.zip"
    
    # 压缩 build 目录
    Compress-Archive -Path "build/*" -DestinationPath $zipName -Force
    Write-Host "已创建压缩包：$zipName" -ForegroundColor Green
    
    Write-Host "构建成功！执行文件位于 build/evs.exe 和 build/evs-console.exe" -ForegroundColor Green
}
else {
    Write-Host "构建失败！" -ForegroundColor Red
    exit 1
}
