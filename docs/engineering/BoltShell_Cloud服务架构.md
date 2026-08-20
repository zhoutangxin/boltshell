# BoltShell Cloud / 服务端：命名与架构

> 配套：[客户端赞助数据统计设计](../business/BoltShell_客户端赞助数据统计设计.md)  
> 定位：收客户端匿名统计、出赞助报表；以后可扩展 Pro 授权签发（仍建议同仓不同模块）。

---

## 一、怎么取名？

「cloud」只是占位，正式名建议 **好记、好搜、能长大**。

### 1.1 候选对比

| 仓库 / 工程名 | 含义 | 优点 | 缺点 | 推荐度 |
|---------------|------|------|------|--------|
| **`boltshell-server`** | 服务端总称 | 直白；以后加授权/同步也不违和 | 略普通 | ⭐⭐⭐⭐⭐ |
| **`boltshell-hub`** | 中枢 / 控制台 | 有产品感；适合「客户端连回中心」 | 略抽象 | ⭐⭐⭐⭐ |
| **`boltshell-cloud`** | 云端 | 与文档 §7.6 已用词一致 | 「cloud」太泛，像公有云 | ⭐⭐⭐ |
| `boltshell-analytics` | 仅分析 | 职责清晰 | 以后加 License 会名字不对 | ⭐⭐ |
| `boltshell-telemetry` | 遥测 | 技术准 | 对外/商务难懂 | ⭐⭐ |
| `boltshell-console` | 管理台 | 强调后台 | 弱化了「收事件 API」 | ⭐⭐ |
| `bolthub` / `boltgate` | 短品牌 | 酷 | 多一个要保护的名字，早期不值 | ⭐ |

### 1.2 建议定稿

| 用途 | 名称 |
|------|------|
| **Git 仓库** | **`boltshell-server`**（首选）或 `boltshell-hub` |
| **进程 / 二进制** | `boltshell-api`（HTTP 服务） |
| **管理台包名** | `boltshell-admin`（前端工程目录 `admin/`） |
| **对外 URL** | 仍挂主域：`https://boltshell.com/api/...`、`/admin/`（不必单独二级域名） |
| **可选二级域** | `api.boltshell.com`（量起来或要拆证书再开） |

**对外话术：** 「BoltShell 服务端 / 控制台」；对内仓库用 `boltshell-server`。  
文档里若仍写 cloud，等同于本工程，逐步改称 server 即可。

与品牌仓库规划对齐时可写成：

```
boltshell-app/
├── boltshell           # 桌面客户端
├── boltshell-website   # 静态官网（或继续放主仓 website/）
└── boltshell-server    # 统计 API + 管理台（+ 未来授权）
```

---

## 二、语言怎么选？

| 语言 | 适合吗 | 说明 |
|------|--------|------|
| **Go** | ✅ **首选** | 与客户端同语言；单二进制好部署；你们已有 Go 经验；SQLite/PG 生态成熟 |
| Node.js | 可 | 管理台全栈 JS 方便，但多一门运行时 |
| Python | 可 | 数据分析顺手，长期常驻服务不如 Go 省心 |
| Java / .NET | 过重 | 早期统计服务不必 |

**结论：后端用 Go；管理台用 Vue 3 + Vite**（与客户端前端栈接近，降低心智切换）。

---

## 三、整体架构（v1 单体，不上微服务）

```text
                    ┌─────────────────────────────────────┐
  桌面客户端         │           boltshell-server          │
  批量 POST ────────►│  api (Go)                           │
                    │   ├─ /api/v1/analytics/events       │
                    │   ├─ /api/v1/admin/*  (鉴权)         │
                    │   └─ 限流 / AppKey+HMAC 校验         │
                    │              │                        │
                    │              ▼                        │
                    │         SQLite / Postgres             │
                    │         events + daily_stats          │
                    │              │                        │
  浏览器 ───────────►│  admin (Vue) 静态资源由同进程或 Nginx │
                    └─────────────────────────────────────┘
                              ▲
  Nginx: /api → api ; /admin → admin ; /config → 静态 sponsors
```

**原则：**

1. **一个可执行文件** 对外提供 API（管理台可同域静态托管）。  
2. **不要** 拆「采集服务 / 聚合服务 / 网关」——日活到很大再拆。  
3. 与 **官网静态、`sponsors.json`** 解耦：Nginx 反代即可。  
4. 客户端 **永不** 直连数据库。

