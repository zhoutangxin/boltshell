# 部署 BoltShell 到 47.108.138.168
# 用法：在项目根目录执行
#   .\deploy\deploy-server.ps1
# 需已配置 SSH 免密，或执行时会提示输入密码

$ErrorActionPreference = "Stop"
$Server = "root@47.108.138.168"
$RemoteWeb = "/var/www/boltshell/website"
$LocalWeb = "$PSScriptRoot\..\website"
$LocalExe = "$PSScriptRoot\..\server\build\bin\BoltShell.exe"

Write-Host "=== 1. 上传官网静态文件 ===" -ForegroundColor Cyan
scp -r "$LocalWeb\index.html" "$LocalWeb\css" "$LocalWeb\assets" "${Server}:${RemoteWeb}/"
scp -r "$LocalWeb\config" "${Server}:${RemoteWeb}/"

Write-Host "=== 2. 上传安装包到 releases ===" -ForegroundColor Cyan
if (-not (Test-Path $LocalExe)) {
    Write-Host "未找到 $LocalExe，请先 wails build" -ForegroundColor Red
    exit 1
}
$size = (Get-Item $LocalExe).Length
if ($size -lt 512KB) {
    Write-Host "BoltShell.exe 仅 $size bytes，可能已损坏，请先重新 build" -ForegroundColor Red
    exit 1
}
ssh $Server "mkdir -p ${RemoteWeb}/releases"
scp "$LocalExe" "${Server}:${RemoteWeb}/releases/BoltShell-1.0.1.exe"

Write-Host "=== 3. 验证 ===" -ForegroundColor Cyan
ssh $Server "ls -lh ${RemoteWeb}/config/release.json ${RemoteWeb}/releases/BoltShell-1.0.1.exe"
Write-Host "完成。请浏览器访问：" -ForegroundColor Green
Write-Host "  http://47.108.138.168/config/release.json"
Write-Host "  http://47.108.138.168/releases/BoltShell-1.0.1.exe"
