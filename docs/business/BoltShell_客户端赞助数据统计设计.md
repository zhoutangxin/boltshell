# BoltShell 客户端赞助数据：设计与统计方案

> 配套文档：[广告术语与指标说明](BoltShell_广告术语与指标说明.md) · [赞助位配置说明](BoltShell_赞助位配置说明.md) · [开源许可证详解与选型](BoltShell_开源许可证详解与选型.md) · [竞品变现与广告风险调研](BoltShell_竞品变现与广告风险调研.md) · [个人开发者商业化 FAQ](BoltShell_个人开发者商业化FAQ.md)  
> 目标：在 SSH 客户端里把 DAU / MAU / 曝光 / 点击等指标 **可采集、可汇总、可报给甲方**，且不碰敏感运维数据。

---

## 一、结论先看

| 问题 | 答案 |
|------|------|
| 客户端有没有 PV/UV？ | **不报 PV/UV**；用 **启动次数 / DAU / MAU** |
| 曝光怎么计？ | slot **首次渲染可见**记 1 次；同一次「界面态」内去重 |
| 数据往哪报？ | **自建轻量上报**（你们服务器），**禁止**穿山甲等广告 SDK |
| 能不能记主机/IP？ | **绝对禁止**（SSH 信任红线） |
| **何时必须上埋点？** | **广泛发版 / 带赞助位的版本之前就必须带上**（见 §1.1） |
| 没统计服务端时？ | 客户端仍写入本地队列；服务端可后补，但客户端埋点不能等 |

**一句话：** 前端触发埋点 → Go 本地队列 → 定时批量 POST 到你们统计接口 → 按天聚合出「免费 MAU / 分 slot 曝光 / CTR」。

### 1.1 硬约束：发版前就要把统计做进客户端

SSH / 桌面客户端和网站不同：

| | 网站 / 小程序 | BoltShell 桌面客户端 |
|--|---------------|----------------------|
| 发新统计逻辑 | 用户下次打开页面就生效 | **只有升级到新版本才生效** |
| 不升级的用户 | 几乎不存在「旧前端」 | 会长期停在旧包，**永远不上报新事件** |

因此：

1. **带赞助位对外分发的版本，必须同时带上最小埋点（P0+P1）**，不能「先发赞助 UI，以后再加统计」。  
2. 后期再加埋点，只能覆盖 **愿意升级** 的那批人；停在旧版的用户对 DAU/曝光是黑洞，报表会系统性偏小。  
3. 「有埋点再涨价」指的是：埋点已随包发出去并 **积累了一段真实数据之后**，再上调 CPT 刊例——**不是**「发版以后再补埋点」。  
4. 服务端接收接口可以简陋（甚至先落盘日志），但 **客户端发事件的代码必须进包**。  
5. 应用内更新（若已有）能提高覆盖率，但仍有人关更新；所以 **首发版本就要带齐**，不能赌全员升级。

**发版门禁（Checklist）：**

- [ ] `installId` 持久化  
- [ ] `app_launch`  
- [ ] `sponsor_impression` / `sponsor_click` / `sponsor_dismiss`  
- [ ] 本地队列（断网可攒）  
- [ ] `analyticsURL` 可配置（或写死正式统计地址）  
- [ ] 隐私开关 + 说明（可默认开）  
- [ ] 抓包确认 **无** SSH 主机/命令  

未勾完 **不要** 把该包当作「可谈赞助的正式渠道包」大规模推。

---

## 二、术语在客户端怎么落地


| 术语文档里的词       | 客户端实际事件 / 算法                                                   | 埋点名建议                                            |
| ------------- | -------------------------------------------------------------- | ------------------------------------------------ |
| （勿叫 PV）       | 进程启动一次                                                         | `app_launch`                                     |
| DAU           | 某自然日至少 1 次 `app_launch`（或更严：`ssh_connected`）的 **去重 installId** | 服务端按日 `COUNT DISTINCT installId`                 |
| MAU           | 自然月内至少活跃 1 天的去重 installId                                      | 同上按月                                             |
| 可售触达          | 免费用户的 DAU/MAU（`isPro=false`）                                   | 上报带 `isPro`                                      |
| slot          | 配置里的 slotId                                                    | 事件字段 `slotId`                                    |
| 曝光 Impression | 某 slot 被展示且计入规则满足                                              | `sponsor_impression`                             |
| 点击 Click      | 点击赞助条打开外链                                                      | `sponsor_click`                                  |
| 关闭 Dismiss    | 点 ×                                                            | `sponsor_dismiss`（已有 `DismissSponsorSlot`，补事件即可） |
| CTR           | `clicks / impressions`（按 slot、按日）                              | 服务端算，客户端不算                                       |


---



## 三、隐私与红线（必须写进实现）

SSH 工具信任成本高，统计只允许「产品使用元数据」，不允许「运维内容」。


| ✅ 允许采集                              | ❌ 禁止采集               |
| ----------------------------------- | -------------------- |
| 匿名 `installId`（本地生成 UUID）           | SSH 主机名 / IP / 端口    |
| 应用版本、OS（windows/mac/linux）          | 用户名、密码、密钥路径          |
| `slotId`、素材 `title` 哈希或配置 `version` | 终端命令、输出内容            |
| `isPro`（是否已去广告）                     | 会话列表、文件路径            |
| 事件时间（客户端本地 ISO 时间）                  | 精确到秒的操作轨迹连环追踪（可只留日级） |


