# BoltShell 前端结构说明

> 更新时间：2026-08-20  
> 技术栈：Vue 3 + TypeScript + Vite + xterm.js，经 Wails v2 嵌入桌面窗口  
> 文档索引：[docs/README.md](../README.md) · 关联：[后端架构](BoltShell_后端架构评审.md)

本文按「目录 → 每个文件做什么 → 数据怎么流」说明 `web/`。业务逻辑在 `composables/`，界面在 `components/`，`App.vue` 只负责编排。

---

## 一、整体职责

前端是桌面客户端的 UI 层，不直连 SSH。它通过 Wails 生成的 `web/wailsjs/go/main/App.js` 调用 Go 方法，并通过 `EventsOn` 接收后端推送。

| 用户看到的区域 | 对应代码 |
|----------------|----------|
| 最左侧 40px 导航 | `LeftRail.vue` |
| 系统监控侧栏 | `SysInfoPanel.vue` + `useSysInfo` |
| 顶部会话 Tab | `SessionTabBar.vue` |
| 未连接时的主机列表 | `QuickConnectPanel.vue` |
| 中间终端 | `useTerminal` 把 xterm 挂到 `App.vue` 的 div |
| 底部 SFTP | `FilePane.vue` + `useFilePane` |
| 右侧传输列表 | `TransferPanel.vue` + `useTransferTasks` |
| 连接管理 / 表单 | `connection/*` + `useConnections` |
| 赞助位 | `SponsorBanner.vue` + `useSponsors` |
| 升级提示 | `UpdatePromptModal.vue` + `useUpdater` |

---

## 二、目录结构（与仓库一致）

```
web/
├── index.html                 # HTML 壳：#app + #wails-spinner
├── package.json               # 依赖与脚本（vite / vitest / vue-tsc）
├── package-lock.json
├── vite.config.ts             # Vite + Wails spinner 补丁 + vitest
├── tsconfig.json / tsconfig.node.json
├── public/                    # 静态资源（不经 Vite 哈希）
│   ├── favicon.ico / favicon.png / apple-touch-icon.png
│   └── config/sponsors.json   # 开发时本地赞助配置兜底
├── docs/                      # 交互原型 HTML（非运行时代码）
│   ├── INTERACTION_PROTOTYPE.html
│   └── FINALSHELL_FTP_PROTOTYPE.html
├── wailsjs/                   # Wails 自动生成，勿手改
│   ├── go/main/App.js         # 调用 Go App 方法
│   ├── go/main/App.d.ts
│   ├── go/models.ts           # Go struct 对应的 TS 类型
│   └── runtime/               # EventsOn、BrowserOpenURL 等运行时
└── src/
    ├── main.ts
    ├── App.vue
    ├── style.css
    ├── vite-env.d.ts
    ├── styles/legacy-ui.css
    ├── types/
    ├── constants/app.ts
    ├── utils/
    ├── composables/
    ├── components/
    └── assets/fonts/          # Nunito 字体 + OFL 许可
```

---

## 三、工程入口与构建文件

### `index.html`

SPA 入口。`#app` 给 Vue 挂载；`#wails-spinner` 给 Wails 开发 Overlay 用，避免 `querySelector('#wails-spinner')` 为空报错。

### `main.ts`

创建 Vue 应用并 `mount('#app')`。必须先引入 `xterm/css/xterm.css`，再引入 `style.css`。漏掉 xterm 样式时，字符测量节点（一串 `W`）和隐藏 textarea 会露在终端里。

### `App.vue`

布局编排层。组合全部 composable，把 props/事件传给子组件；在 `onMounted` 里通过 `subscribeBackendEvents` 订阅后端事件。文件编辑保存等少量胶水逻辑仍留在这里（`ReadRemoteFile` / `WriteRemoteFile`）。

### `style.css`

全局底色、字体、`@font-face`（Nunito）。不包含面板级 UI。

### `styles/legacy-ui.css`

从早期单体 `App.vue` 抽出的界面样式（连接管理器、文件面板、Tab、传输栏等）。尚未按组件拆成 scoped style。

### `vite-env.d.ts`

Vite 客户端类型 + `*.vue` 模块声明，让 TypeScript 能 import Vue 单文件组件。

### `vite.config.ts`

- `@vitejs/plugin-vue`
- `wailsSpinnerAnchor`：开发时保证页面有 `#wails-spinner`
- vitest：`happy-dom`，匹配 `src/**/*.test.ts`
- Windows 下对 `wailsjs` 用 polling，避免绑定更新后 Vite 缓存不失效

### `package.json`

| 脚本 | 作用 |
|------|------|
| `dev` | 仅 Vite（日常用 `wails dev`） |
| `build` | `vue-tsc --noEmit` 后再打包 |
| `build:release` | 跳过类型检查直接 Vite 打包 |
| `test` / `test:watch` | vitest |

依赖：`vue`、`xterm`、`xterm-addon-fit`。

