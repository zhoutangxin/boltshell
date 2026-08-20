# BoltShell 后端架构评审

> 评审时间：2026-08-20（文档已按当前仓库补全文件级说明）  
> 技术栈：Go + Wails v2 + SQLite（`modernc.org/sqlite`）+ `golang.org/x/crypto/ssh` + `github.com/pkg/sftp`  
> 文档索引：[docs/README.md](../README.md) · 关联：[前端结构](BoltShell_前端结构说明.md)

桌面 UI 只走 Wails（`server/main.go`）。`server/cmd/boltshell` 是独立命令行入口，不是 Gio GUI。

---

## 一、总体评价

**结论：当前后端设计合理，适合个人/小团队 SSH 客户端阶段；模块边界清晰，可继续迭代。**

| 维度 | 评分 | 说明 |
|------|------|------|
| 模块划分 | ✅ 良好 | `app_*.go` 按域拆分，`internal` 包职责单一 |
| 并发安全 | ✅ 良好 | `termMu` 保护 sessions；`sync.Once` 防止重复清理；browse 池有独立锁 |
| 连接复用 | ✅ 良好 | SFTP 与 SSH 终端同连接；浏览目录 60s 内复用临时 SSH |
| 可测试性 | ⚠️ 一般 | App 与 Wails runtime 耦合；`emitEvent` 在 `ctx==nil` 时跳过，便于单测 |
| 安全性 | ⚠️ 待加强 | 密码明文存 SQLite；HostKey 未校验；尚无生产密钥/Agent 登录 |
| 可扩展性 | ✅ 良好 | 新增导出方法即可给前端用 |

---

## 二、仓库后端目录（每个文件做什么）

```
boltshell/
├── deploy/                  # 发版脚本、图标生成
├── web/                     # Vue 前端
└── server/                  # Go 后端 + Wails
    ├── main.go              # Wails 桌面入口，embed frontend/dist
    ├── app.go               # App 核心：启动、连接 CRUD、SSH 会话生命周期
    ├── app_sftp.go          # SFTP API、跨服传送、本地选文件
    ├── app_transfer.go      # 上传/下载进度事件、打开本地下载目录
    ├── app_sysinfo.go       # 远端 CPU/内存/磁盘/进程
    ├── app_browse_pool.go   # BrowseConnectionDir 临时 SSH 连接池
    ├── app_sponsors.go      # 赞助位配置 + Pro 判断给前端
    ├── app_upgrade.go       # 版本号、检查更新、应用更新
    ├── app_groups.go        # 连接分组
    ├── app_connections_io.go # 连接导入导出
    ├── go.mod / go.sum
    ├── wails.json           # frontend:dir = ../web
    ├── cmd/boltshell/main.go
    └── internal/
        ├── config/          # config.json
        ├── db/              # SQLite connections 表
        ├── sshclient/       # SSH Dial、PTY、Run 命令
        ├── sftpclient/      # SFTP 列表/读写/上传下载
        ├── logging/         # 分级日志
        ├── appdata/         # 用户数据目录、开发模式开关
        ├── version/         # 客户端版本字符串
        ├── updater/         # 拉 release.json、Windows 替换 exe
        ├── sponsors/        # 远程赞助配置、缓存、关闭记录
        └── license/         # Pro license.dat 校验
```

已删除：`internal/gui`（Gio）。桌面界面只保留 Wails。

---

## 三、Wails 绑定层（`package main`）

这些文件组成绑定到前端的 `App` 对象。Wails 会把 **导出的方法**（大写开头、可 JSON 序列化）生成到 `web/wailsjs/go/main/App.js`。

### `main.go`

`wails.Run`：标题 BoltShell、最大化、嵌入 `frontend/dist`、`OnStartup: app.startup`、`Bind: []{app}`。无业务逻辑。

### `app.go`

`App` 结构体与会话核心。

**结构体字段（按职责）：**

| 字段 | 作用 |
|------|------|
| `ctx` | Wails 运行时，用于 `EventsEmit`、系统对话框 |
| `db` | SQLite，存连接配置 |
| `logger` | 分级日志 |
| `termMu` + `sessions` | sessionID → `terminalHolder`（PTY + 同连接 SFTP） |
| `browseMu` + `browsePool` | 跨服浏览用的临时 SFTP |
| `userDataDir` | 赞助缓存、license 等用户目录 |
| `remoteConfig` / `sponsor*` / `updaterClient` | 赞助 URL、更新检查 |
| `proLicensedDev` | 仅开发模式可读的 config 开关 |

