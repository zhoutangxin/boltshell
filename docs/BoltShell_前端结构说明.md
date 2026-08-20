# BoltShell 前端结构说明

> 更新时间：2026-08-20

---

## 一、目录结构

```
frontend/src/
├── App.vue                 # 布局编排层（~350 行，组合 composables + 组件）
├── main.ts                 # Vue 入口
├── style.css               # 全局基础样式
├── styles/
│   └── legacy-ui.css       # 从原 App.vue 提取的 UI 样式（~1600 行）
├── types/
│   └── index.ts            # Connection、SessionTab、SysInfo 等共享类型
├── utils/
│   └── format.ts           # formatBytes、connTitle、errText 等工具函数
├── constants/
│   └── app.ts              # localStorage Key、赞助位配置
├── composables/
│   ├── useConnections.ts   # 连接管理器 + 会话 Tab + 快速连接
│   ├── useTerminal.ts      # xterm.js 生命周期与 Wails 终端事件
│   ├── useFilePane.ts      # SFTP 文件面板逻辑
│   ├── useTransferTasks.ts # 本地上传/下载任务列表
│   ├── useServerTransfer.ts# 跨服务器传送对话框
│   └── useSysInfo.ts       # 系统信息 5s 轮询
└── components/
    ├── layout/
    │   └── LeftRail.vue           # 左侧 40px 导航栏
    ├── sysinfo/
    │   └── SysInfoPanel.vue       # CPU/内存/磁盘/进程 + 赞助位
    ├── session/
    │   ├── SessionTabBar.vue      # 顶部 Tab 栏
    │   └── QuickConnectPanel.vue  # 快速连接 + 底部赞助 Banner
    ├── file/
    │   ├── FilePane.vue           # SFTP 文件面板主体
    │   ├── FileEditorModal.vue    # 远端文件编辑器
    │   └── ServerTransferModal.vue# 跨服务器传送弹窗
    ├── transfer/
    │   └── TransferPanel.vue      # 右侧传输进度面板
    └── connection/
        ├── ConnectionManager.vue  # 连接管理器主弹窗
        ├── ConnectionFormModal.vue# 新建/编辑连接
        └── AddGroupModal.vue      # 新增分组
```

---

## 二、拆分前后对比

| 指标 | 拆分前 | 拆分后 |
|------|--------|--------|
| App.vue 行数 | ~3700 | ~350 |
| 组件数量 | 1（+废弃 HelloWorld） | 12 |
| Composables | 0 | 6 |
| 可维护性 | 低 | 中高 |

---

## 三、数据流

```
App.vue
  ├── useConnections()     → ConnectionManager / QuickConnect / SessionTabBar
  ├── useTerminal()        → xterm 挂载 div
  ├── useFilePane()        → FilePane
  ├── useTransferTasks()   → TransferPanel
  ├── useServerTransfer()  → ServerTransferModal
  └── useSysInfo()         → SysInfoPanel

Wails EventsOn (App.vue onMounted)
  terminal-output    → useTerminal.writeToTerminal
  transfer-update    → useTransferTasks.upsertTransfer
  terminal-closed    → useTerminal.markSessionClosed
```

---

## 四、后续可继续拆分

1. `legacy-ui.css` 按组件拆成 scoped styles
2. `useConnections.ts` 可拆为 `useConnectionManager` + `useSessionTabs`
3. 命令面板 Tab（批量命令）独立 `CmdPanel.vue`

---

*详见各 composable / 组件文件头注释。*