### `public/`

打包时原样拷到产物根目录。图标给窗口/标签页；`config/sponsors.json` 可在开发环境作为赞助配置本地文件。

### `web/docs/`

FinalShell 风格交互/FTP 原型，给设计对照，不参与 `wails build`。

### `web/wailsjs/`

`wails generate` / `wails dev` 生成。前端只应从这里调 Go，不要手改。`App.js` 里每个 `export function Xxx` 对应后端 `App` 的导出方法。

---

## 四、类型与常量

### `types/index.ts`

与 Go JSON 字段对齐的共享类型：

| 类型 | 含义 |
|------|------|
| `Connection` | SQLite 连接配置（对应 `db.Connection`） |
| `SessionTab` | 已打开的 SSH Tab（含 `closed`） |
| `RemoteEntry` | SFTP 目录项 |
| `FilePaneState` | 每个会话一份的文件面板状态 |
| `SysInfo` / `SysProcInfo` | 远端资源与进程快照 |
| `TransferTask` | 本地上传/下载任务 |
| `MgrMessageType` | 连接管理器提示条样式 |
| `AdSlot` | 旧赞助结构，已被远程 `sponsors.json` 替代 |

### `types/sponsors.ts`

`SponsorSlot` / `SponsorConfig`，对齐 Go 的 `SponsorConfigView`（含 `SlotID`、`IsPro`、`DismissedUntil`）。

### `types/updater.ts`

`UpdateCheckResult`：当前/最新版本、是否有更新、发行说明、下载地址、是否强制更新。

### `constants/app.ts`

localStorage Key，避免魔法字符串散落：

| 常量 | 用途 |
|------|------|
| `DOWNLOAD_DIR_KEY` | 默认本地下载目录 |
| `RECENT_KEY` | 快速连接最近使用 |
| `EMPTY_GROUPS_KEY` | 尚无连接的空分组名 |
| `UPDATE_DISMISS_KEY` | 用户关掉过的升级提示版本号 |

---

## 五、工具函数

### `utils/format.ts`

纯函数，不碰 Wails：

