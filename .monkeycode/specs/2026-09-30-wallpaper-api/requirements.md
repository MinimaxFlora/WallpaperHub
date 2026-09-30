# Requirements Document

## Introduction

本功能提供一个可自托管的壁纸 HTTP 服务（Go 实现），对外暴露一个仿 Bing 图片接口
（`HPImageArchive.aspx`）。客户端（典型场景：OpenWrt Argon 主题的"必应每日壁纸"配置）
只需把接口地址指向本服务，即可用自有图库替换 Bing 官方图源。图片存放在代码仓库的
`images/` 目录中，按服务器日期确定性轮换，接口响应结构与 Bing 兼容，因此无需修改客户端解析逻辑。

## Glossary

- **Wallpaper Service（服务）**: 本功能实现的 Go HTTP 服务，提供图片列表接口与图片文件分发。
- **Image（图片）**: 位于 `images/` 目录（含子目录）下、扩展名属于受支持集合的图片文件。
- **Image Index（图片索引）**: 服务从图片目录扫描得到的、按稳定规则排序的图片列表。
- **Bing 兼容接口**: 路径为 `/HPImageArchive.aspx`、响应体形如 `{"images":[...]}` 的接口。
- **Target Date（目标日期）**: 由请求日期与 `idx` 计算得到的、用于选图的日期。
- **Base URL**: 用于把图片相对路径拼装成绝对 URL 的服务对外地址。
- **Argon**: OpenWrt LuCI 的第三方主题 `luci-theme-argon`，其壁纸配置会读取接口返回的 `images[0].url`。
- **IP 模式**: 未配置域名时的默认运行模式，服务以纯 HTTP 提供接口。
- **域名模式**: 配置了域名后的运行模式，服务通过 ACME 自动获取证书并以 HTTPS 提供接口。
- **ACME**: 自动证书管理协议，服务通过 ACME 向 Let's Encrypt 申请与续期证书。
- **证书缓存**: 存放已签发证书与 ACME 账户信息的目录，供重启后复用。
- **Image Source（图片来源）**: 图片索引与图片字节的提供者，可选本地目录（`local`）或 GitHub 仓库（`github`）。
- **本地源**: 从服务器本地目录递归扫描图片并直接分发的 Image Source。
- **GitHub 源**: 从 GitHub 仓库读取图片清单、并由本服务代理转发图片字节的 Image Source。

## Requirements

### Requirement 1: Bing 兼容的图片列表接口

**User Story:** AS 一个 Web 客户端（如 Argon 主题），I want 用一个 Bing 兼容的接口获取壁纸列表，
so that 我可以只替换接口地址就使用自有壁纸。

#### Acceptance Criteria

1. WHEN 客户端以 `GET` 请求 `/HPImageArchive.aspx`，服务 SHALL 返回 HTTP 200 且 `Content-Type` 为 `application/json`。
2. WHEN 客户端提供 `format` 参数且值为 `js` 或 `json`，服务 SHALL 返回 JSON 响应体。
3. WHEN 客户端未提供 `format` 参数，服务 SHALL 按 `format=js` 处理。
4. The 响应体 SHALL 为顶层包含 `images` 数组的 JSON 对象。
5. The `images` 数组的每个元素 SHALL 包含字段 `url`、`urlbase`、`copyright`、`copyrightlink`、`title`、`startdate`、`fullstartdate`、`enddate`、`wp`、`hsh`、`drk`、`top`、`bot`、`quiz`。
6. The `images[i].url` SHALL 为一个可直接加载的绝对图片 URL，格式为 `<Base URL>/images/<相对路径>`。
7. The `images[i].startdate` SHALL 为 `YYYYMMDD` 格式的目标日期。
8. The `images[i].fullstartdate` SHALL 为 `YYYYMMDDHHMM` 格式的目标日期时间（时分固定为 `0000`）。
9. The `images[i].enddate` SHALL 为 `YYYYMMDD` 格式的目标日期。
10. The `images[i].copyrightlink` SHALL 为 `images[i].url` 对应的图片详情页或图片自身 URL。
11. The `images[i].hsh` SHALL 为对图片相对路径计算的确定性摘要字符串，同一次请求与重复请求结果 SHALL 一致。
12. The 服务 SHALL 支持跨域访问，对接口响应附带 `Access-Control-Allow-Origin: *` 响应头。

### Requirement 2: 请求参数与默认行为

**User Story:** AS 一个壁纸客户端，I want 通过参数控制取图和取图数量，
so that 我可以获取今天或历史某天的壁纸。

#### Acceptance Criteria