---

## 四、仓库内目录（推荐）

```
boltshell-server/
├── cmd/
│   └── api/main.go           # 入口
├── internal/
│   ├── config/               # 环境变量 / 配置文件
│   ├── auth/                 # AppKey+HMAC；管理员 JWT/Session
│   ├── analytics/            # 收事件、校验、写入
│   ├── aggregate/            # 按日聚合 DAU/MAU/曝光（可 cron 或写入时增量）
│   ├── adminapi/             # 报表、导出 CSV
│   ├── store/                # SQLite/PG 实现
│   └── httpapi/              # 路由、中间件（限流、日志）
├── admin/                    # Vue3 管理台（独立 package.json）
├── migrations/               # SQL 迁移
├── deploy/
│   ├── docker-compose.yml
│   └── nginx.example.conf
├── go.mod
└── README.md
```

---

## 五、核心模块职责

| 模块 | 做什么 |
|------|--------|
| **analytics** | 校验事件白名单、字段；写入 `events`；拒绝含 host/命令等脏数据 |
| **aggregate** | `installId` 去重算日 DAU/MAU；按 `slotId` 汇总 impression/click/dismiss |
| **auth（双轨）** | 客户端：AppKey+HMAC；管理员：账号密码或 Token（与客户端无关） |
| **adminapi** | summary、分日曲线、CSV 导出 |
| **store** | 仓储接口；v1 默认 **SQLite**，配置可切 Postgres |

### 5.1 表（最小）

```text
events(
  id, received_at, event_ts, install_id, app_version, os,
  is_pro, event, slot_id, config_version, props_json
)

daily_install_active(day, install_id, is_pro)     -- 可选物化，加速 DAU
daily_slot_stats(day, slot_id, impressions, clicks, dismisses)
```

也可用「只存 events，报表 SQL 现算」；量小更简单，量升再物化。

---

## 六、接口一览（v1）

| 方法 | 路径 | 调用方 | 认证 |
|------|------|--------|------|
| `POST` | `/api/v1/analytics/events` | 桌面客户端 | AppKey + 签名 + 限流 |
| `GET` | `/api/v1/admin/summary` | 管理台 | 管理员登录 |
| `GET` | `/api/v1/admin/slots` | 管理台 | 管理员登录 |
| `GET` | `/api/v1/admin/export.csv` | 管理台 | 管理员登录 |
| `POST` | `/api/v1/admin/login` | 管理台 | 账号密码 |

健康检查：`GET /healthz`（无鉴权，供探活）。

---

## 七、技术选型速查

| 项 | 选型 |
|----|------|
| 语言 | **Go 1.21+** |
| HTTP | 标准库 `net/http` 或 chi / echo（任选一，保持简单） |
| DB | **SQLite**（单机起步）→ 以后 Postgres |
| 迁移 | goose / golang-migrate |
| 管理台 | **Vue 3 + Vite + TypeScript** |
| 部署 | 单二进制 + Nginx；或 docker-compose（api + 可选 volume） |
| 日志 | slog；请求 ID |

**明确不做（v1）：** Kafka、Redis 集群、微服务、K8s、用户 OAuth、实时 WebSocket 大屏。

---

## 八、与客户端、官网的边界

```text
boltshell (Wails)
  └─ 只负责：installId、队列、批量 POST、赞助 UI

boltshell-server (本工程)
  └─ 只负责：收数、存数、算数、给你们看数
  └─ 以后可选：签发 license.dat（仍建议放本仓 module，勿另起炉灶太早）

website /
  └─ 静态页 + sponsors.json + release.json
```

配置：客户端 `sponsors.remote.json` 增加 `analyticsURL`，指向  
`https://boltshell.com/api/v1/analytics/events`。

---

## 九、实施顺序

1. 定名建仓：`boltshell-server`  
2. Go：`POST /events` + SQLite + AppKey  
3. 客户端 P0+P1 对接（**同包发版**）  
4. 管理台 summary + CSV  
5. 跑数数周 → 校准赞助刊例  

---

*文档结束。命名推荐：`boltshell-server`；语言：Go + Vue3；架构：单体 API，不上微服务。*
