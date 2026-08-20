# BoltShell 官网（静态单页）

`boltshell.com` 官网骨架，与 Wails 客户端分离部署。

## 目录

```
website/
├── index.html          # 单页：Hero / 特性 / 下载 / Pro 定价
├── css/style.css
├── assets/logo-icon.png
└── config/sponsors.json   # 客户端拉取的赞助配置（部署到 /config/sponsors.json）
```

## 本地预览

```powershell
# 方式 1：Python
cd website
python -m http.server 8080
# 打开 http://localhost:8080

# 方式 2：VS Code Live Server 打开 index.html
```

## 部署

将整个 `website/` 目录上传到：

- Gitee Pages / GitHub Pages
- 阿里云 OSS + CDN
- `boltshell.com` 根目录

确保以下 URL 可访问：

| URL | 文件 |
|-----|------|
| `https://boltshell.com/` | index.html |
| `https://boltshell.com/config/sponsors.json` | config/sponsors.json |

## 与客户端关系

- 桌面客户端默认拉取 `https://boltshell.com/config/sponsors.json`
- 改赞助内容：编辑 `config/sponsors.json` 后重新部署官网即可（客户端有 6h 缓存）

## 待办

- [ ] 购买域名并绑定
- [ ] Release 下载链接指向 Gitee/GitHub Releases
- [ ] Pro 购买页 / 支付接入
- [ ] install.sh 安装脚本