1. WHEN 客户端提供 `idx` 参数，服务 SHALL 以 `idx` 作为相对今天向前的天数偏移（`idx=0` 为今天，`idx=1` 为昨天）。
2. WHEN 客户端未提供 `idx` 参数，服务 SHALL 以 `idx=0` 处理。
3. WHEN 客户端提供 `n` 参数，服务 SHALL 返回数量不超过 `n` 的图片元素，且这些元素按目标日期从近到远排列。
4. WHEN 客户端未提供 `n` 参数，服务 SHALL 以 `n=1` 处理。
5. IF `idx` 或 `n` 的取值不是非负整数，THEN 服务 SHALL 使用对应参数的默认值继续处理，并正常返回结果。
6. IF 请求要求返回的图片数量超过图片总数，THEN 服务 SHALL 返回不超过图片总数的图片元素。
7. WHEN 客户端提供 `mkt` 参数，服务 SHALL 接受该参数并忽略其对结果的影响。

### Requirement 3: 按日期确定性选图

**User Story:** AS 一个壁纸客户端，I want 同一天的请求返回同一张图，
so that 结果稳定、可缓存、可复现。

#### Acceptance Criteria

1. The 服务 SHALL 在每次处理接口请求时，基于当前日期与 `idx` 计算目标日期。
2. The 服务 SHALL 以"自 Unix 纪元起的天数序号对图片总数取模"的规则，由目标日期确定选中图片在图片索引中的位置。
3. WHILE 图片索引内容与目标日期均未改变，服务 SHALL 对相同参数返回相同的图片。
4. WHEN `n` 大于 1，服务 SHALL 对 `idx` 至 `idx+n-1` 的每个偏移分别按第 2 条规则独立选图。
5. WHILE 图片索引非空，服务 SHALL 保证取模结果落在图片索引的有效下标范围内。

### Requirement 4: 图片索引的构建与刷新

**User Story:** AS 一个运维者，I want 往仓库丢图就能生效，
so that 我不需要重新构建镜像即可更新图库。

#### Acceptance Criteria

1. WHEN 服务启动，服务 SHALL 递归扫描图片目录并构建图片索引。
2. The 图片索引 SHALL 仅包含扩展名属于受支持集合（`jpg`、`jpeg`、`png`、`webp`、`gif`、`bmp`、`avif`，大小写不敏感）的文件。
3. The 图片索引 SHALL 按图片相对路径的字典序稳定排序。
4. The 服务 SHALL 每隔一个可配置的时间间隔重新扫描图片目录并刷新图片索引。
5. IF 图片目录不存在或图片索引为空，THEN 服务 SHALL 正常启动，并对接口请求返回 `images` 为空数组的合法 JSON 响应。

### Requirement 5: 图片文件分发

**User Story:** AS 一个 Web 客户端，I want 通过接口返回的 URL 直接加载图片字节流，
so that 背景图可以正常显示。

#### Acceptance Criteria

1. WHEN 客户端以 `GET` 请求 `/images/<相对路径>`，服务 SHALL 返回该图片文件的字节流。
2. WHEN 服务返回图片文件，服务 SHALL 附带与图片类型匹配的 `Content-Type` 响应头。
3. The 图片响应 SHALL 附带 `Access-Control-Allow-Origin: *` 响应头。
4. The 图片响应 SHALL 支持条件请求，并在资源未变化时返回 HTTP 304。
5. WHEN 客户端请求的图片路径包含向上跳转（`..`）或指向图片目录之外，服务 SHALL 返回 HTTP 404。
6. IF 客户端请求的图片不存在，THEN 服务 SHALL 返回 HTTP 404。

### Requirement 6: 运行配置

**User Story:** AS 一个运维者，I want 通过环境变量配置服务，
so that 我无需改动代码即可适配不同部署环境。

#### Acceptance Criteria

1. The 服务 SHALL 支持通过环境变量配置监听端口、图片目录路径、Base URL、版权文案、时区与图片重扫间隔。
2. The 服务 SHALL 支持通过环境变量配置日志级别。
3. WHEN 某项配置未提供，服务 SHALL 使用其默认值。
4. WHERE Base URL 未被显式配置，服务 SHALL 依据请求的 `Host` 头与请求协议推导 Base URL。

### Requirement 7: 图片元数据生成

**User Story:** AS 一个壁纸客户端，I want 接口返回的标题与版权字段有可读语义，
so that 界面上展示的信息有意义。

#### Acceptance Criteria

