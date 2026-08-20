# BoltShell 后端架构评审

> 评审时间：2026-08-20  
> 技术栈：Go + Wails v2 + SQLite + golang.org/x/crypto/ssh + pkg/sftp

---

## 一、总体评价

**结论：当前后端设计合理，适合个人/小团队 SSH 客户端阶段；模块边界清晰，可继续迭代。**

| 维度 | 评分 | 说明 |
|------|------|------|
| 模块划分 | ✅ 良好 | `app_*.go` 按域拆分，internal 包职责单一 |
| 并发安全 | ✅ 良好 | `termMu` 保护 sessions map；`sync.Once` 防止重复清理 |
| 连接复用 | ✅ 良好 | SFTP 与 SSH 终端同连接，减少重复登录 |
| 可测试性 | ⚠️ 一般 | App 与 Wails runtime 耦合，单元测试需 mock |
| 安全性 | ⚠️ 待加强 | 密码明文存 SQLite，尚无密钥/Agent 支持 |
| 可扩展性 | ✅ 良好 | 新增 Wails 方法即可扩展前端能力 |

---

## 二、目录与职责

```
main.go              Wails 入口，embed frontend/dist
app.go               App 核心、会话生命周期、连接 CRUD、终端 I/O
app_sftp.go          SFTP 文件操作、跨服传送、临时连接浏览
app_sysinfo.go       远端系统信息采集（shell 脚本）
app_transfer.go      传输进度事件、本地文件夹对话框
internal/
  config/            JSON 配置
  db/                SQLite connections 表
  sshclient/         PTY 终端会话
  sftpclient/        SFTP 上传下载、目录列表
  logging/           日志级别
  gui/               遗留 Gio GUI（与 Wails 并行，可逐步废弃）
```

---

## 三、核心数据流

### 3.1 建立 SSH 会话

```
前端 StartSession(connID)
  → db.GetByID
  → sshclient.NewTerminalSession (PTY)
  → sftpclient.NewFromSSH (同连接)
  → sessions[sid] = terminalHolder
  → goroutine: terminalReadLoop → EventsEmit("terminal-output")
  → goroutine: term.Wait → finalizeSession → EventsEmit("terminal-closed")
```

### 3.2 SFTP 文件操作

```
前端 ListRemoteDir(sessionID, path)
  → getSFTP(sessionID)  // 复用已登录连接
  → sftpclient.ListDir
```

### 3.3 跨服务器传送

```
前端 TransferToConnection(srcSession, srcPath, targetConnID, dstDir)
  → 源：getSFTP(srcSession)
  → 目标：dialConnectionSFTP(targetConnID)  // 临时连接
  → doTransferWithLog → EventsEmit 进度
```

---

## 四、设计优点

1. **SSH + SFTP 同连接**：`terminalHolder` 同时持有 `term` 和 `sftp`，符合 FinalShell/Xshell 用户预期。
2. **多 Tab 独立会话**：每次 `StartSession` 生成新 sessionID，支持同主机多 Tab。
3. **输出编码兜底**：`decodeRemote` 处理 GB18030，适配国内 Linux 服务器。
4. **传输进度事件化**：`transfer-update` / `srv-transfer-progress` 解耦前后端，UI 可独立展示。
5. **DB 逻辑删除**：`deleted` 字段软删除，可恢复连接配置。
6. **按文件拆分 API**：SFTP / SysInfo / Transfer 独立文件，便于维护。

---

## 五、待改进项（按优先级）

### 🔴 高优先级

| 问题 | 建议 |
|------|------|
| **密码明文存储** | 接入 OS Keychain 或 AES-256-GCM（主密钥存 Keychain），见 `docs/TODO.md` |
| **临时连接无并发限制** | ✅ 已加 browse 连接池（60s 内复用 SSH，见 `app_browse_pool.go`） |
| **会话泄漏风险** | `CloseSession` 与 `finalizeSession` 两条路径，确保所有出口都释放 SFTP |

### 🟡 中优先级

| 问题 | 建议 |
|------|------|
| **SysInfo shell 脚本** | 改为解析 `/proc` 的 Go 原生实现或缓存上次结果，减少 SSH 往返 |
| **无 SSH 密钥登录** | 支持 `IdentityFile`、ssh-agent → **已列入 `docs/TODO.md`，v1.5+** |
| **App 结构体过大** | 可拆 `SessionManager`、`ConnectionService` 便于测试 |
| **Gio GUI 遗留** | ✅ 已删除，CLI 入口见 `cmd/boltshell`，桌面 UI 仅 Wails |

### 🟢 低优先级

| 问题 | 建议 |
|------|------|
| **端口默认值** | AddConnection 允许 port=0，应在 db 层默认 22 |
| **连接超时配置** | sshclient Dial 超时硬编码，应进 config.json |
| **日志落盘** | 目前仅 stdout，生产环境建议文件日志 |

---

## 六、Wails 绑定清单

### Go → 前端方法（30+）

- 连接：`ListConnections` `AddConnection` `UpdateConnection` `SetDeleted`
- 终端：`StartSession` `CloseSession` `SendSessionInput` `ResizeSession`
- SFTP：`ListRemoteDir` `UploadToRemote` `DownloadFromRemote` …
- 系统：`GetSessionSysInfo`
- 对话框：`PickLocalFile` `PickDownloadDir` …

### 前端 → Go 事件

| 事件 | 用途 |
|------|------|
| `terminal-output` | SSH 输出 → xterm |
| `terminal-closed` | Tab 标记断开 |
| `transfer-update` | 本地传输进度 |
| `transfer-log` | 跨服传送日志 |
| `srv-transfer-progress` | 跨服传送进度 |

---

## 七、推荐演进路线

```
v0.x（现在）     v1.0              v1.5+
────────────────────────────────────────────
当前架构        密码加密存储        SSH 密钥/agent
按文件拆分 ✅    SessionManager     连接池/超时配置
               废弃 Gio GUI        Team 云同步 API
```

---

*评审结束。当前架构可支撑 v1.0 发布，安全与密钥是下一版本重点。*