**合规建议：**

- 设置页增加「帮助改进产品（匿名统计）」开关，默认 **开**（或默认关，按你们口碑策略二选一；谈赞助需要数据时建议默认开 + 可关）。
- 隐私说明写清：不上报服务器连接信息；仅统计启动与赞助位互动。
- Pro 用户：可不报赞助事件（本来不展示）；`app_launch` 仍可报以便算全体 MAU，对外报赞助时再筛 `isPro=false`。

### 3.1 「静默上报」是什么意思？为什么要避免

| 坏做法 | 含义 |
|--------|------|
| **静默上报** | 后台一直 POST，设置找不到、首次启动也不提示 |
| **没有开关** | 用户无法关闭 |
| **隐私政策找不到** | 官网/关于页没有「我们传什么」 |

这三样叠加 = 「偷偷监视」。**有 HTTPS 请求本身不是原罪**（Navicat、JetBrains、FinalShell 云同步也会连自家服），伤信任的是**没说清 + 不可验证**。

商业软件常连自家服务器做更新 / License；赞助统计也属于「接口通讯」，但必须讲清：v1 **只**传启动与赞助元数据，**不**用上报抓未付费（抓未付费若要做，应走离线 License）。详见 [个人开发者商业化 FAQ](BoltShell_个人开发者商业化FAQ.md)。

### 3.2 代码位置（便于官网写「可审计」）

| 路径 | 作用 |
|------|------|
| `server/app_analytics.go` | `TrackSponsorEvent` / 开关 / Flush / InstallID |
| `server/internal/analytics/` | 本地队列、批量 POST |
| `web/src/components/sponsor/SponsorBanner.vue` | 曝光 / 点击触发 |
| 本机 `%LOCALAPPDATA%\BoltShell\` | `analytics.queue`、`install.id`、`analytics.prefs.json` |

### 3.3 官网应提供「抓包示例」页

建议独立页（如 `/privacy` 或 `/telemetry`）：

1. 字段白名单 + **真实 JSON 样例**（见 §5.2 / §5.3）  
2. 永不上传清单（主机 / IP / 命令 / 密码…）  
3. Wireshark / mitmproxy 指向统计子域的步骤  
4. 设置关闭 + hosts 屏蔽方法（§3.4）  
5. 「查看我上报的数据」→ 打开本地 `analytics.queue`

### 3.4 独立子域 + 主动告知屏蔽（会少样本，但更划算）

推荐统计走独立子域，例如 `https://t.boltshell.com/api/v1/analytics/events`（与官网静态、赞助 JSON 分离）。

关于页写明：

> 可在设置中关闭匿名统计，或在 hosts 屏蔽 `t.boltshell.com`。  
> **关闭后 SSH / SFTP / 监控 / 跨机传送不受影响。**

**会不会因此少收益？** 会少一部分上报率（敏感用户关掉），但：

- 关掉的人往往是最爱发帖的；硬采 → 一张「偷偷上报」截图顶掉整月推广；  
- 早期赞助用 **CPT 包月 + UTM 自核**，不依赖 100% 埋点保量；  
- 公式：`赞助收入 ≈ 真实活跃 × 信任 × 续约`，不是 `强制上报率`。

---

## 四、身份：用什么做「一个人」

桌面端没有 Cookie，用 **安装级匿名 ID**：


| 字段           | 说明                                                                                    |
| ------------ | ------------------------------------------------------------------------------------- |
| `installId`  | 首次启动写入 `%LOCALAPPDATA%\BoltShell\install.id`（macOS/Linux 对应 App Support），UUID v4，重装会变 |
| `isPro`      | 是否存在有效 `license.dat`                                                                  |
| `appVersion` | 如 `1.0.2`                                                                             |
| `os`         | `windows` / `darwin` / `linux`                                                        |


**不要用：** 机器名、网卡 MAC、硬盘序列号（隐私与跨平台坑）。  
**可选升级：** 以后账号体系上线后，可用 `userId` 做第二口径，但赞助报数仍建议以 installId 为主（简单、与付费无关）。

---



## 五、事件设计（最小集）



### 5.1 事件清单（v1 只做这些）


| event                | 何时触发                      | 用途               |
| -------------------- | ------------------------- | ---------------- |
| `app_launch`         | Go `startup` 成功后发 1 次     | DAU / MAU / 启动次数 |
| `ssh_connected`      | 首次 SSH 会话建立成功（可选）         | 更严的「有效活跃」        |
| `sponsor_impression` | 某 slot 满足曝光规则             | 分位曝光             |
| `sponsor_click`      | `SponsorBanner` 打开链接前     | 点击 / CTR         |
| `sponsor_dismiss`    | 调用 `DismissSponsorSlot` 时 | 关闭率              |




### 5.2 公共字段（每条事件都带）

```json
{
  "schema": 1,
  "event": "sponsor_impression",
  "ts": "2026-08-20T20:45:00+08:00",
  "installId": "a1b2c3d4-...",
  "appVersion": "1.0.2",
  "os": "windows",
  "isPro": false,
  "props": {}
}
```



