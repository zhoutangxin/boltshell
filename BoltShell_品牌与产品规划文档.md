# BoltShell 品牌与产品规划文档

> 生成时间：2026-08-20
> 产品定位：一体化 SSH 运维终端（对标 Xshell / FinalShell）

---

## 一、产品定位

### 1.1 产品形态
- **类型**：一体化 SSH 客户端（非纯终端模拟器）
- **核心功能**：
  - SSH 会话管理（左侧树形结构）
  - 内置终端（连接后执行 `ll`、`ls`、`pwd` 等命令）
  - SFTP 文件管理（底部/侧边文件浏览器）
  - 服务器监控（CPU、内存、网络实时图表）
- **对标产品**：Xshell、FinalShell、MobaXterm

### 1.2 目标用户
- 国内运维开发者、后端工程师
- 需要管理多台远程服务器的个人/团队
- 习惯 GUI 工具而非纯命令行的用户

### 1.3 核心卖点
- **方便简单**：双击即连，无需记忆命令
- **一体化**：终端 + 文件 + 监控，一个窗口搞定
- **闪电连接**：保存的会话秒级连接

---

## 二、命名决策

### 2.1 命名历程
| 候选名 | 状态 | 放弃原因 |
|--------|------|---------|
| `hopshell` | 域名/商标可用 | GitHub Organization 被抢，且 "hop" 偏轻量，不适合重型运维工具 |
| `shellhop` | 域名可用 | 拗口，动作感弱 |
| `liteshell` | 域名可用 | "lite" 暗示功能不全，与产品定位矛盾 |
| `hopterm` | 域名 85 元 | 偏终端模拟器，不适合一体化 SSH 工具 |
| `gateshell` | .com 被抢 | — |
| `bridgeshell` | .com 被抢 | — |
| `suoke` (梭壳) | .com 被抢，.cn 天价 | — |

### 2.2 最终选择：BoltShell

**品牌名**：BoltShell  
**域名**：`boltshell.com`（85 元）+ `boltshell.cn`（顺手注册）  
**GitHub**：`github.com/boltshell-app/boltshell`  
**命令**：`bolt`  
**Slogan**：*Bolt into your server.* / 闪电连接，一键运维。

**命名逻辑**：
- `Bolt` = 闪电，暗示"秒连服务器"的速度感
- `Shell` = 直接对标 Xshell、FinalShell 的命名脉络，用户一看就知道是"连接服务器的工具"
- 命令 `bolt` 仅 4 个字母，输入成本极低
- 与竞品（Xshell、FinalShell、WindTerm）站在同一条命名战线上

**关于 "Bolt = 螺栓" 的顾虑**：
- 在科技圈，`bolt` 的"速度/闪电"意象远强于"螺栓"
- Stripe 有支付产品叫 Bolt，Usain Bolt 是闪电博尔特
- 通过 Logo（⚡ 闪电符号）和 Slogan 强化"速度"意象即可

---

## 三、品牌资产清单

### 3.1 域名
| 域名 | 状态 | 费用 | 用途 |
|------|------|------|------|
| `boltshell.com` | ✅ 待购买 | 85 元 | 国际主站、官网 |
| `boltshell.cn` | ✅ 待购买 | ~35 元 | 国内备案、跳转 |

### 3.2 GitHub
| 资产 | 地址 | 状态 |
|------|------|------|
| Organization | `github.com/boltshell-app` | ✅ 已注册 |
| 核心仓库 | `github.com/boltshell-app/boltshell` | ⏳ 待创建 |
| 个人保底仓库 | `github.com/zhoutangxin/boltshell` | ⏳ 待创建 |

**仓库规划**：
```
boltshell-app/
├── boltshell          # 核心产品（GUI 客户端源码）
├── boltshell-cli      # 命令行版本（bolt 命令的实现）
├── boltshell-docs     # 官方文档网站
├── boltshell-website  # 官网 boltshell.com 源码
└── .github            # 组织级配置（Issue 模板、Workflow）
```

### 3.3 商标
| 类别 | 内容 | 重要性 | 状态 |
|------|------|--------|------|
| 第9类 | 科学仪器 / 计算机软件 | ⭐⭐⭐⭐⭐ | ⏳ 待申请 |
| 第42类 | 技术服务 / 软件开发 | ⭐⭐⭐⭐⭐ | ⏳ 待申请 |
| 第35类 | 广告销售 / 商业管理 | ⭐⭐⭐ | 可延后 |