**`terminalHolder`：** 一个 SSH Tab。`term` 是 PTY；`sftp` 在 `StartSession` 时用同一 `ssh.Client` 创建；`finalize sync.Once` 保证关闭只跑一次。

**生命周期：**

1. `NewApp` → `startup` → `initBackend`：读 `config.json`、开库、`InitSchema`、日志、用户目录、赞助/更新客户端。
2. `StartSession(connID)`：`db.GetByID` → `sshclient.NewTerminalSession` → 可选 SFTP → 写入 `sessions` → goroutine 读 stdout 发 `terminal-output` → `term.Wait` 后 `finalizeSession` 发 `terminal-closed`。
3. `SendSessionInput` / `ResizeSession` / `CloseSession`：输入、PTY 尺寸、主动关。
4. `decodeRemote`：非法 UTF-8 时按 GB18030 转，适配国内 Linux。

**连接 CRUD（给连接管理器）：**

- `ListConnections(includeDeleted, groupFilter)`
- `AddConnection` / `UpdateConnection`
- `SetDeleted`：软删除，`deleted` 为 0/1

### `app_sftp.go`

已登录会话走 `getSFTP(sessionID)`（复用 Tab 的 SFTP）。未开 Tab 的浏览走 `dialConnectionSFTP` + browse 池。

| 方法 | 作用 |
|------|------|
| `ListActiveSessions` | 当前有 SFTP 的 Tab，给「传到另一已开会话」 |
| `GetRemoteHome` / `ListRemoteDir` | 家目录、列目录 |
| `MkdirRemote` / `RemoveRemote` / `RenameRemote` | 建目录、删、改名 |
| `ReadRemoteFile` / `WriteRemoteFile` | 在线编辑 |
| `UploadToRemote` / `DownloadFromRemote` | 本地↔远端（进度在 `app_transfer.go`） |
| `PickLocalFile` / `PickLocalDir` / `PickSaveFile` | 系统文件对话框 |
| `TransferBetweenServers` | 两个已开会话之间传 |
| `TransferToConnection` | 当前会话 → 目标连接（可临时 Dial） |
| `GetConnectionHome` / `BrowseConnectionDir` | 传送对话框浏览目标机 |

内部：`doTransferWithLog`、`transferFileBetween`、`transferDirBetween` 发 `transfer-log` 和进度回调。

### `app_transfer.go`

把 SFTP 进度收成 `TransferEvent`，事件名 `transfer-update`。

- `uploadWithProgress` / `downloadWithProgress`：status 为 `running` / `done` / `error`
- `PickDownloadDir`：选本地下载根目录
- `OpenLocalFolder`：用系统资源管理器打开目录（Windows `explorer` 等）

### `app_sysinfo.go`

`GetSessionSysInfo`：在已有 SSH 上 `sshclient.RunOnClient` 跑 shell，解析 `/proc/stat`、`free`、`df`、`ps`。返回 `SysInfo`。多一次往返且依赖远端命令，见「待改进」。

### `app_browse_pool.go`

`BrowseConnectionDir` / `GetConnectionHome` 的连接池。同一 `connID` 在 **60 秒**空闲期内复用 SSH，避免每次点目录都握手。过期由 `evictStaleBrowseConnsLocked` 关掉。`dial` 竞态时关掉旧连接只留新的。

### `app_sponsors.go`

给前端的赞助视图（过滤未启用、未过期关闭、Pro 则空 Slots）。

- `GetSponsorConfig(forceRefresh)` / `RefreshSponsorConfig`
- `DismissSponsorSlot(slotID, days)`（默认 7 天）
- `IsProLicensed`
- 固定顺序：`quick_connect_bottom`、`sidebar_1`、`sidebar_2`

### `app_upgrade.go`

- `GetAppVersion` → `internal/version`
- `CheckForUpdate` → 拉远程 `release.json`
- `ApplyUpdate` → 下载并安装（Windows 替换 exe 并重启）

### 测试文件

| 文件 | 覆盖 |
|------|------|
| `app_session_test.go` | 无真实 SSH：`ctx==nil` 时不发事件；finalize / CloseSession |
| `app_browse_pool_test.go` | 池复用与过期清理 |

---

## 四、`internal` 包（每个文件）

### `internal/config/`

| 文件 | 作用 |
|------|------|
| `config.go` | `Config`：`appName`、`logLevel`、默认 SSH、`dbPath`、`proLicensed`（仅开发模式）。`Default()` / `Load(path)` |
| `config_test.go` | 默认值与 JSON 加载 |

Wails 与 CLI 都读 exe 旁或工作目录的 `config.json`。

### `internal/db/`

