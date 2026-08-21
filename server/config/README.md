# BoltShell `config/` 配置说明

> 本目录存放**开发者侧**的远程入口与离线兜底模板，随仓库维护、发版时同步。  
> **不是**用户日常编辑的 `config.json`（运行时配置见 [`config.example.json`](../config.example.json)）。

相关文档：

- [赞助位配置说明](../../docs/business/BoltShell_赞助位配置说明.md)（slot 字段细则）
- [开发调试与正式部署](../../docs/engineering/开发调试与正式部署.md)（发版步骤）
- [文档索引](../../docs/README.md)

---

## 一、文件一览

| 文件 | 角色 | 谁改 | 最终落点 |
|------|------|------|----------|
| [`sponsors.remote.json`](sponsors.remote.json) | 远程服务入口（赞助 / 升级 / 购买页 URL） | 开发者 | 打包进 exe（`remote.embed.json`），也可用 exe 旁 `config/` 覆盖 |
| [`sponsors.default.json`](sponsors.default.json) | 赞助位内容的本地 fallback | 开发者 | 开发断网时用；正式内容以服务器 `sponsors.json` 为准 |
| [`release.default.json`](release.default.json) | 升级信息模板 | 开发者 | **样例**；线上真实文件是 `website/config/release.json` |

---

## 二、`sponsors.remote.json` — 远程地址总入口

告诉客户端「去哪拉赞助配置」「去哪检查更新」「Pro 购买页是哪」。

```json
{
  "remoteURL": "http://example.com/config/sponsors.json",
  "releaseURL": "http://example.com/config/release.json",
  "proUpgradeURL": "http://example.com/#pricing",
  "analyticsURL": "http://example.com/api/v1/analytics/events",
  "analyticsAppKey": "boltshell-desktop",
  "analyticsAppSecret": "change-me"
}
```

| 字段 | 说明 |
|------|------|
| `remoteURL` | 远程赞助配置 JSON 地址（广告/赞助位内容） |
| `releaseURL` | 远程升级信息 JSON 地址（客户端 `CheckForUpdate`） |
| `proUpgradeURL` | Pro 升级/购买页；赞助位里也可单独配 `proUpgradeUrl` |
| `analyticsURL` | 匿名统计批量上报地址（`POST /api/v1/analytics/events`） |
| `analyticsAppKey` | 统计 AppKey（可进包） |
| `analyticsAppSecret` | HMAC 密钥（进包；仅防君子 + 服务端限流） |

### 加载顺序

```
exe 旁 / 向上查找 config/sponsors.remote.json
    ↓ 没有或无效
编译进 exe 的 internal/sponsors/remote.embed.json
```

### 发版必做

改完本文件后同步内置副本，再打包：

```powershell
Copy-Item config\sponsors.remote.json internal\sponsors\remote.embed.json
wails build
```

CI（GitHub Actions）也会自动执行上述拷贝。

> 换服务器 IP/域名：只改本文件 → 同步 embed → 重新打包。用户不必改自己的 `config.json`。

---

## 三、`sponsors.default.json` — 赞助位离线兜底内容

结构与服务器上的 `sponsors.json`（如 `website/config/sponsors.json`）一致，用于：

- 本地开发、断网、远程 404 时的 fallback
- **不是**「关闭广告」的开关；真正去广告靠 Pro License

| 字段 | 说明 |
|------|------|
| `version` | 配置版本号 |
| `updatedAt` | 更新日期（展示/排查用） |
| `cacheTTLSeconds` | 客户端内存缓存秒数，默认 `21600`（6 小时） |
| `proUpgradeUrl` | Pro 购买页（可与 remote 里的 `proUpgradeURL` 互补） |
| `slots` | 赞助位字典；key 为 slotId |

### 内置 slotId

| slotId | 位置 | `type` |
|--------|------|--------|
| `quick_connect_bottom` | 未连接时快速连接页底部 | `banner` |
| `sidebar_1` / `sidebar_2` | 连接后左侧系统信息栏 | `compact` |

