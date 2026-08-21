# BoltShell 开发 TODO

> 更新时间：2026-08-21  
> 优先级权威说明：[business/BoltShell_产品亮点与竞品对比.md](business/BoltShell_产品亮点与竞品对比.md) §五～§七

---

## 安全与认证（P0）

- [ ] **SSH 密钥登录**：IdentityFile、OpenSSH 私钥、ssh-agent（**要做**）
- [ ] **密码存储加密**：SQLite 字段 AES-256-GCM + 主密钥 OS Keychain（继续用 SQLite 即可）
- [ ] HostKey / known_hosts（勿长期 InsecureIgnoreHostKey）

---

## 已完成

- [x] 删除遗留 Gio GUI
- [x] `BrowseConnectionDir` 连接池
- [x] 跨服务器文件传送 + 终端/SFTP/监控一体（**宣传主卖点**）
- [x] 赞助位 UI + 匿名统计埋点骨架
- [x] 编辑连接「密码可见」切换（体验加分，非安全卖点）

---

## 商业化

- [ ] License：free / supporter / pro（**Pro 不去赞助**）
- [ ] 支持者 / Pro 买断收款（忌 ¥1/月）
- [ ] 赞助：Pro 仍可见；可 dismiss
- [ ] 官网：抓包示例 + 隐私 + 统计子域可屏蔽
- [ ] **至少 2～3 个 Pro 真功能落地后再上架 Pro**

---

## 功能

### P0

- [ ] FinalShell 一键导入（现有导入 ≠ 竞品导入）
- [ ] （安全三项见上）

### P1（可作 Pro）

- [ ] Xshell 导入
- [ ] Snippets / 命令片段
- [ ] 会话日志录制
- [ ] 分屏
- [ ] 端口转发 / 隧道
- [ ] 批量命令 Tab
- [ ] `bolt` CLI
- [ ] 「查看我上报的数据」

### P2

- [ ] 云 Sync、Team 审计（有单再做）

---

## 密码安全方案备忘

| 方案 | 说明 | 推荐阶段 |
|------|------|----------|
| **OS Keychain + AES 字段** | 密码密文存 SQLite，主密钥在系统凭据库 | **P0** |
| SQLCipher 整库 | 亦可 | 可选 |
| 明文 SQLite | 当前若仍如此 | 仅开发 |

平台：Windows Credential Manager / macOS Keychain / Linux Secret Service。