| 文件 | 作用 |
|------|------|
| `sqlite.go` | 纯 Go SQLite。表 `connections`：ID 手工随机串（**无 AUTO_INCREMENT**）。`Enabled`/`Deleted` 为整数 0/1。`Open`、`InitSchema`、`ensureColumn` 做简单迁移。CRUD：`Insert`/`List`/`Update`/`GetByID`/`SetDeleted`/`NewID` |

密码目前明文列存储。

### `internal/sshclient/`

| 文件 | 作用 |
|------|------|
| `sshclient.go` | `Dial` / `Connect`（测通）、`Run` / `RunOnClient`（执行命令）、`NewTerminalSession`（PTY，给 Wails）、`Interactive`（CLI 本机 raw 终端）。`Resize`、`Wait`、`Close`、`SSHClient()` 给 SFTP 复用。TCP KeepAlive。超时约 10s。`HostKeyCallback: InsecureIgnoreHostKey()` |

### `internal/sftpclient/`

| 文件 | 作用 |
|------|------|
| `sftpclient.go` | `NewFromSSH`。列目录、Home、mkdir/remove/rename、读写文件、文件/目录上传下载（`ProgressFunc`）。路径按 POSIX 整理。 |

### `internal/logging/`

| 文件 | 作用 |
|------|------|
| `logging.go` | 包一层 `log.Logger`：`DEBUG/INFO/WARN/ERROR`，低于阈值丢弃。默认 stdout。 |

### `internal/appdata/`

| 文件 | 作用 |
|------|------|
| `paths.go` | `Dir()`：Windows `%LOCALAPPDATA%\BoltShell`，macOS Application Support，Linux `~/.config/boltshell`。`IsDevMode()`：`BOLTSHELL_DEV=1` 才允许 config 里的 Pro 开关。 |

赞助缓存、`sponsor.state`、`license.dat` 放这里，不跟 exe 混放。

### `internal/version/`

| 文件 | 作用 |
|------|------|
| `version.go` | `Version` 默认字符串；可用 ldflags `-X boltshell/internal/version.Version=...` 覆盖。`Current()` 给检查更新。 |

### `internal/updater/`

| 文件 | 作用 |
|------|------|
| `updater.go` | `Client.Check()` 拉 `release.json`，`Compare` 语义化版本。`Apply` 下载、Windows 校验 PE 后替换并重启。 |
| `updater_test.go` | 版本比较、exe 校验 |

### `internal/sponsors/`

| 文件 | 作用 |
|------|------|
| `sponsors.go` | `Client.Load`：远程 → 磁盘缓存 → 本地 json → embed `default.json`。HMAC 保护的 `DismissStore`（用户关掉某 slot 到何时）。 |
| `remote.go` | `sponsors.remote.json` / embed `remote.embed.json`：`remoteURL`、`releaseURL`、`proUpgradeURL`。发版前需把 `config/sponsors.remote.json` 同步进 embed。 |
| `default.json` / `remote.embed.json` | 内置兜底 |
| `*_test.go` | 默认配置、关闭存储、远程配置加载 |

### `internal/license/`

| 文件 | 作用 |
|------|------|
| `license.go` | `IsPro`：用户目录 `license.dat` HMAC 校验；开发模式可用 `BOLTSHELL_PRO` 或 config。`SaveLicenseForDev` 方便本地造许可证。 |
| `license_test.go` | 开发开关行为 |

Pro 为真时 `GetSponsorConfig` 不返回赞助 Slots。

---

## 五、CLI：`cmd/boltshell/main.go`

不启动 Wails。解析 `-config`、`-host/-user/-pass/-port`、`-cmd`、`-shell`、`-verbose`；环境变量 `SSH_*`、`DB_PATH`。打开同一套 SQLite 后：交互 Shell 或执行一条远程命令。给脚本和无 GUI 环境用，与桌面会话池无关。

---

## 六、核心数据流

### 6.1 建立 SSH 会话

```
前端 StartSession(connID)
  → db.GetByID
  → sshclient.NewTerminalSession (PTY)
  → sftpclient.NewFromSSH (同 ssh.Client)
  → sessions[sid] = terminalHolder
  → goroutine: terminalReadLoop → EventsEmit("terminal-output")
  → goroutine: term.Wait → finalizeSession → EventsEmit("terminal-closed")
```

### 6.2 SFTP（已开 Tab）

```
前端 ListRemoteDir(sessionID, path)
  → getSFTP(sessionID)
  → sftpclient.ListDir
```

### 6.3 跨服务器传送（目标可未开 Tab）

```
前端 TransferToConnection(srcSession, srcPath, targetConnID, dstDir, taskID)
  → 源：getSFTP(srcSession)
  → 目标：getBrowseSFTP / dialConnectionSFTP
  → doTransferWithLog → transfer-log + srv-transfer-progress
```

