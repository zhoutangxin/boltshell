# BoltShell ⚡（闪壳）

> Bolt into your server.  
> 闪电连接，一键运维。

一体化 SSH 桌面客户端：**Go + Wails**，终端 + SFTP + 系统监控 + 跨服务器文件传输。

> Go 模块名：`boltshell`；Wails 打包产物为 **BoltShell.exe**。

## Logo

应用图标：`build/appicon.png`（设计稿：`docs/product/logo/boltshell-logo-icon-v1.png`）

- 电蓝闪电 — 秒连服务器的速度感  
- 终端 `>_` — SSH 命令行  
- 白底圆角 — 现代桌面应用风格  

重新打包后 exe 会使用新图标：`wails build`

## 特性
- 标准布局：`cmd/` + `internal/`
- 配置模块：支持从 JSON 文件加载，带默认值
- 日志模块：封装 `log`，支持 `DEBUG/INFO/WARN/ERROR`
- 单元测试：覆盖配置加载的关键路径

## 目录结构
```
.
├─ cmd/
│  └─ boltshell/
│     └─ main.go           # 程序入口与参数解析
├─ internal/
│  ├─ config/
│  │  ├─ config.go         # 配置结构与 JSON 加载
│  │  └─ config_test.go    # 配置加载测试
│  └─ logging/
│     └─ logging.go        # 简单日志封装
├─ go.mod                  # 模块信息
├─ .gitignore              # 常见忽略规则
└─ README.md               # 项目说明
```

## 开发与打包

本项目为 **Wails v2 桌面应用**（Go + Vue 3）。文档索引见 [`docs/README.md`](docs/README.md)；完整开发说明见 [`docs/engineering/开发调试与正式部署.md`](docs/engineering/开发调试与正式部署.md)。

### 环境

- Go 1.21+、Node.js / npm
- Wails CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0`
- 首次：`cd frontend && npm install`

### 开发

```powershell
cd "E:\resource\person\ShellLite"
wails dev
# 浏览器打开 http://localhost:34115
```

### 打包

```powershell
wails build
# Windows 产物：build\bin\BoltShell.exe
```

### 发版前

1. 改 `internal/version/version.go`、`website/config/release.json`
2. `Copy-Item config\sponsors.remote.json internal\sponsors\remote.embed.json`
3. `wails build` → `.\scripts\deploy-server.ps1`

**无 Mac 打 macOS 包**：push 到 GitHub 后 `git tag v1.0.1 && git push github v1.0.1`，见 [GitHub Actions 说明](docs/engineering/开发调试与正式部署.md#06-github-actions-自动打包推荐无-mac-时打-macos-包)。

### 测试

```powershell
go test ./...
cd frontend; npm run test
```

## 配置
可通过 JSON 文件设置基础参数，示例：

```json
{
  "appName": "BoltShell",
  "logLevel": "INFO",
  "port": 22,
  "host": "192.168.1.10",
  "user": "root",
  "password": "123456"
}
```

- `appName`：应用名称，缺省时为 `BoltShell`
- `logLevel`：日志等级，支持 `DEBUG/INFO/WARN/ERROR`
- `port`：SSH 端口，默认 22
- `host`/`user`/`password`：连接所需的目标、账号与密码

启动参数：
- `-config`：配置文件路径（JSON，未提供时默认读取当前目录 config.json）
- `-verbose`：启用更详细日志（等效于 `DEBUG`）
- `-host` `-user` `-pass` `-port` `-cmd`：可通过命令行提供连接信息与命令
- `-shell`：是否进入交互式 Shell（默认 true）
- 环境变量：`SSH_HOST` `SSH_USER` `SSH_PASS` `SSH_PORT`

## Web 页面与 SQLite
- 启动 Web 页面并使用 SQLite 存储：

```bash
go run ./cmd/boltshell -http 127.0.0.1:8080
# 数据库路径可选：环境变量 DB_PATH 或配置中的 dbPath，默认 exe 同目录 data.db
```

- 页面功能：
  - 表单录入主机、端口、账号、密码、启用状态（1/0）
  - 列表展示已保存的连接

- 打包后使用：
  - 将 config.json 和可选 data.db 放在 exe 同目录
  - 运行：BoltShell.exe -http 127.0.0.1:8080

## 日志
日志统一以等级前缀输出，例如：

```
[INFO] BoltShell 启动
[INFO] 配置: app=BoltShell level=INFO port=0
```

## 约定
- 模块名：`boltshell`
- 布局：命令行入口置于 `cmd/boltshell/`，业务封装放入 `internal/`
- 配置默认值：未指定配置文件或字段缺省时，使用安全默认值
- 仓库忽略：`config.json` 默认被忽略，避免提交环境私密信息

## 后续扩展
可按需增加：
- SSH 连接与操作模块
- 更完善的错误处理与日志输出目标（文件、分级等）
- 配置校验与多环境配置加载
- 交互式 Shell（请求 PTY，绑定标准输入输出）
