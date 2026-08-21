# BoltShell 文档索引

文档按模块分目录，便于查找。

| 模块 | 目录 | 内容 |
|------|------|------|
| **产品与品牌** | [`product/`](product/) | 定位、命名、品牌资产、CLI 规划、赞助位 UI 规划 |
| **商业化与广告** | [`business/`](business/) | 开源策略、赞助位配置、广告术语与报数口径 |
| **工程与架构** | [`engineering/`](engineering/) | 前端结构、后端架构、开发调试与发版 |
| **待办** | [`TODO.md`](TODO.md) | 跨模块待办清单 |

---

## 产品与品牌 · `product/`

| 文档 | 说明 |
|------|------|
| [品牌与产品规划文档](product/BoltShell_品牌与产品规划文档.md) | 产品定位、命名、域名/商标、命令行体验、赞助位 UI 规划 |
| [`logo/`](product/logo/) | Logo 设计稿（图标 / 横版 / 深色） |

## 商业化与广告 · `business/`

| 文档 | 说明 |
|------|------|
| [商业化与开源策略](business/BoltShell_商业化与开源策略.md) | Freemium / Pro / Team、开源边界、收入粗算 |
| [赞助位配置说明](business/BoltShell_赞助位配置说明.md) | 远程 `sponsors.json`、slotId、客户端读取路径 |
| [广告术语与指标说明](business/BoltShell_广告术语与指标说明.md) | PV / UV / DAU / MAU / slot / CTR / CPM / CPT / SDK 等口径 |
| [赞助位报价与市场参考](business/BoltShell_赞助位报价与市场参考.md) | 双页位数量、刊例、报价示例、公开市场价格与曝光参考 |
| [客户端赞助数据统计设计](business/BoltShell_客户端赞助数据统计设计.md) | SSH 客户端如何埋点、去重曝光、上报与汇总报数；含 cloud 工程建议 |
| [竞品变现与广告风险调研](business/BoltShell_竞品变现与广告风险调研.md) | SSH / 工具类软件靠广告能不能赚钱、同行踩坑案例、云厂商 CPS、赞助位行为约束 |
| [开源许可证详解与选型](business/BoltShell_开源许可证详解与选型.md) | MIT / Apache / GPL / BSL / ELv2 逐条详解、许可证防不住什么、是否开源的决策框架 |
| [个人开发者商业化 FAQ](business/BoltShell_个人开发者商业化FAQ.md) | 一个人怎么赚钱；卖解锁 vs 卖服务；FinalShell 导入；埋点信任；子域可屏蔽 |
| [产品亮点与竞品对比](business/BoltShell_产品亮点与竞品对比.md) | 亮点、竞品表、P0/P1、**如何宣传**、**Pro 额外功能**、Go/Termius/跨机传说明 |

## 工程与架构 · `engineering/`

| 文档 | 说明 |
|------|------|
| [前端结构说明](engineering/BoltShell_前端结构说明.md) | Vue 目录、组件、状态与赞助位接入 |
| [后端架构评审](engineering/BoltShell_后端架构评审.md) | Go / Wails 分层、API、安全与待改进 |
| [Cloud / 服务端命名与架构](engineering/BoltShell_Cloud服务架构.md) | `boltshell-server` 命名、Go+Vue 单体、统计 API 设计 |
| [开发调试与正式部署](engineering/开发调试与正式部署.md) | 本地开发、打包、发版、GitHub Actions |

## 相关（非本目录）

| 路径 | 说明 |
|------|------|
| [`server/config/README.md`](../server/config/README.md) | 项目 `server/config/`：远程入口、赞助兜底、升级模板（与用户 `config.json` 区分） |
| [`server/config.example.json`](../server/config.example.json) | 用户运行时 `config.json` 字段说明与示例 |
| [`website/`](../website/) | 官网静态页与线上 `sponsors.json` / `release.json` |
| [`web/docs/`](../web/docs/) | 交互原型 HTML（非运行时代码） |

---

*从项目根 README 进入完整开发说明时，请看 [`engineering/开发调试与正式部署.md`](engineering/开发调试与正式部署.md)。*
