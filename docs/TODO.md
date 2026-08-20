# BoltShell 开发 TODO

> 更新时间：2026-08-20

---

## 安全与认证

- [ ] **SSH 密钥登录**：支持 `IdentityFile`、OpenSSH 格式私钥、ssh-agent（v1.5+ 计划）
- [ ] **密码存储加密**：SQLite 中密码字段 AES-256-GCM 加密，主密钥存 OS Keychain（见下方说明）

---

## 已完成

- [x] 删除遗留 Gio GUI（`internal/gui/gioapp.go`）
- [x] `BrowseConnectionDir` 连接池：同一目标服务器 60 秒内复用 SSH 连接（`app_browse_pool.go`）

---

## 商业化（见 [business/BoltShell_商业化与开源策略.md](business/BoltShell_商业化与开源策略.md)）

- [ ] License 模块（free / pro / team）
- [ ] 免费版连接数上限
- [ ] Pro 购买与去广告

---

## 功能

- [ ] 批量命令 Tab（多机执行）
- [ ] 从 Xshell / FinalShell 导入配置
- [ ] `bolt` CLI 命令行

---

## 密码安全方案备忘

| 方案 | 说明 | 推荐阶段 |
|------|------|----------|
| **OS Keychain** | 密码不存 SQLite，存系统凭据库 | Pro / 正式版 |
| **AES-256-GCM + Keychain 主密钥** | DB 加密，密钥由系统保管 | 过渡方案 |
| **明文 SQLite** | 当前实现 | 仅开发/内网 |

Keychain 平台对应：

- Windows → Credential Manager（`wincred` / `github.com/danieljoos/wincred`）
- macOS → Keychain（`github.com/keybase/go-keychain`）
- Linux → Secret Service（`github.com/99designs/keyring`）