浏览目标目录：`BrowseConnectionDir` → `getBrowseSFTP`（60s 复用）。

### 6.4 检查更新

```
前端 CheckForUpdate
  → updater.Client 拉 remoteConfig.ReleaseURL
  → 比较 version.Current() 与远程 Version
前端 ApplyUpdate(url)
  → 下载 → 校验 → 替换进程文件 → 重启
```

---

## 七、设计优点

1. **SSH + SFTP 同连接**：一个 Tab 一份 `terminalHolder`，符合 FinalShell/Xshell 习惯。
2. **多 Tab 独立会话**：每次 `StartSession` 新 sessionID，同主机可开多个 Tab。
3. **输出编码兜底**：`decodeRemote` 处理 GB18030。
4. **传输进度事件化**：`transfer-update` / `srv-transfer-progress` / `transfer-log`，UI 可独立画。
5. **DB 软删除**：`deleted` 可恢复配置。
6. **按文件拆 API**：SFTP / SysInfo / Transfer / Sponsors / Upgrade 分开。
7. **浏览连接池**：传送对话框不会每次点文件夹都重新登录。
8. **用户数据与 exe 分离**：赞助缓存和 license 在 `appdata.Dir()`。

---

## 八、待改进项（按优先级）

### 高优先级

| 问题 | 建议 |
|------|------|
| **密码明文存储** | OS Keychain 或 AES-256-GCM（主密钥进 Keychain），见 [`docs/TODO.md`](../TODO.md) |
| **HostKey 不校验** | `InsecureIgnoreHostKey` 有 MITM 风险，应存 known_hosts |
| **会话泄漏** | `CloseSession` 与 `finalizeSession` 双路径，所有出口都要关 SFTP |

临时连接并发：browse 池已做（60s 复用）。

### 中优先级

| 问题 | 建议 |
|------|------|
| **SysInfo 靠 shell** | 改解析 `/proc` 的 Go 实现或缓存，少一次 SSH 往返 |
| **无密钥登录** | `IdentityFile`、ssh-agent → [`docs/TODO.md`](../TODO.md)，v1.5+ |
| **App 结构体偏大** | 可拆 `SessionManager`、`ConnectionService` 方便测 |
| **Gio GUI** | 已删除；CLI 在 `cmd/boltshell` |

### 低优先级

| 问题 | 建议 |
|------|------|
| **端口默认值** | `AddConnection` 允许 port=0，宜在 db 层默认 22 |
| **连接超时** | Dial 超时写死，宜进 `config.json` |
| **日志落盘** | 目前 stdout，生产可加文件日志 |

---

## 九、Wails 绑定清单

### Go → 前端方法

**连接：** `ListConnections` `AddConnection` `UpdateConnection` `SetDeleted`

**终端：** `StartSession` `CloseSession` `SendSessionInput` `ResizeSession`

**SFTP：** `ListRemoteDir` `GetRemoteHome` `MkdirRemote` `RemoveRemote` `RenameRemote` `ReadRemoteFile` `WriteRemoteFile` `UploadToRemote` `DownloadFromRemote` `ListActiveSessions` `TransferBetweenServers` `TransferToConnection` `GetConnectionHome` `BrowseConnectionDir`

**系统：** `GetSessionSysInfo`

**对话框：** `PickLocalFile` `PickLocalDir` `PickSaveFile` `PickDownloadDir` `OpenLocalFolder`

**赞助 / 许可：** `GetSponsorConfig` `RefreshSponsorConfig` `DismissSponsorSlot` `IsProLicensed`

**升级：** `GetAppVersion` `CheckForUpdate` `ApplyUpdate`

### 前端 → Go 事件（后端 `EventsEmit`）

| 事件 | 用途 |
|------|------|
| `terminal-output` | SSH 输出 → xterm |
| `terminal-closed` | Tab 标断开 |
| `transfer-update` | 本地上传/下载进度 |
| `transfer-log` | 跨服传送日志行 |
| `srv-transfer-progress` | 跨服传送字节进度 |

---

## 十、推荐演进路线

```
v0.x（现在）          v1.0                 v1.5+
────────────────────────────────────────────────
当前架构              密码加密存储           SSH 密钥 / agent
按文件拆分 ✅         SessionManager        连接池 / 超时进配置
browse 池 ✅          known_hosts          Team 云同步 API
赞助 / 应用内升级 ✅
CLI 与 Wails 分离 ✅
```

---

*评审结论：架构可支撑 v1.0。下一版本重点是凭据加密、主机指纹、密钥登录。*