**商标查询结果**：
- `boltshell` 精确匹配：第9类无已注册商标 ✅
- 近似商标（BODYSHELL、BEST SHELL）：类别不同（医药/日化），无冲突 ✅
- 风险：济南国货之光商贸有限公司近期大量注册 `xxxSHELL` 组合商标，需尽快提交申请

**申请建议**：
- 通过阿里云商标服务或代理机构提交
- 官费 270 元/类，代理费约 300-800 元/类
- 总耗时 8-12 个月

### 3.4 社交媒体占位（待执行）
- Twitter/X：@boltshell
- 知乎：boltshell
- 微信公众号：boltshell

---

## 四、命令行体验设计

### 4.1 命令结构
```bash
# 启动 GUI
$ boltshell

# 快速连接（最高频）
$ bolt prod-db
$ bolt 192.168.1.100

# 会话管理
$ bolt list              # 列出所有保存的会话
$ bolt ls                # 别名
$ bolt add <name>        # 添加新会话（交互式）
$ bolt edit <name>       # 编辑配置
$ bolt remove <name>     # 删除会话

# 快捷操作
$ bolt sftp <name>       # 直接打开 SFTP 面板
$ bolt --copy <name>     # 复制 session 配置

# 配置迁移
$ bolt export > backup.json
$ bolt import backup.json
```

### 4.2 Shell 补全（安装时注入）
用户运行一次，永久生效：
```bash
$ bolt init bash    # 或 zsh / fish
```

补全效果：
```bash
$ bolt <TAB>
add     edit    export  import  list    remove  sftp    prod-db  staging  jump-box
```

### 4.3 安装方式
```bash
# 一行安装（官网脚本）
curl -fsSL https://boltshell.com/install.sh | sh

# Homebrew
brew install boltshell

# Scoop（Windows）
scoop install boltshell
```

---

## 五、Shell 集成方案

### 5.1 作为命令行工具
安装后放到 PATH，支持所有常见操作：
```bash
$ bolt connect prod-server
$ bolt list
$ bolt add ali-cloud --host 1.2.3.4 --key ~/.ssh/id_rsa
```

### 5.2 注册 Shell 别名/函数
用户可加到 `~/.bashrc` 或 `~/.zshrc`：
```bash
alias s='bolt'           # 超短别名
alias ss='bolt connect'  # 更短

# 或函数，支持补全
s() { bolt connect "$@"; }
```

### 5.3 Shell 自动补全脚本

**Bash** (`/etc/bash_completion.d/bolt`):
```bash
_bolt_completions() {
    local cur="${COMP_WORDS[COMP_CWORD]}"
    COMPREPLY=($(compgen -W "connect list add edit remove sftp" -- "$cur"))
}
complete -F _bolt_completions bolt
```

**Zsh** (`_bolt`):
```zsh
#compdef bolt
_bolt() {
    _arguments '1: :->command' '2: :->host'
    case "$state" in
        command) _values 'command' connect list add edit remove sftp ;;
        host) _values 'host' $(bolt list --names) ;;
    esac
}
compdef _bolt bolt
```

### 5.4 集成 SSH Config
工具可以读取/写入 `~/.ssh/config`，让用户原有 `ssh xxx` 也能走 BoltShell：
```ssh-config
Host myserver
    HostName 1.2.3.4
    User root
    IdentityFile ~/.ssh/id_rsa
```

或者反向——**BoltShell 生成 ssh config**：
```bash
$ bolt export --format ssh-config >> ~/.ssh/config
```

---

## 六、竞品分析

### 6.1 纯终端模拟器（不是你的赛道）
| 产品 | 特点 | 命名逻辑 |
|------|------|---------|
| Warp | AI 驱动、块级 UI | 极简意象（曲速） |
| iTerm2 | macOS 老大哥 | 版本号命名 |
| Alacritty | GPU 加速、极简 | 拉丁语"敏捷" |
| Ghostty | Mitchell Hashimoto 新作 | 意象词 |
| Kitty | GPU 终端、图片内联 | 动物名 |

**规律**：纯终端走极简/意象路线，不提 SSH/Shell。

