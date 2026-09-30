# wallpaper-api

一个用 Go 1.27 写的自托管壁纸服务，对外提供仿 Bing `HPImageArchive.aspx` 的接口。
把 OpenWrt Argon 主题的壁纸地址改成这个服务，就能用你自己的图库替换 Bing 官方图源。

图片可以来自两个数据源，由 `WALLPAPER_SOURCE` 选择：

- `github`（推荐）：图片存放在 GitHub 仓库的 `images/` 目录，服务通过 GitHub 树接口建立索引，
  再把原始图片字节转发给客户端。客户端只连这台服务器，不占服务器磁盘，加图只需推送文件，无需重启。
- `local`（默认）：图片放在本地目录，递归扫描后直接分发，适合离线或内网自持场景。

无论哪种数据源，选图都按服务器日期确定性轮换：同一天、同样参数的请求永远返回同一张图，
结果稳定、可缓存、可复现。

两种运行模式：

- 默认（不设域名）：纯 HTTP，用 `IP:8080` 访问。
- 设置域名后：自动用 Let's Encrypt 申请并续期证书，直接以 `https://域名` 提供服务，HTTP 自动跳转 HTTPS。

## 接口

| 方法与路径 | 说明 |
|------------|------|
| `GET /HPImageArchive.aspx?format=js&idx=0&n=1&mkt=zh-CN` | Bing 兼容的图片列表接口 |
| `GET /images/<相对路径>` | 图片文件分发 |
| `GET /healthz` | 健康检查 |

参数：

- `idx`：相对今天向前的天数偏移，`0` 为今天，`1` 为昨天。默认 `0`。
- `n`：返回的图片数量。默认 `1`，上限为图片总数。
- `format`：`js` 或 `json`，两者等价。其他值也返回 JSON。
- `mkt`：接受但忽略。

响应示例：

```json
{
  "images": [
    {
      "url": "http://localhost:8080/images/landscapes/lake.jpg",
      "urlbase": "http://localhost:8080/images/landscapes/lake",
      "copyright": "lake (© My Wallpapers)",
      "copyrightlink": "http://localhost:8080/images/landscapes/lake.jpg",
      "title": "lake",
      "startdate": "20260930",
      "fullstartdate": "202609300000",
      "enddate": "20260930",
      "wp": true,
      "hsh": "0d4e2a4d3e3c1b90",
      "drk": 0,
      "top": 0,
      "bot": 0,
      "quiz": ""
    }
  ]
}
```

## 选图规则

图片索引按相对路径的字典序稳定排序。目标日期 = 今天 - `idx` 天，
选中下标 = `((天数序号 % 图片总数) + 图片总数) % 图片总数`。
`idx` 每加 1 取上一张（循环），因此同一天结果固定，新增图片会改变后续轮换结果。

支持的图片扩展名：`jpg`、`jpeg`、`png`、`webp`、`gif`、`bmp`、`avif`（大小写不敏感）。
目录递归扫描，子目录会体现在图片 URL 里。

`title` 取文件名去扩展名，`copyright` 为 `标题 (© 配置的版权文案)`。

建议图片按 `01`、`02`、`03` 这样的两位序号命名（如 `01.webp`、`02.webp`），并按加入顺序递增，
这样字典序即加入顺序，选图轮换的顺序稳定可预期。

## 配置

全部通过环境变量配置，均有默认值：

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `WALLPAPER_ADDR` | `:8080` | HTTP 监听地址 |
| `WALLPAPER_SOURCE` | `local` | 数据源：`github` 或 `local`。设置 `WALLPAPER_GITHUB_REPO` 时会自动切换为 `github` |
| `WALLPAPER_IMAGES_DIR` | `./images` | 本地图片目录，仅 `local` 模式使用 |
| `WALLPAPER_BASE_URL` | 空 | 图片绝对 URL 前缀；为空时按请求 `Host` 与 `X-Forwarded-Proto` 推导 |
| `WALLPAPER_COPYRIGHT` | `Wallpaper Collection` | 版权文案 |
| `WALLPAPER_TIMEZONE` | `Local` | 计算"今天"所用时区，如 `Asia/Shanghai` |
| `WALLPAPER_RESCAN_INTERVAL` | `60s` | 本地图片目录重扫间隔，仅 `local` 模式使用，`0` 表示关闭 |
| `WALLPAPER_LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error` |
| `WALLPAPER_DOMAIN` | 空 | 域名，可逗号分隔多个。设置后启用 HTTPS 自动证书；为空则是 HTTP/IP 模式 |
| `WALLPAPER_ACME_EMAIL` | 空 | ACME 账户邮箱，建议填写以便接收证书到期通知 |
| `WALLPAPER_ACME_CACHE_DIR` | `./certs` | 证书缓存目录，必须可写且持久化 |
| `WALLPAPER_ACME_STAGING` | `false` | `true` 时使用 Let's Encrypt 测试环境，避免调试时触发速率限制 |
| `WALLPAPER_HTTP_ADDR` | `:80` | 仅在域名模式下使用，负责 ACME 校验与跳转 HTTPS |
| `WALLPAPER_HTTPS_ADDR` | `:443` | 仅在域名模式下使用，提供 HTTPS 服务 |
| `WALLPAPER_GITHUB_REPO` | 空 | GitHub 仓库，格式 `owner/name`，如 `MinimaxFlora/WallpaperHub` |
| `WALLPAPER_GITHUB_REF` | `master` | 分支、标签或提交 |
| `WALLPAPER_GITHUB_PATH` | `images` | 仓库内存放壁纸的目录 |
| `WALLPAPER_GITHUB_TOKEN` | 空 | 可选。GitHub Token，用于提高 API 速率限制或访问私有仓库 |
| `WALLPAPER_GITHUB_REFRESH_INTERVAL` | `15m` | 索引刷新间隔，`0` 表示关闭定时刷新 |
| `WALLPAPER_GITHUB_API_BASE` | `https://api.github.com` | GitHub API 地址，自建或镜像时改这里 |
| `WALLPAPER_GITHUB_RAW_BASE` | `https://raw.githubusercontent.com` | 原始文件地址，自建或镜像时改这里 |