- `formatBytes` / `formatFileSize`：字节显示
- `formatModTime`：远端修改时间
- `connTitle` / `groupName`：连接标题与分组（空分组显示「未分组」）
- `errText`：把 Wails/Go 错误收成可读字符串
- `joinLocalPath`：按 Windows `\` 或 Unix `/` 拼本地路径
- `emptySysInfo`：系统信息面板初始空数据

### `utils/terminalXterm.test.ts`

回归：确认 `main.ts` 引入了 `xterm.css`，且该 CSS 会隐藏测量元素和 helper textarea。没有对应的 `terminalXterm.ts` 源文件。

---

## 六、Composables（业务逻辑）

每个 composable 只做一块领域，返回 state + 方法给 `App.vue`。

### `useConnections.ts`

连接管理器 + 会话 Tab + 快速连接。

- 调 `ListConnections` / `AddConnection` / `UpdateConnection` / `SetDeleted`
- `StartSession` 开 Tab，`CloseSession` 关会话
- 最近连接、空分组写入 localStorage
- 维护 `sessions`、`activeSessionID`、管理器/表单弹窗开关
- `isTypingInForm`：表单输入时不要把按键送到 xterm

测试：`useConnections.test.ts`。

### `useTerminal.ts`

xterm.js 生命周期。

- 每 session 一个 `Terminal` + `FitAddon`
- 输出缓冲上限约 512KB，避免内存涨太快
- `writeToTerminal`：后端 `terminal-output`
- `onData` → `SendSessionInput`
- 尺寸变化 → `ResizeSession`（用 `syncedCols/Rows` 避免重复 WindowChange）
- `markSessionClosed`：Tab 标断开
- 导出 `parseEventArgs`：兼容 Wails 多种事件参数格式

测试：`useTerminal.test.ts`。

### `useBackendEvents.ts`

统一订阅后端事件。必须用 `EventsOn`（流式），不能用 `EventsOnMultiple(..., 1)`，否则只收到第一条。热更新前先 `EventsOff`。事件名见下文「数据流」。

测试：`useBackendEvents.test.ts`。

### `useFilePane.ts`

每个 SSH 会话一份 SFTP 状态（路径、列表、选中项）。

- `ListRemoteDir` / `GetRemoteHome` / `MkdirRemote` / `RemoveRemote` / `RenameRemote`
- `UploadToRemote` / `DownloadFromRemote`
- `PickLocalFile` / `PickLocalDir` / `PickSaveFile` / `PickDownloadDir`
- 路径栏编辑、目录树、打开编辑器回调、打开传输面板回调

### `useTransferTasks.ts`

本地上传/下载任务列表。

- `upsertTransfer`：吃 `transfer-update`
- 默认下载目录（localStorage）
- `PickDownloadDir` / `OpenLocalFolder`
- `activeTransferCount` 给 Tab 栏徽章

### `useServerTransfer.ts`

跨服务器传送对话框。

- `ListConnections` 选目标机
- `GetConnectionHome` / `BrowseConnectionDir`（走后端 browse 连接池，不必先开终端 Tab）
- `TransferToConnection`
- 进度可接到 `srv-transfer-progress` / `transfer-log`

### `useSysInfo.ts`

当前活动会话每 5 秒 `GetSessionSysInfo`。算出内存/交换/磁盘百分比。无活动会话或 Tab 已断开则停轮询。

### `useSponsors.ts`

启动拉 `GetSponsorConfig`；可 `RefreshSponsorConfig`、`DismissSponsorSlot`。拆出 `quickSlot`（快速连接底）和 `sidebarSlots`（系统信息侧栏）。Pro 用户后端不返回 Slots。

### `useUpdater.ts`

启动静默 `CheckForUpdate`；有新版本且该版本未被 dismiss 则弹出提示。`ApplyUpdate` 下载安装并重启。超时约 5 分钟。dismiss 写入 `UPDATE_DISMISS_KEY`。

---

## 七、组件（按界面区域）

组件尽量无业务：只收 props、向外 emit。

### `components/layout/LeftRail.vue`

最左侧窄栏：打开连接管理器；底部「检查更新 / 升级」按钮（显示当前版本或新版本提示）。

### `components/sysinfo/SysInfoPanel.vue`

CPU、内存、交换、磁盘、Top 进程；下方可嵌多个 `SponsorBanner`（compact）。

### `components/session/SessionTabBar.vue`

会话 Tab（选中、关闭、断开样式）；折叠系统栏/文件栏；打开传输面板（带进行中数量）。

### `components/session/QuickConnectPanel.vue`

无活动会话时的快速连接列表；底部 banner 赞助位。

### `components/file/FilePane.vue`

底部 SFTP：工具栏、路径、目录树、文件表。内嵌 `FileEditorModal`、`ServerTransferModal`。`fileTab: 'files' | 'cmd'` 里 `cmd`（批量命令）尚未独立成组件。

### `components/file/FileEditorModal.vue`

远端文件文本编辑。内容由 `App.vue` 调 `ReadRemoteFile` / `WriteRemoteFile`。

### `components/file/ServerTransferModal.vue`

选目标连接、浏览远端目录、确认传送、看日志。

### `components/transfer/TransferPanel.vue`

右侧任务列表：进度、状态、选下载目录、打开本地下载文件夹、清除已完成。

### `components/connection/ConnectionManager.vue`

分组树 + 连接列表：搜索、过滤、含已删除、连接后关闭、新建/编辑/删除。

### `components/connection/ConnectionFormModal.vue`

新建/编辑：主机、端口、账号、密码、分组、启用。

### `components/connection/AddGroupModal.vue`

只新增分组名（可先建空分组，连接稍后再加）。

### `components/sponsor/SponsorBanner.vue`

`banner` / `compact`。点击用 `BrowserOpenURL` 打开外链；关闭则 emit dismiss。

### `components/updater/UpdatePromptModal.vue`

启动时的新版本弹窗：版本号、发行说明、立即升级 / 关闭。

---

## 八、数据流

```
App.vue
  ├── useConnections()      → ConnectionManager / QuickConnect / SessionTabBar
  ├── useTerminal()         → 中间 xterm 宿主 div
  ├── useFilePane()         → FilePane
  ├── useTransferTasks()    → TransferPanel
  ├── useServerTransfer()   → ServerTransferModal
  ├── useSysInfo()          → SysInfoPanel
  ├── useSponsors()         → SponsorBanner（快速连接底 + 侧栏）
  └── useUpdater()          → LeftRail 升级按钮 + UpdatePromptModal

Wails EventsOn（useBackendEvents）
  terminal-output         → useTerminal.writeToTerminal
  terminal-closed         → useTerminal.markSessionClosed
  transfer-update         → useTransferTasks.upsertTransfer
  transfer-log            → 跨服传送日志
  srv-transfer-progress   → 跨服传送进度
```

调用方向：组件 emit → App 调 composable → composable 调 `wailsjs/go/main/App.js` → Go `App` 方法。

---

## 九、拆分前后对比

| 指标 | 拆分前 | 当前 |
|------|--------|------|
| App.vue | ~3700 行 | 编排层（约数百行） |
| 组件 | 1 | 14（含赞助、升级） |
| Composables | 0 | 9 |
| 可维护性 | 低 | 中高 |

---

## 十、后续可继续拆分

1. `legacy-ui.css` 按组件改成 scoped styles
2. `useConnections.ts` 拆成 `useConnectionManager` + `useSessionTabs`
3. 命令面板 Tab（批量命令）独立 `CmdPanel.vue`
4. `App.vue` 里文件编辑器状态可下沉到 `useFileEditor`

---

*实现细节以各 composable / 组件文件头注释为准。*
