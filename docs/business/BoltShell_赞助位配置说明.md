# 赞助位远程配置说明

> 相关文档：[广告术语与指标](BoltShell_广告术语与指标说明.md) · [商业化策略](BoltShell_商业化与开源策略.md) · [文档索引](../README.md)

客户端启动时由 Go 后端拉取远程 `sponsors.json`，断网或拉取失败时使用本地 fallback。

## 远程地址（开发者配置，非用户 config.json）

远程 URL 写在**项目内** `config/sponsors.remote.json`，打包时随 `config/` 目录分发，**不是**让用户在 exe 旁的 `config.json` 里填。

```json
{
  "remoteURL": "http://192.168.1.100/config/sponsors.json"
}
```

支持 IP 或域名。发版前改此文件并重新打包即可；用户无需也不应修改。

> 未配置或文件不存在时跳过远程，直接使用本地 fallback。

## 字段规范

| 字段 | 类型 | 说明 |
|------|------|------|
| `version` | int | 配置版本号 |
| `updatedAt` | string | 更新时间 |
| `cacheTTLSeconds` | int | 客户端缓存秒数，默认 21600（6h） |
| `proUpgradeUrl` | string | Pro 购买页 |
| `slots` | object | 赞助位字典，key 为 slotId |

### slot 对象

| 字段 | 说明 |
|------|------|
| `enabled` | 是否启用 |
| `type` | `banner`（快速连接底）/ `compact`（侧栏） |
| `badge` | 角标文字 |
| `title` / `desc` | 标题与描述 |
| `linkUrl` | 点击跳转 |
| `imageUrl` | 可选图片 |
| `dismissDays` | 用户点 × 后隐藏天数，默认 7 |

### 内置 slotId

- `quick_connect_bottom` — 未连接时快速连接页底部
- `sidebar_1` / `sidebar_2` — 连接后左侧系统信息栏底部

## 客户端读取路径（完整）

> 前端不直接读文件，统一走 Go API：`GetSponsorConfig()` → `internal/sponsors/sponsors.go`

### 加载优先级

```
内存缓存（TTL 内）
    ↓
远程 URL（config/sponsors.remote.json 的 remoteURL，支持 IP）
    ↓ 失败（断网 / 404 / 未配置）
本地 sponsors.json（exe 旁 / 项目 config/）
    ↓
磁盘缓存 sponsors.cache（上次成功拉取的副本）
    ↓
内置 default.json（编译进 exe）
```

### 各路径一览

| 类型 | 路径 | 说明 |
|------|------|------|
| **远程（正式主来源）** | `config/sponsors.remote.json` → `remoteURL` | 开发者配置；打包后位于 `{exe}/config/sponsors.remote.json` |
| **本地 fallback** | `{exe}/config/sponsors.json` | 按顺序找第一个存在的文件 |
| | `{exe}/sponsors.json` | |
| | `config/sponsors.json` | 开发时工作目录 |
| | `config/sponsors.default.json` | 项目内默认（开发常见） |
| **磁盘缓存** | `%LOCALAPPDATA%\BoltShell\sponsors.cache` | 远程成功后写入；远程失败时备用 |
| **内置默认** | `internal/sponsors/default.json` | 编译进 exe，最后一层 fallback |
| **点 × 关闭状态** | `%LOCALAPPDATA%\BoltShell\sponsor.state` | 非广告内容；7 天临时隐藏 |
| **Pro 授权** | `%LOCALAPPDATA%\BoltShell\license.dat` | 有则永久不展示赞助位 |

macOS：`~/Library/Application Support/BoltShell/`  
Linux：`~/.config/BoltShell/`

### 源码入口

| 文件 | 职责 |
|------|------|
| `config/sponsors.remote.json` | 远程 URL（开发者改，随包分发） |
| `app_sponsors.go` | Wails API、`resolveSponsorLocalPath` |
| `internal/sponsors/remote.go` | 读取 sponsors.remote.json |
| `internal/sponsors/sponsors.go` | 远程拉取、缓存、Load 优先级 |
| `frontend/src/composables/useSponsors.ts` | 前端启动时调用 |

### 你怎么改广告？

| 场景 | 改哪里 |
|------|--------|
| 换服务器 / IP | 改 `config/sponsors.remote.json` 后重新打包 |
| 改广告内容 | 服务器上的 `sponsors.json`（如 `website/config/sponsors.json`） |
| 本地开发 | `config/sponsors.default.json` 作断网 fallback |
| 断网测试 | 拔掉网络，应自动 fallback 到本地/内置默认 |

## 客户端行为（摘要）

1. 优先拉 `config/sponsors.remote.json` 里的 `remoteURL`（支持 IP，不在 Go 代码里写死）
2. 失败则读本地 `config/sponsors.default.json` 或内置默认（**fallback 内容，非关闭开关**）
3. **Pro 用户**：验证 `%LOCALAPPDATA%\BoltShell\license.dat`；开发模式 `BOLTSHELL_DEV=1` 可模拟
4. **点 × 关闭**：写入用户目录 `sponsor.state`（带签名）；篡改无效；删除文件则赞助位恢复

## 安全说明（必读）

| 问题 | 现状与对策 |
|------|------------|
| 改 `config.json` 的 `proLicensed` | **正式包无效**；仅 `BOLTSHELL_DEV=1` 开发模式可读 |
| 用户改 `sponsors.remote.json` | 随包分发，普通用户一般不会改；核心去广告仍靠 Pro License |
| 改 `sponsors.default.json` | 只影响离线 fallback 文案 |
| 真正去广告 | 购买 Pro → 官网签发 `license.dat` |

## 修改配置（无后台阶段）

1. 编辑 `config/sponsors.remote.json`（URL）和服务器 `sponsors.json`（内容）
2. 重新打包 / 部署官网
3. 客户端 6h 内用缓存；重启或调用 `RefreshSponsorConfig` 强制刷新