### 6.2 一体化 SSH 工具（你的赛道）
| 产品 | 特点 | 命名逻辑 |
|------|------|---------|
| Xshell | 老牌、稳定 | X + Shell |
| FinalShell | 国产、一体化 | 形容词 + Shell |
| MobaXterm | 全能工具箱 | 复合词 + Xterm |
| Termius | 跨平台、现代 | Term 词根 |
| WindTerm | 轻量、开源 | 自然意象 + Term |

**规律**：一体化工具必须让用户一眼知道"这是连服务器的"，所以带 Shell/Term/Secure。

### 6.3 BoltShell 的差异化
- 比 Xshell 更现代、更轻量
- 比 FinalShell 更简洁、不臃肿
- 比 Termius 更懂国内用户习惯
- 命令行体验 `bolt` 比所有竞品都短

---

## 六点五、赞助位 UI 规划（v1）

> 详细商业化策略见 [BoltShell_商业化与开源策略.md](docs/BoltShell_商业化与开源策略.md)  
> 远程配置规范见 [BoltShell_赞助位配置说明.md](docs/BoltShell_赞助位配置说明.md)

### 6.5.1 按页面划分（两个界面）

BoltShell 客户端只有 **两种界面态**，广告位 **互斥显示**（不会同时出现 3 条）：

| 页面 | 何时出现 | 广告位数量 | slotId | 说明 |
|------|----------|------------|--------|------|
| **A. 快速连接页** | 未连接任何服务器 | **1 个** | `quick_connect_bottom` | 主工作区底部居中窄条（52px） |
| **B. 系统信息侧栏** | 已连接服务器后 | **2 个** | `sidebar_1`、`sidebar_2` | 240px 侧栏 **最底部** 各 1 条 compact（64px） |

**其他页面不放广告：**

| 页面 | 广告位 |
|------|--------|
| 终端主区域（黑底） | ❌ 0 |
| SFTP 文件面板 | ❌ 0 |
| 连接管理器弹窗 | ❌ 0 |
| 传输队列面板 | ❌ 0 |
| 官网 `website/` | ❌ 0（v1 仅 Footer 文字链，不算客户端广告位） |

**关于中间大块空白（两页都会出现）：**

- **快速连接页**：空白留给连接列表增长 → **不要在中间加广告**
- **系统信息侧栏**：空白在监控数据下方、赞助位 **固定在侧栏最底** → **不要在 CPU/内存 和 赞助位 之间插入广告**

### 6.5.2 位置数量：全局固定 3 个 slotId

| slotId | 场景 | 数量 | 用途 |
|--------|------|------|------|
| `quick_connect_bottom` | **未连接**（快速连接页） | 1 | 品牌 / 开源 / 大赞助商 |
| `sidebar_1` | **已连接**（左侧系统信息下方） | 1 | 外部赞助（云厂商等） |
| `sidebar_2` | **已连接** | 1 | 自营 Pro 转化（去广告） |

**禁止新增的位置：** 终端区域内、SFTP 文件列表内、弹窗 / 开屏、连接管理器内。

**v2 可扩展：** `settings_footer`（设置页底部 1 条文字链）。同时在线不超过 **3～4 个**。

### 6.5.3 尺寸规范

| 位置 | 高度 | 宽度 | 文案 |
|------|------|------|------|
| `quick_connect_bottom` | **48～52px** | **max 560px 居中窄条** | 标题 + 描述 **单行** 横排 |
| `sidebar_1` / `sidebar_2` | **60～68px** | 侧栏满宽（240px） | 角标 + 标题 + 描述各 1 行 |

### 6.5.4 布局示意

**未连接（快速连接页）— 居中窄条，非通栏大卡片：**

```
┌─────────────────────────────────────┐
│  快速连接列表（flex:1，可滚动）        │
│                                     │
│     ┌──────────────────────┐        │
│     │ ⚡ 标题 · 链接 →      │  52px │
│     └──────────────────────┘        │
└─────────────────────────────────────┘
        ↑ max-width: 560px 居中
```

**已连接（左侧系统信息栏）— 底部 2 条 compact：**

```
┌──────────┐
│ 系统信息  │
│ CPU/内存  │
│ 进程/磁盘 │
├──────────┤
│ [开源]64px│  ← sidebar_1
│ [Pro] 64px│  ← sidebar_2
└──────────┘
```