单个 slot 常用字段：`enabled`、`type`、`badge`、`title`、`desc`、`linkUrl`、`imageUrl`（可选）、`dismissDays`（点 × 隐藏天数，默认 7）。

更完整字段说明见 [赞助位配置说明](../../docs/business/BoltShell_赞助位配置说明.md)。

### 和内置 default 的关系

- 仓库还有 `internal/sponsors/default.json`（编译进 exe 的最后一层兜底）
- 开发时优先找：`{exe}/config/sponsors.json` → … → **`config/sponsors.default.json`** → 内置 `default.json`

日常改广告文案：改**服务器**上的 `sponsors.json`；本文件只保证离线仍有合理展示。

---

## 四、`release.default.json` — 升级信息模板

客户端通过 `sponsors.remote.json` 的 `releaseURL` 拉取线上的 `release.json`。  
本文件是**字段样例 / 本地参考**，不会被 Go 代码直接读取。

线上真实文件：

| 路径 | 用途 |
|------|------|
| [`website/config/release.json`](../../website/config/release.json) | 部署到官网后，供客户端检查更新 |

```json
{
  "version": "1.0.2",
  "buildNumber": 3,
  "releaseNotes": "发行说明…",
  "downloadURL": "http://example.com/releases/BoltShell-1.0.2.exe",
  "mandatory": false,
  "publishedAt": "2026-08-20"
}
```

| 字段 | 说明 |
|------|------|
| `version` | 最新版本号，与客户端 `internal/version/version.go` 比较 |
| `buildNumber` | 构建号（辅助） |
| `releaseNotes` | 升级弹窗说明 |
| `downloadURL` | 安装包下载地址（Windows 等） |
| `mandatory` | 是否强制升级 |
| `publishedAt` | 发布时间 |

发版时改 `website/config/release.json`（或同步本模板字段），部署网站后再发客户端包。

---

## 五、和「用户 config.json」的区别

| | 项目 `config/` | 用户 `config.json` |
|--|----------------|-------------------|
| 位置 | 仓库 `server/config/`，可随包放在 `{exe}/config/` | exe 同目录或工作目录 |
| 用途 | 远程 URL、赞助兜底、升级模板 | 日志级别、DB 路径、CLI 默认 SSH 等 |
| 是否提交 | ✅ 提交（无密钥） | ❌ `.gitignore` 忽略 |
| 模板 | 本目录各 json | [`config.example.json`](../config.example.json) |
| Pro | 不管 License | `proLicensed` **仅** `BOLTSHELL_DEV=1` 时有效 |

复制示例：

```powershell
Copy-Item config.example.json config.json
# 按需填写后放在 exe 旁；勿把含密码的 config.json 提交进 git
```

---

## 六、相关路径速查

| 类型 | 路径 |
|------|------|
| 远程赞助内容（线上） | 由 `remoteURL` 指向，常见 `website/config/sponsors.json` |
| 远程升级信息（线上） | 由 `releaseURL` 指向，常见 `website/config/release.json` |
| 编译进 exe 的远程入口 | `internal/sponsors/remote.embed.json` |
| 编译进 exe 的赞助兜底 | `internal/sponsors/default.json` |
| 用户侧缓存 / 关闭状态 / License | `%LOCALAPPDATA%\BoltShell\`（macOS / Linux 见赞助位文档） |

---

## 七、常见操作

| 想做什么 | 改哪里 |
|----------|--------|
| 换赞助/升级服务器地址 | `sponsors.remote.json` → 同步 `remote.embed.json` → 重新打包 |
| 改线上广告文案 | 服务器 `sponsors.json`（如官网 `website/config/sponsors.json`） |
| 本地断网仍能看到赞助位 | 调 `sponsors.default.json` |
| 发新版本供客户端检测 | `website/config/release.json`（可对照本目录 `release.default.json`） |
| 调日志 / DB / 开发模拟 Pro | 用户 `config.json`（来自 `config.example.json`） |