### 5.3 各事件 props

`sponsor_impression` **/** `sponsor_click` **/** `sponsor_dismiss`**：**

```json
{
  "slotId": "sidebar_1",
  "configVersion": 1,
  "linkHost": "example.com"
}
```

- `linkHost`：只取 URL 的 host，**不要**带完整 path/query（减少敏感与体积）。
- 不要上传 `title`/`desc` 全文也可；用 `configVersion` + `slotId` 即可对齐当时素材。

`app_launch`**：** `props` 可空，或带 `{ "ui": "gui" }`。

---



## 六、曝光计数规则（最容易算错）

目标：接近「用户看见了几次」，又避免刷爆数字。

### 6.1 推荐规则（v1）

对每个 `(installId, slotId, 本地自然日)`：

1. **同一 UI 态只计 1 次**
  - 快速连接页：slot 挂载成功 → 记 1 次 `quick_connect_bottom`  
  - 侧栏：`sidebar_1` / `sidebar_2` 各自在进入「已连接且面板可见」时各记 1 次
2. **同一天再次回到该态**（断线回快速连接 / 再连上）→ **可以再记**（真实二次看见）
3. **窗口最小化 / 切到后台再回来**：v1 **不计** 新曝光（实现简单）
4. **Pro / 该 slot 已 dismiss**：不展示 → **不发** impression
5. **拉取** `sponsors.json`：只算配置，**绝不**当曝光



### 6.2 实现落点（与现有代码对齐）


| 位置                              | 做什么                                               |
| ------------------------------- | ------------------------------------------------- |
| `SponsorBanner.vue` `onMounted` | 调 `TrackSponsorEvent('impression', slotId)`（经 Go） |
| `SponsorBanner.vue` `openLink`  | 先 track `click`，再 `BrowserOpenURL`                |
| `useSponsors.dismiss`           | track `dismiss`                                   |
| `App` / Go `startup`            | track `app_launch`                                |
| 连接成功回调（已有 session 逻辑）           | 可选 `ssh_connected`                                |


前端不直连统计服务器；统一走 Go API，便于队列、签名、开关。

### 6.3 防抖伪代码

```text
key = slotId + "|" + localDate
if alreadySent[key] for currentSurfaceSession:
  skip
else:
  emit impression; mark key for this surfaceSession
```

`surfaceSession`：例如「进入快速连接页」或「某次连接会话的侧栏生命周期」；断线重连生成新 session。

---



## 七、上报架构

```
┌─────────────┐     TrackEvent      ┌──────────────────┐
│ Vue 赞助位   │ ─────────────────► │ Go analytics     │
│ / startup    │                    │ · 写本地队列     │
└─────────────┘                    │ · 开关 / 采样    │
                                   └────────┬─────────┘
                                            │ 批量 POST（如每 5min / 退出时）
                                            ▼
                                   ┌──────────────────┐
                                   │ 你们的统计 API   │
                                   │ POST /v1/events  │
                                   └────────┬─────────┘
                                            ▼
                                   ┌──────────────────┐
                                   │ 日表聚合         │
                                   │ DAU/MAU/曝光…    │
                                   └──────────────────┘
```



### 7.1 本地路径（建议）


| 文件    | 路径                                         | 内容               |
| ----- | ------------------------------------------ | ---------------- |
| 安装 ID | `%LOCALAPPDATA%\BoltShell\install.id`      | UUID 文本          |
| 事件队列  | `%LOCALAPPDATA%\BoltShell\analytics.queue` | NDJSON 或 JSON 数组 |
| 用户开关  | 用户 `config.json` 的 `analyticsEnabled`      | bool             |




### 7.2 上报协议（示例）

`POST https://boltshell.com/api/v1/analytics/events`

```json
{
  "batchId": "…",
  "events": [ { "...": "单条事件" }, … ]
}
```

- 单批建议 ≤ 100 条；失败指数退避，队列有上限（如 2000 条，超出丢最旧）。
- **无后台阶段**：可先只写本地队列 + 开发者用调试命令导出；或复用现有服务器加一个极简接收脚本落盘。



### 7.3 与赞助配置同源

统计上报 URL 可写在 `sponsors.remote.json` 旁，例如：

```json
{
  "remoteURL": "https://boltshell.com/config/sponsors.json",
  "analyticsURL": "https://boltshell.com/api/v1/analytics/events"
}
```

未配置 `analyticsURL` → 只本地记或不记，不影响主功能。

### 7.4 会不会频繁调用后端？——不会（按设计就不该）

**埋点种类多 ≠ HTTP 请求多。** 事件先写本地队列，再 **批量** 上报。

| 层 | 频率 | 说明 |
|----|------|------|
| 前端 → Go `TrackEvent` | 可多次 | 进程内函数调用，**不是**网络请求 |
| Go → 本地队列文件 | 同左 | 追加写盘，极轻 |
| Go → 统计服务器 | **很低** | 默认例如：**每 5～15 分钟一批**，或 **队列 ≥ N 条**，或 **应用退出时冲刷** |

**单用户一天量级（正常使用）：**

