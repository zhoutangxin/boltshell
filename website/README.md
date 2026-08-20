# BoltShell 官网（静态单页）

`boltshell.com` 官网骨架，与 Wails 客户端分离部署。

## 目录

```
website/
├── index.html          # 单页：Hero / 特性 / 下载 / Pro 定价
├── css/style.css
├── assets/logo-icon.png
└── config/sponsors.json   # 赞助配置模板；远程 fallback 时部署到 /config/sponsors.json
```

## 本地预览

```powershell
# 方式 1：Python
cd website
python -m http.server 8080
# 打开 http://localhost:8080

# 方式 2：VS Code Live Server 打开 index.html
```

## Nginx 部署（推荐）

**只需上传 `website/` 目录里的静态文件，不是整个仓库。**

### 需要上传的文件

```
/var/www/boltshell/website/          ← 服务器上的站点根目录
├── index.html
├── css/style.css
├── assets/logo-icon.png
└── config/
    ├── sponsors.json                  # 赞助位（客户端拉取）
    └── release.json                   # 版本升级（客户端拉取）
```

`website/README.md` 不必上传。

### 上传示例（SCP）

```bash
# 在本地项目根目录执行
scp -r website/* user@your-server:/var/www/boltshell/
```

### Nginx 配置示例

```nginx
server {
    listen 80;
    server_name boltshell.com www.boltshell.com;

    root /var/www/boltshell;
    index index.html;

    # 单页 + 静态资源
    location / {
        try_files $uri $uri/ /index.html;
    }

    # 赞助配置 JSON（客户端拉取，建议允许跨域）
    location /config/sponsors.json {
        default_type application/json;
        add_header Cache-Control "public, max-age=300";
        add_header Access-Control-Allow-Origin "*";
    }

    # 静态资源缓存
    location ~* \.(css|png|ico|jpg|webp)$ {
        expires 7d;
        add_header Cache-Control "public, immutable";
    }
}
```

启用 HTTPS（Let's Encrypt）：

```bash
sudo certbot --nginx -d boltshell.com -d www.boltshell.com
```

### 验证

```bash
curl -I https://boltshell.com/
curl https://boltshell.com/config/sponsors.json
```

## 部署（其他方式）

将整个 `website/` 目录内容上传到：

- Gitee Pages / GitHub Pages
- 阿里云 OSS + CDN
- `boltshell.com` 根目录

确保以下 URL 可访问：

| URL | 文件 |
|-----|------|
| `https://boltshell.com/` | index.html |
| `https://boltshell.com/config/sponsors.json` | config/sponsors.json |

## 与客户端关系

- 远程 URL 在 `config/sponsors.remote.json`（开发者配置，随 `config/` 打包，**不是用户 config.json**）
- 客户端优先拉取该 URL（支持 `http://IP/...`）；断网失败时 fallback 本地默认
- 改赞助内容：编辑 `website/config/sponsors.json` 部署到服务器；改 URL 后重新打包客户端

## 待办

- [ ] 购买域名并绑定
- [x] Release 下载链接指向 GitHub Releases（Windows / macOS / Linux 三平台）
- [ ] Pro 购买页 / 支付接入
- [ ] install.sh 安装脚本

> 发新版后需同步更新 `index.html` 下载区的版本号与三个 Release 资源链接。