1. The `images[i].title` SHALL 为图片文件去除扩展名后的文件名。
2. The `images[i].copyright` SHALL 由图片标题与可配置的版权文案拼接而成。
3. The `images[i].wp` SHALL 为布尔值 `true`。
4. The `images[i].quiz` SHALL 为空字符串。
5. The `images[i].drk`、`images[i].top`、`images[i].bot` SHALL 为整数 `0`。

### Requirement 8: 可观测与健康检查

**User Story:** AS 一个运维者，I want 探测服务状态与查看日志，
so that 我可以判断服务是否正常。

#### Acceptance Criteria

1. WHEN 客户端以 `GET` 请求 `/healthz`，服务 SHALL 返回 HTTP 200。
2. The 服务 SHALL 在处理接口请求时记录包含请求路径与响应状态码的访问日志。
3. IF 服务处理请求时发生内部错误，THEN 服务 SHALL 返回 HTTP 500 并记录错误日志。

### Requirement 9: 容器化部署

**User Story:** AS 一个运维者，I want 用 Docker 部署服务，
so that 我可以在服务器或 NAS 上一键运行。

#### Acceptance Criteria

1. The 服务 SHALL 提供一个多阶段构建的 `Dockerfile`，最终镜像仅包含服务二进制与运行时所需的证书。
2. The 构建产物 SHALL 为单个静态链接的可执行文件，无需外部运行时依赖。
3. The 部署 SHALL 支持通过只读卷把宿主机图片目录挂载到容器内图片目录。
4. The 服务 SHALL 提供 `docker-compose` 示例，包含端口映射、图片目录挂载与环境变量配置。
5. The 镜像 SHALL 在挂载图片目录为空时仍能正常启动。

### Requirement 10: 域名模式与自动证书

**User Story:** AS 一个运维者，I want 配置域名后自动获得 HTTPS，
so that 我不需要手工申请和续期证书。

#### Acceptance Criteria

1. WHEN 域名配置项为空，服务 SHALL 以纯 HTTP 在 HTTP 监听地址提供服务。
2. WHEN 域名配置项包含一个或多个域名，服务 SHALL 启用域名模式，并通过 ACME 为这些域名获取与续期证书。
3. WHILE 处于域名模式，服务 SHALL 在 HTTP 监听地址响应 ACME 校验请求，并把其余请求以 HTTP 301 跳转到对应的 HTTPS 地址。
4. WHILE 处于域名模式，服务 SHALL 仅接受已配置域名，并在 TLS 握手阶段拒绝对未配置域名的访问。
5. WHILE 处于域名模式且 Base URL 未被显式配置，服务 SHALL 以 `https://<首个域名>` 作为 Base URL。
6. The 服务 SHALL 将证书与 ACME 账户信息写入可配置的证书缓存目录，并在重启后优先复用已存在的证书。
7. WHERE 测试环境开关被启用，服务 SHALL 使用 ACME 测试目录签发证书。
8. The 域名配置项 SHALL 接受逗号分隔的多个域名，并忽略各域名的大小写与协议前缀差异。
9. The 服务 SHALL 在启动日志中记录当前运行模式与已配置域名。

### Requirement 11: GitHub 图片源

**User Story:** AS 一个运维者，I want 把图片放在 GitHub 仓库并让服务代为转发，
so that 服务器不占磁盘，加图只需推送文件。

#### Acceptance Criteria

1. The 服务 SHALL 支持通过环境变量选择图片来源，取值为 `local` 或 `github`。
2. WHEN 仅配置了 GitHub 仓库而未显式指定来源，服务 SHALL 使用 `github` 来源。
3. WHEN 来源为 `github` 而未配置仓库，服务 SHALL 在启动时报告配置错误并退出。
4. WHILE 来源为 `github`，服务 SHALL 通过 GitHub 树接口按配置的仓库、引用与目录构建图片索引。
5. The GitHub 图片索引 SHALL 仅包含扩展名属于受支持集合、且位于配置目录之下的文件。
6. The 服务 SHALL 按可配置的间隔重新拉取 GitHub 索引；IF 刷新失败，THEN 服务 SHALL 保留上一次成功的索引并记录错误日志。
7. WHEN 客户端请求 `/images/<相对路径>` 且该路径存在于当前索引，服务 SHALL 从 GitHub 获取图片字节并转发给客户端。
8. WHEN 客户端请求的图片路径不在当前索引中，服务 SHALL 返回 HTTP 404，服务 SHALL NOT 充当开放代理。
9. The 服务 SHALL 向 GitHub 源转发请求时透传条件请求与 `Range` 头，并透传上游的 `Content-Type` 与缓存相关响应头。
10. The 服务 SHALL 支持通过环境变量配置 GitHub API 基地址、原始文件基地址与访问令牌。