| 事件 | 大约次数/天 |
|------|-------------|
| `app_launch` | 1～几次 |
| `sponsor_impression` | 去重后往往个位数～十几 |
| `sponsor_click` / `dismiss` | 0～几次 |
| **合计事件** | 通常 **远小于 50 条/天** |
| **HTTP 次数** | 理想情况 **每天几次批量 POST**，不是每条事件打一次 |

**禁止的错误实现：**

- ❌ 每点一次赞助就 `fetch` 一次服务器  
- ❌ 每次渲染 slot 就立刻 HTTP  
- ❌ 前端直连统计域名（应走 Go 队列）

**还可再降频：**

- 曝光已有「同界面态去重」（见 §6）  
- 可选：仅免费用户上报赞助事件；Pro 只报 `app_launch` 或完全不报赞助类  
- 失败退避：网络错误则拉长间隔，避免狂重试打爆接口  

拉取 `sponsors.json`（配置）与统计上报是 **两件事**：配置仍可 6h 缓存；统计按上面批量策略，互不绑架。

### 7.5 统计接口要不要认证体系？

分两种「认证」，不要混：

| 类型 | 要不要 | 说明 |
|------|--------|------|
| **用户登录体系**（账号密码 / OAuth） | **不要** | 匿名 `installId` 即可算 DAU；强迫登录会劝退，也与「不上报运维隐私」叙事冲突 |
| **接口防刷 / 防滥用** | **要（轻量）** | 公网 POST 若不加任何门槛，会被扫到后灌垃圾数据，污染 MAU |

**推荐 v1（够用、好实现）：应用级密钥 + 可选签名，不是用户体系。**

```http
POST /api/v1/analytics/events
Content-Type: application/json
X-BoltShell-App-Key: <写在客户端包内的公开应用标识>
X-BoltShell-Ts: <unix 秒>
X-BoltShell-Sign: <HMAC-SHA256(appSecret, ts + "\n" + bodyHash)>
```

| 手段 | 作用 | 注意 |
|------|------|------|
| `appKey` | 识别来自官方客户端渠道 | 可放包内；防的是「随便的脚本乱打」里最弱的一层 |
| `HMAC` 签名 + 时间窗（如 ±5 分钟） | 提高伪造成本 | `appSecret` 会进客户端，**只能防君子与脚本小子，防不了逆向**；对匿名统计通常够用 |
| 限流 | 单 IP / 单 installId 单位时间上限 | 服务端必做，比完美签名更重要 |
| HTTPS | 防中间人篡改明文 | 正式环境必须 |
| 内容校验 | `event` 白名单、字段长度、丢弃含 host/IP 的脏包 | 保护口径与隐私 |

**明确不做（v1）：**

- ❌ 为报个曝光让用户注册登录  
- ❌ 把 SSH 会话 token 拿来当统计认证  
- ❌ 重型 OAuth / JWT 用户体系（除非以后做账号同步再复用）

**管理后台查报表**（你们自己看数）可以另做：Basic Auth / 内网 / VPN，与客户端上报接口分离。

**安全预期要诚实：** 桌面端任何写死的 Secret 都能被拆包取出。目标是 **挡住随意刷接口 + 限流**，不是做到银行级不可伪造。真要对抗黑产刷量，后期可加：证书绑定、定期轮换 key、异常 installId 熔断——仍不必上「用户登录」。

### 7.6 要不要单独开发后端？要不要新工程？

**要后端接口。** 现在的 `website/` 是 **纯静态**（Nginx 只吐 HTML/JSON），**不能**可靠地接收批量事件、做去重聚合、出 DAU/MAU 报表。客户端埋点必须 POST 到一个 **会写库的服务**。

**推荐：新建独立工程，仓库名优先 `boltshell-server`（见下节与专项文档）。**

```
boltshell-server/         ← 推荐仓库名（亦称 hub/cloud，见命名表）
├── cmd/api/              ← Go 后端入口
├── internal/...          ← analytics / admin / store
├── admin/                ← Vue3 管理台
├── deploy/               ← Nginx / docker-compose
└── README.md
```