### 6.5.5 行为规则

- **Pro 用户：** 所有赞助位隐藏；正式版仅认 `%LOCALAPPDATA%\BoltShell\license.dat`（购买激活下发）
- **开发调试：** 设置 `BOLTSHELL_DEV=1` 后可使用 `BOLTSHELL_PRO=1` 或 `config.json` 的 `proLicensed`（正式包忽略该字段）
- **免费用户点 ×：** 该 slot 隐藏 **7 天**；状态写入用户目录 `sponsor.state`（HMAC 签名，手改无效；删文件则恢复展示）
- **连接成功后：** 快速连接页赞助位自动消失，仅显示侧栏 2 条
- **默认 sponsors.json：** 远程不可用时的 fallback，不是「关闭开关」
- **配置下发：** `config/sponsors.remote.json`（开发者配置 remoteURL，支持 IP）；6h 客户端缓存

### 6.5.6 配置与官网

| 资产 | 路径 |
|------|------|
| 客户端远程 URL | `config/sponsors.remote.json`（开发者配置，随包分发） |
| 客户端本地 fallback | `config/sponsors.default.json` |
| 官网静态配置 | `website/config/sponsors.json` |
| 官网单页骨架 | `website/index.html` |
| Nginx 部署说明 | `website/README.md` |
| **配置文件读取路径（技术细节）** | [BoltShell_赞助位配置说明.md](docs/BoltShell_赞助位配置说明.md) |

无后台阶段：编辑 `config/sponsors.remote.json` + 服务器 `sponsors.json` → 打包 / 部署 → 客户端拉取生效。

---

## 七、下一步行动清单

### 🔴 本周内（紧急）
- [ ] 购买 `boltshell.com`（85 元）
- [ ] 购买 `boltshell.cn`（~35 元）
- [ ] 创建 GitHub 仓库 `github.com/boltshell-app/boltshell`
- [ ] 创建个人保底仓库 `github.com/zhoutangxin/boltshell`
- [ ] 写 README 占位（见下方模板）
- [ ] 设置仓库 Topics：`ssh`, `terminal`, `sftp`, `devops`

### 🟡 本月内
- [ ] 提交第9类 + 第42类商标申请
- [ ] 注册 Twitter/X @boltshell
- [ ] 注册知乎 boltshell
- [ ] 注册微信公众号 boltshell
- [ ] 设计 Logo（⚡ 闪电 + 终端符号）

### 🟢 开发阶段
- [ ] 实现 `bolt` 命令行核心（连接、list、add）
- [ ] 实现 GUI 客户端原型
- [ ] 实现 Shell 补全脚本（Bash/Zsh/Fish）
- [ ] 编写安装脚本 `install.sh`
- [ ] 发布 v0.1.0

---

## 八、README 占位模板

```markdown
# BoltShell ⚡

> Bolt into your server.  
> 闪电连接，一键运维。

BoltShell 是一款面向开发者的 SSH 客户端工具，支持会话管理、SFTP 文件传输和系统监控。

## 特性

- 🚀 秒级连接远程服务器
- 📁 内置 SFTP 文件管理
- 📊 实时系统监控面板
- 🔐 安全的密钥管理
- ⚡ 命令行快速连接：`bolt <session-name>`

## 安装

```bash
# 即将推出
curl -fsSL https://boltshell.com/install.sh | sh
```

## 命令行用法

```bash
$ bolt list              # 列出所有会话
$ bolt add prod-db       # 添加新会话
$ bolt prod-db           # 快速连接
$ bolt sftp prod-db      # 打开 SFTP 面板
```

## 状态

🚧 开发中，敬请期待。

## 官网

[boltshell.com](https://boltshell.com)

## 许可证

MIT
```

---

## 九、命名铁律（经验总结）

1. **域名是租来的门牌号，商标是法律发的房产证** —— 没有商标，你只是在帮别人养品牌
2. **产品做大了，名字自然好听；产品做死了，再好的名字也是垃圾**
3. **不要追求完美名字，追求"足够好 + 现在就能用"**
4. **.com + 短命令 > 完美语义 + 长域名**
5. **中文市场用中文名传播力是英文名的 10 倍** —— 但 boltshell 可以配中文宣传名"闪壳"

---

*文档结束。现在去买 boltshell.com。*
