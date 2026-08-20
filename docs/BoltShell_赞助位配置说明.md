# 赞助位远程配置说明

客户端启动时由 Go 后端拉取 `sponsors.json`，支持缓存与本地 fallback。

## 托管地址（官网静态文件）

```
https://boltshell.com/config/sponsors.json
```

开发阶段可使用项目内：

- `config/sponsors.default.json` — Go 后端本地 fallback
- `frontend/public/config/sponsors.json` — Vite 静态资源（官网同源）

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

## 客户端行为

1. 优先拉远程 URL（`config.json` 的 `sponsorConfigURL` 可覆盖）
2. 失败则读本地 `config/sponsors.json` 或内置默认（**fallback 内容，非关闭开关**）
3. **Pro 用户**：验证 `%LOCALAPPDATA%\BoltShell\license.dat`（HMAC 签名）；开发模式 `BOLTSHELL_DEV=1` 可模拟
4. **点 × 关闭**：写入用户目录 `sponsor.state`（带签名）；篡改无效；删除文件则赞助位恢复

## 安全说明（必读）

| 问题 | 现状与对策 |
|------|------------|
| 改 `config.json` 的 `proLicensed` | **正式包无效**；仅 `BOLTSHELL_DEV=1` 开发模式可读 |
| 改 `sponsor-dismiss.json` 永久关广告 | 已改为 **用户目录 + 签名**；手改 `until` 无效；删文件 = 广告回来（7 天关不了永久） |
| 改 `sponsors.default.json` | 只影响离线 fallback 文案，**不能关 Pro、不能永久去广告** |
| 真正去广告 | 购买 Pro → 官网签发 `license.dat` → 闭源校验模块（Pro 上线前接 Ed25519/在线激活） |

> 本地客户端无法 100% 防破解；目标是 **提高 casual 绕过成本**，核心转化靠 Pro 体验与在线 License。

## 修改配置（无后台阶段）

1. 编辑 `frontend/public/config/sponsors.json` 或部署到官网
2. 客户端 6h 内用缓存；重启或调用 `RefreshSponsorConfig` 强制刷新