| 工程 | 职责 | 和统计的关系 |
|------|------|----------------|
| **boltshell**（现有客户端） | Wails 桌面 App | 只负责采集、队列、批量 POST |
| **website/**（现有静态站） | 官网下载 / `sponsors.json` | 继续静态；Nginx 把 `/api/` 反代到 server |
| **boltshell-server**（新建） | 收事件、入库、聚合、管理台 | **统计后端 + 看数前端** |

命名对比、模块划分、表结构、技术选型详见：  
[Cloud / 服务端命名与架构](../engineering/BoltShell_Cloud服务架构.md)。

**部署拓扑（同一台机即可）：**

```text
客户端 ──POST──► https://boltshell.com/api/v1/analytics/events
                      │
                   Nginx
                   ├── /           → website 静态
                   ├── /config/*   → sponsors.json / release.json
                   └── /api/*      → boltshell-cloud:8080（新服务）

你们浏览器 ──► https://boltshell.com/admin/  → cloud 管理台（要登录）
```

**为什么单独工程：** 客户端发版 ≠ 服务端发版；管理台密钥与 DB 不要进桌面安装包。若暂不想多仓库，可在本仓加顶层 `cloud/`，但进程须独立部署，勿打进 `wails build`。

**管理台前端：**

| 阶段 | 建议 |
|------|------|
| **v1** | **必须有 API**（收事件 + 按日聚合）；管理台可极简甚至先 SQL/CSV |
| **v1.1** | 再做 `admin/` 看板 |
| 不必 | 一上来就做数据中台、用户体系、实时大屏 |

**v1 API 最小集：** `POST /api/v1/analytics/events`（客户端）；`GET .../admin/summary`、`export.csv`（管理台，另鉴权）。存储早期 SQLite 即可。

**落地顺序：** ① 新建 cloud 打通收事件 → ② 客户端 P0+P1 同包发版 → ③ 跑数后再补看板 / 涨价。

**一句话：** 要后端；建议 **新工程 = 后端 API +（可稍后的）管理前端**；官网继续静态；客户端只当上报方。

---

## 八、服务端怎么算出「给甲方的数」

假设事件表有：`day`, `installId`, `event`, `slotId`, `isPro`。


| 报表字段       | SQL 思路（示意）                                                               |
| ---------- | ------------------------------------------------------------------------ |
| 日启动次数      | `COUNT(*) WHERE event=app_launch AND day=?`                              |
| **免费 DAU** | `COUNT(DISTINCT installId) WHERE event=app_launch AND day=? AND isPro=0` |
| **免费 MAU** | 当月 `COUNT(DISTINCT installId)` 同上                                        |
| slot 日曝光   | `COUNT(*) WHERE event=sponsor_impression AND slotId=? AND day=?`         |
| slot 日点击   | `COUNT(*) WHERE event=sponsor_click AND …`                               |
| CTR        | 点击 / 曝光                                                                  |
| 关闭率        | dismiss / impression                                                     |
| Pro 占比     | 当日有 launch 且 isPro=1 的去重 / 全体 DAU                                        |


**媒体包一页纸可直接填：**

1. 免费 MAU（上月）
2. 免费 DAU（近 7 日均值）
3. `sidebar_1` 月曝光、月点击、CTR
4. `quick_connect_bottom` 同上（若在售）
5. 关闭率（健康度）

---



## 九、没埋点时的估算（过渡期）

有下载量、还没统计服务时，对外可写清「估算」：

```text
免费 MAU ≈ 累计下载 × 活跃系数（早期可试 0.15～0.35，有数据后校准）
sidebar_1 月曝光 ≈ 免费 MAU × 人均连接天数 × 每天侧栏曝光次数
```

商业化文档粗算用过「每人每天约看广告 2 次」——埋点上线后用真实 `impressions / free_dau` 替换该假设。

估算对外时必须同时满足：

- 明确标注「估算 / 非正式埋点」
- 优先报 **区间**（如免费 MAU 约 800–1500），避免装成精确审计数
- 有埋点后 **以实数为准**，替换口头估算，不要两套数并存糊弄甲方

---

## 十、报数诚信：禁止夸大指标

> 技术上可以改数字，但 **故意夸大 DAU / MAU / 曝光 / 点击 给甲方看，不应做。**

### 10.1 夸大会有什么影响

| 层面 | 影响 |
|------|------|
| **甲方核验** | 云厂商等常会看 UTM、落地页 UV，或自己点几次对口径；虚高很容易穿帮 |
| **续约** | 第一月夸大，第二月真实曝光对不上 → 不续约、要求解释甚至退款 |
| **口碑** | 运维 / 开发者圈小，一次造假会传开，比少报几个 MAU 更伤品牌 |
| **合同** | 若写进「保底曝光 / 保底 DAU」却兑不了，可能违约、扯皮 |
| **内部决策** | 假数会误导产品与定价（真以为用户很多） |

埋点一旦上线，**实数与口头数会对不上**——「先口头夸大、以后再埋点」比一直不埋点更危险。

### 10.2 明确禁止

| ❌ 禁止 | 说明 |
|--------|------|
| 改上报逻辑 / 刷机 / 伪造报表 | 属于对赞助商造假 |
| 把下载量说成 MAU | 口径欺诈（下载通常大于活跃） |
| 把 `sponsors.json` 拉取次数说成曝光 | 配置请求 ≠ 用户看见 |
| 把全体用户说成「都能看到广告」 | Pro 不去广告，可售触达更小 |
| 合同写死虚高保底曝光却无兑现能力 | 法律与商务双重风险 |

### 10.3 早期人少时的正确做法（不是造假）

1. **写清「估算」**：下载量 × 活跃系数，并注明非正式埋点  
2. **报区间**：如「免费 MAU 约 800–1500（估算）」  
3. **卖 CPT 包月**：按位卖时间，少绑死「保证 XX 万曝光」  
4. **用人群价值补量**：强调「SSH 运维精准人群」，比虚报 UV 更有说服力  
5. **给可核验项**：跳转链接带 UTM，让甲方自己看落地页转化  

### 10.4 结论（对外口径）

> 夸大短期可能好开口，长期几乎必然反噬。  
> 人少时用「诚实估算 + 包月赞助 + 精准人群」谈单；有埋点后只报实数。

---

## 十一、分阶段落地（相对发版的时间，不是「发完再做」）

> **P0 + P1 必须进「带赞助位的正式包」**；P2/P3 可随后续小版本，但仍受「不升级就采不到」约束，宜尽早。

| 阶段 | 做什么 | 与发版关系 | 产出 |
|------|--------|------------|------|
| **P0** | `installId` + `app_launch` + 本地队列 + 可上报 | **发版前必做** | 真实 DAU/MAU |
| **P1** | impression / click / dismiss + 曝光去重 | **发版前必做**（与赞助 UI 同包） | 分 slot 报表，可谈赞助 |
| **P2** | 设置页开关文案打磨、管理后台/CSV | 可紧随首发后的小版本；开关建议首发就有 | 给甲方导出 |
| **P3** | （可选）`ssh_connected`、留存 | 后续版本；仅覆盖升级用户 | 更严活跃与留存 |

**明确不做：** 第三方广告 SDK、实时竞价、用户行为画像、与 SSH 会话数据关联；**不做虚报 / 刷量。**

**错误节奏（禁止）：** 先发无埋点正式版 → 用户装上旧包 → 再发带埋点版 → 大量用户不升级 → **永远统计不全。**

**正确节奏：** 统计客户端代码 + 赞助 UI **同包首发** → 跑 2～4 周攒数 → 再考虑「有数据后涨价」。

---

## 十二、和现有模块的衔接

| 现有能力 | 统计侧用法 |
|----------|------------|
| `GetSponsorConfig` / Pro 过滤 | impression 仅在有 Slot 返回时发生 |
| `DismissSponsorSlot` + `sponsor.state` | 打 `sponsor_dismiss`；关闭期内无 impression |
| `sponsors.remote.json` | 可扩展 `analyticsURL` |
| `appdata.Dir()` | 放 `install.id`、队列文件 |
| 官网下载计数（nginx / 对象存储） | 补「下载量」；与客户端 MAU 分开报 |

---

## 十三、验收标准（实现时对照）

1. 关闭网络时，主功能与赞助展示正常；队列仅本地堆积。
2. Pro 用户界面无赞助位，且无 sponsor_* 事件（或极少）。
3. 抓包可见上报 **不含** 主机 IP/命令。
4. 同一快速连接页挂载期间，同一 slot 不会连续刷几十次 impression。
5. 用测试 `installId` 跑一天，服务端能对上：1 DAU、对应 slot 曝光/点击次数。
6. 对外报表与库内聚合一致，**无**「口头一套、库内一套」。
7. **发版门禁 §1.1 Checklist 全部勾完**，才允许作为带赞助位的正式渠道包分发。

---

## 十四、本地开发：配置、建表与逐场景验证

> 目标：在本机同时跑客户端 + `boltshell-server`，把埋点链路跑通。  
> **不必手写建表 SQL**（见 §14.1）；验证时 **不要指望 WebView DevTools → Network** 能看到统计 POST（上报由 Go 发出）。

### 14.1 要不要手动创建数据库表？

| 情况 | 要不要手动建表 |
|------|----------------|
| 默认本地开发（SQLite + `disable-auto-migrate: false`） | **不用**。服务端启动时 GORM `AutoMigrate` 会创建/补齐：`bs_analytics_events`、`bs_daily_install_active`、`bs_daily_slot_stats` |
| 已有库、只是刚拉了带统计的代码 | **不用**。重启服务端即可自动加表 |
| 生产环境关闭了自动迁移（`disable-auto-migrate: true`） | **要**。执行 `boltshell-server/server/sql/bs_analytics.sql` |
| 想对照字段 | 可看上述 SQL 文件，仅作文档/手工迁移参考 |

主键均为 **字符串 UUID**，状态类字段如 `is_pro` 用字符串 `true`/`false`。

### 14.2 配置文件在哪（本地默认）

| 角色 | 文件 | 关键项 |
|------|------|--------|
| 客户端上报地址 | `boltshell/server/config/sponsors.remote.json` | `analyticsURL` → `http://127.0.0.1:8888/api/v1/analytics/events`；`analyticsAppKey` / `analyticsAppSecret` |
| 客户端内置副本 | `boltshell/server/internal/sponsors/remote.embed.json` | 发版前与上一文件同步；`wails dev` 一般优先读到仓库里的 `config/sponsors.remote.json` |
| 服务端端口与密钥 | `boltshell-server/server/config.yaml` | `system.addr: 8888`；`analytics.enabled` / `app-key` / `app-secret`（**须与客户端一致**） |
| 用户侧数据（勿提交） | `%LOCALAPPDATA%\BoltShell\` | `install.id`、`analytics.queue`、`analytics.prefs.json` |

密钥不一致 → 上报 401，队列不会被消费掉（可下次改对后再 Flush）。

### 14.3 如何启动（本地）

**服务端（先起）：**

```powershell
cd e:\resource\person\boltshell\boltshell-server\server
go run .
```

- API：`http://127.0.0.1:8888`
- 健康：`GET http://127.0.0.1:8888/health` → `"ok"`
- 管理台前端（可选）：`cd ..\web` → `npm run dev`

**客户端：**

```powershell
cd e:\resource\person\boltshell\boltshell\server
wails dev
```

左侧栏「统」= 统计开，「静」= 关（默认开）。

### 14.4 验证时看哪里（总览）

| 现象 | 正确观察方式 | 错误期待 |
|------|--------------|----------|
| 点赞助位开外链 | 系统默认浏览器打开落地页 | WebView Network 里出现导航请求 |
| 埋点写入 | `%LOCALAPPDATA%\BoltShell\analytics.queue` | 前端 fetch 日志 |
| 上报到服务端 | 表 `bs_analytics_events` / admin API / 服务端日志 | DevTools Network |
| 立刻冲刷队列 | DevTools Console：`window.go.main.App.FlushAnalytics()` | 等 5 分钟定时器（也可等，但不便调试） |

---

### 14.5 场景 A：服务端存活 + 表已自动建好

**步骤：**

1. 启动 `go run .`，日志无 AutoMigrate 报错。
2. 浏览器或 curl：`http://127.0.0.1:8888/health`。
3. 用 DB 工具打开 `boltshell-server/server/boltshell.db`（路径以 `config.yaml` → `sqlite.path` + `db-name` 为准，默认当前目录 `boltshell.db`）。

**通过标准：**

- health 返回 ok  
- 存在三张表：`bs_analytics_events`、`bs_daily_install_active`、`bs_daily_slot_stats`  
- **无需**事先执行建表 SQL  

---

### 14.6 场景 B：`app_launch`（启动即记）

**步骤：**

1. 确认左侧「统」为开启。
2. 完全退出客户端后重新 `wails dev` / 启动。
3. 打开 `%LOCALAPPDATA%\BoltShell\analytics.queue`（JSON 数组）。
4. Console 执行：`await window.go.main.App.FlushAnalytics()`。
5. 查库：

```sql
SELECT id, event, install_id, app_version, os, is_pro, event_ts
FROM bs_analytics_events
WHERE event = 'app_launch'
ORDER BY received_at DESC
LIMIT 10;
```

**通过标准：**

- 队列或库中出现 `event=app_launch`  
- 同一次启动通常只有 1 条（再启动会再增加）  
- `install_id` 与 `%LOCALAPPDATA%\BoltShell\install.id` 文件内容一致  
- `os` 为 `windows` / `darwin` / `linux` 之一  

**失败排查：**

- 队列没有：开关是否关闭、`initAnalytics` 是否报错（看客户端日志）  
- 队列有、库没有：`analyticsURL` 是否指向 `127.0.0.1:8888`、服务端是否在跑、AppKey/Secret 是否一致、是否执行了 Flush  

---

### 14.7 场景 C：赞助位曝光 `sponsor_impression`

**步骤：**

1. 非 Pro，能看到赞助位（快速连接底栏或连接后侧栏）。
2. 进入对应界面，让 `SponsorBanner` 完成挂载。
3. 看队列或 Flush 后查库：

```sql
SELECT slot_id, event, link_host, config_version, event_day
FROM bs_analytics_events
WHERE event = 'sponsor_impression'
ORDER BY received_at DESC;
```

4. **去重验证：** 在同一界面态下反复触发重绘/不离开该态，同一 `slot_id` 不应狂刷几十条；断线回到快速连接（新的 surfaceSession）或新开会话侧栏，允许再记 1 次。

**通过标准：**

- 每个可见 slot 在同一 `surfaceSession + 自然日` 下约 1 次 impression  
- Pro 用户无赞助 UI → 无（或极少）`sponsor_*`  
- 仅拉 `sponsors.json` **不会**产生 impression  

---

### 14.8 场景 D：点击开链 `sponsor_click` + BrowserOpenURL

**步骤：**

1. 点击侧栏/底栏赞助条（如「开源」「Pro」）。
2. 观察 **系统浏览器** 是否打开配置里的 `linkUrl`（不是 WebView 内跳转）。
3. Flush 后查库：

```sql
SELECT slot_id, link_host, event_ts
FROM bs_analytics_events
WHERE event = 'sponsor_click'
ORDER BY received_at DESC
LIMIT 10;
```

**通过标准：**

- 外链能打开（主功能优先；埋点失败不应挡住打开）  
- 库中有对应 `sponsor_click`；`link_host` 仅为 host（如 `gitee.com`），**无**完整 path/query，**无** IP 当 host  
- DevTools Network **可以没有** 任何与点击相关的文档请求 —— 这是正常的  

**失败排查：**

- 浏览器没开：看 Console 是否有 runtime 报错；Console 试 `window.runtime.BrowserOpenURL('https://example.com')`  
- 有打开无 click 事件：看队列、开关、是否 Pro  

---

### 14.9 场景 E：关闭赞助位 `sponsor_dismiss`

**步骤：**

1. 鼠标悬停赞助条，点右上角「×」。
2. 该 slot 应暂时消失（`sponsor.state` 关闭期）。
3. Flush 后查库 `event = 'sponsor_dismiss'`。
4. 关闭期内再进界面：不应再出现该位，也不应再刷 impression。

**通过标准：**

- UI 隐藏 + 有 `sponsor_dismiss` 事件  
- 关闭期内无该 slot 的新 impression  

---

### 14.10 场景 F：`ssh_connected`（可选 P3，已实现首次会话）

**步骤：**

1. 成功建立一次 SSH 会话（`StartSession` 成功）。
2. Flush 后查库 `event = 'ssh_connected'`。

**通过标准：**

- 同一进程生命周期内通常只记 **1 次**（首次连接成功）  
- 事件中 **不含** 主机名、IP、端口、命令  

---

### 14.11 场景 G：隐私开关关闭

**步骤：**

1. 点左侧「统」→ 变为「静」（关闭）。
2. 重启或继续操作赞助位。
3. 看 `analytics.prefs.json` 中 `enabled: false`。
4. 队列不应再增长（或不再新增事件）；Flush 也不应再往服务端推新事件。

**通过标准：**

- 关闭后主功能、赞助展示仍正常  
- 无新埋点入队  

再点「静」→「统」可恢复。

---

### 14.12 场景 H：断网 / 服务端未启动（本地队列）

**步骤：**

1. 停掉 `boltshell-server`（或改错 `analyticsURL`）。
2. 正常使用客户端：启动、看广告、点击。
3. 确认 `analytics.queue` 仍在增长。
4. 主功能（SSH/SFTP）不受影响。
5. 再启动服务端，执行 `FlushAnalytics()` 或等待定时上报。

**通过标准：**

- 断网/无服务时功能正常，队列堆积  
- 恢复后能成功入库，队列缩短或清空  

---

### 14.13 场景 I：签名与限流（防刷）

**步骤：**

1. 用错误 `app-secret` 的客户端配置启动 → Flush → 应失败，队列保留。
2. 用 curl **不带** `X-BoltShell-*` 头 POST `/api/v1/analytics/events` → 401。
3. （可选）短时间大量请求 → 触发 429 限流。

**通过标准：**

- 无合法签名不能污染库  
- 合法客户端改回正确密钥后可继续上报  

示例（合法签名需自算 HMAC，调试更简单的方式是改 yaml 密钥做正反对比）：

```http
POST /api/v1/analytics/events
Content-Type: application/json
X-BoltShell-App-Key: boltshell-desktop
X-BoltShell-Ts: <unix秒>
X-BoltShell-Sign: <hmac-sha256(secret, ts + "\n" + sha256hex(body))>
```

---

### 14.14 场景 J：管理端看数 / CSV（给甲方口径）

管理台侧栏：**统计数据 → 赞助统计**（页面 `view/analytics/index.vue`）。

- 支持日期区间、按 `slotId` 过滤、查看免费 MAU / 近 7 日均免费 DAU、分日 DAU、分 slot 曝光/点击/CTR  
- **导出 CSV** 按钮下载 slot 汇总  

也可直接调 API（需登录 JWT / `x-token`）：

| 接口 | 作用 |
|------|------|
| `GET /api/v1/analytics/admin/summary?from=YYYY-MM-DD&to=YYYY-MM-DD` | 免费/全体 MAU、分日 DAU、近 7 日均免费 DAU |
| `GET /api/v1/analytics/admin/slots?from=...&to=...&slotId=sidebar_1` | 分 slot 曝光/点击/关闭/CTR |
| `GET /api/v1/analytics/admin/export.csv?from=...&to=...` | CSV 导出 |

**首次上菜单（已有库）：** 执行 `server/sql/seed_analytics_menu.sql`，然后 **重启后端** 并 **重新登录** 管理台（动态路由刷新）。新库初始化会走 `source/system/menu.go` / `api.go` / `casbin.go`。

**通过标准（自洽）：**

- 侧栏能看到「统计数据 / 赞助统计」  
- 今日本机测过：summary 里免费 DAU ≥ 1（`is_pro=false` 的 launch）  
- slots 里 impression / click 与手动点的次数大致一致（注意去重规则）  
- CTR ≈ clicks / impressions  
- Casbin 若 403：确认 `casbin_rule` 已有上述 3 条 GET，并重启服务端  

也可直接 SQL 对账：

```sql
-- 今日免费 DAU
SELECT COUNT(DISTINCT install_id)
FROM bs_daily_install_active
WHERE day = date('now','localtime') AND is_pro = 'false';

-- 某 slot 今日汇总
SELECT * FROM bs_daily_slot_stats
WHERE day = date('now','localtime') AND slot_id = 'sidebar_1';
```

---

### 14.15 场景 K：一键冒烟清单（每次改统计代码后）

按顺序勾选：

- [ ] 服务端 `go run .`，health ok，三张 `bs_*` 表存在（**未**手工建表）  
- [ ] 客户端 `sponsors.remote.json` 的 `analyticsURL` 指向 `127.0.0.1:8888`  
- [ ] AppKey/Secret 与 `config.yaml` 一致  
- [ ] 冷启动 → queue/库有 `app_launch`  
- [ ] 看见赞助位 → 有 `sponsor_impression`（不过度刷）  
- [ ] 点击 → 系统浏览器打开 + 有 `sponsor_click`  
- [ ] 点 × → 位消失 + 有 `sponsor_dismiss`  
- [ ] Flush 成功，admin summary / slots 能看到数  
- [ ] 关「统」后不再入队；开网/关服务时主功能正常  

---

*文档结束。**带赞助位的正式包发布前，必须完成 P0+P1**；服务端可简陋，客户端埋点不能缺。本地开发默认走 `127.0.0.1:8888`，表由 AutoMigrate 自动创建。*