# ShellLite（壳轻）

轻量 FinalShell 风格 SSH 桌面客户端：**Go + Wails**，上终端 + 下 SFTP + 系统监控 + 文件传输。

> 仓库模块名仍为 `shelllite`；打包产物为 **ShellLite.exe**。

## Logo

应用图标：`build/appicon.png`（设计稿另存于 `docs/shelllite-logo.png`）

- 深蓝渐变底 — 终端/科技感  
- 薄荷绿 `>_` — SSH 命令行  
- 右上角光点 — 「Lite」轻量寓意  

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
│  └─ shelllite/
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

## 快速开始
1) 安装 Go 1.21+  
2) 构建

```bash
go build ./cmd/shelllite
```

3) 运行（SSH 连接）

```bash
go run ./cmd/shelllite -host 192.168.1.10 -user root -pass 123456 -port 22
# 指定命令（可选）
go run ./cmd/shelllite -host 192.168.1.10 -user root -pass 123456 -cmd "uname -a"
# 交互式 Shell（默认开启）
go run ./cmd/shelllite -host 192.168.1.10 -user root -pass 123456
# 只执行命令，不进入交互式
go run ./cmd/shelllite -host 192.168.1.10 -user root -pass 123456 -shell=false -cmd "ls -la"
```

默认读取当前目录的 config.json；也可通过环境变量或命令行覆盖

```bash
# 环境变量方式（无需配置文件）
# Windows PowerShell
$env:SSH_HOST="192.168.1.10"; $env:SSH_USER="root"; $env:SSH_PASS="123456"; $env:SSH_PORT="22"
go run ./cmd/shelllite

# 指定其他配置文件路径
go run ./cmd/shelllite -config ./config.json
```

4) 运行测试

```bash
go test ./...
```

## 配置
可通过 JSON 文件设置基础参数，示例：

```json
{
  "appName": "ShellLite",
  "logLevel": "INFO",
  "port": 22,
  "host": "192.168.1.10",
  "user": "root",
  "password": "123456"
}
```

- `appName`：应用名称，缺省时为 `ShellLite`
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
go run ./cmd/shelllite -http 127.0.0.1:8080
# 数据库路径可选：环境变量 DB_PATH 或配置中的 dbPath，默认 exe 同目录 data.db
```

- 页面功能：
  - 表单录入主机、端口、账号、密码、启用状态（1/0）
  - 列表展示已保存的连接

- 打包后使用：
  - 将 config.json 和可选 data.db 放在 exe 同目录
  - 运行：ShellLite.exe -http 127.0.0.1:8080

## 打包为 exe
在 Windows 下：

```powershell
go build -trimpath -ldflags "-s -w" -o dist/ShellLite.exe ./cmd/shelllite
# 将 config.json 放到 dist 与 exe 同目录，双击即可按配置连接
```

跨平台（在任意平台交叉编译 Windows）：

```bash
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/ShellLite.exe ./cmd/shelllite
# 运行时会优先读取 exe 所在目录的 config.json，其次读取当前工作目录
```

无外部配置文件运行：
- 设置环境变量或使用命令行参数即可，无需放置 config.json

## 日志
日志统一以等级前缀输出，例如：

```
[INFO] ShellLite 启动
[INFO] 配置: app=ShellLite level=INFO port=0
```

## 约定
- 模块名：`shelllite`
- 布局：命令行入口置于 `cmd/shelllite/`，业务封装放入 `internal/`
- 配置默认值：未指定配置文件或字段缺省时，使用安全默认值
- 仓库忽略：`config.json` 默认被忽略，避免提交环境私密信息

## 后续扩展
可按需增加：
- SSH 连接与操作模块
- 更完善的错误处理与日志输出目标（文件、分级等）
- 配置校验与多环境配置加载
- 交互式 Shell（请求 PTY，绑定标准输入输出）