### GitHub 数据源

`WALLPAPER_SOURCE=github` 时，服务按 `WALLPAPER_GITHUB_REFRESH_INTERVAL` 调用 GitHub 树接口
（`GET /repos/<repo>/git/trees/<ref>?recursive=1`）建立一次索引，然后按需把图片字节从
`<raw_base>/<repo>/<ref>/<path>/<文件>` 取回并转发。要点：

- 只有出现在索引里的路径才会被转发，`/images/` 不能当开放代理使用。
- 条件请求（`If-None-Match`、`If-Modified-Since`）和 `Range` 会透传，客户端可缓存、可断点续传。
- 未认证的 GitHub API 每个来源每小时限 60 次；15 分钟一次的刷新远低于上限，无需 Token。
  若提高刷新频率或访问私有仓库，请设置 `WALLPAPER_GITHUB_TOKEN`。
- 新增图片后，索引会在一个刷新周期内自动更新，无需重启服务。

## 本地运行

```bash
go run .
```

默认读取 `./images`（`local` 模式），监听 `:8080`。验证：

```bash
curl -s "http://localhost:8080/HPImageArchive.aspx?format=js&idx=0&n=1"
```

改用 GitHub 数据源：

```bash
WALLPAPER_SOURCE=github \
WALLPAPER_GITHUB_REPO=MinimaxFlora/WallpaperHub \
WALLPAPER_GITHUB_REF=master \
WALLPAPER_GITHUB_PATH=images \
go run .
```

## Docker 部署

镜像只含静态二进制，图片放在 GitHub 仓库里，因此镜像体积与图片数量无关，加图只需推送文件。
`docker-compose.yml` 默认使用 GitHub 数据源：

```bash
docker compose up -d --build
```

或直接构建运行：

```bash
docker build -t wallpaper-api:latest .
docker run -d --name wallpaper-api -p 8080:8080 \
  -e WALLPAPER_SOURCE=github \
  -e WALLPAPER_GITHUB_REPO=MinimaxFlora/WallpaperHub \
  wallpaper-api:latest
```

容器以非 root 用户运行。若改用 `local` 模式，请把图片目录只读挂载进容器并设置 `WALLPAPER_IMAGES_DIR`。

## HTTPS 自动证书（域名模式）

在 `docker-compose.yml` 里填入域名后，服务会自动向 Let's Encrypt 申请证书，之后以 HTTPS 提供接口，
HTTP 请求一律 301 跳转到 HTTPS：

```yaml
    environment:
      WALLPAPER_DOMAIN: wall.example.com
      WALLPAPER_ACME_EMAIL: you@example.com
```

证书存放在 `certs` 卷里，重启不会重复申请。`WALLPAPER_BASE_URL` 未显式设置时会自动取 `https://<首个域名>`。

前提条件：

1. 域名的 DNS A/AAAA 记录指向这台服务器。
2. 服务器的 80 和 443 端口对公网开放（Let's Encrypt 需要回调校验）。
3. 证书缓存目录可写并持久化，否则每次重启都会重新申请，容易触发速率限制。

调试阶段建议先设 `WALLPAPER_ACME_STAGING: "true"` 使用测试环境，签发的是不受浏览器信任的测试证书；
确认流程无误后改回 `false` 获取正式证书。此时浏览器也可用 `curl -k` 忽略证书告警验证。

注意：Let's Encrypt 不支持纯 IP 地址和单段主机名（如 `localhost`）证书，因此域名模式必须填写形如
`wall.example.com` 的域名。

## 接入 Argon 主题

在 Argon 主题设置里把必应壁纸接口地址（默认是 `https://www.bing.com/HPImageArchive.aspx`）改为：

```
http://<你的服务器地址>:8080/HPImageArchive.aspx
```

Argon 会把接口返回的 `images[0].url` 直接当作背景图加载，因此 `url` 必须是客户端能访问到的绝对地址。
如果服务在反向代理后面，建议显式设置 `WALLPAPER_BASE_URL`，例如：

```bash
WALLPAPER_BASE_URL=https://wall.example.com docker compose up -d
```

## 测试

```bash
go test ./...
```

## 项目结构

```
main.go                      进程入口、优雅退出、定时重扫
internal/acme                ACME 自动证书（autocert 封装）
internal/config              环境变量配置
internal/index               图片来源接口、本地目录扫描、过滤、排序、原子快照
internal/github              GitHub 树接口索引与原始图片转发
internal/selector            日期到图片下标的确定性映射
internal/bing                Bing 兼容响应与字段组装
internal/server              路由、接口、图片分发、CORS、访问日志
images/                      壁纸目录（GitHub 数据源时即仓库内的 images/）
Dockerfile                   多阶段构建，产物为静态二进制
docker-compose.yml           部署示例
```

启动日志会打印当前模式（`mode=http(ip)` 或 `mode=https(acme)`），便于确认域名是否生效。
