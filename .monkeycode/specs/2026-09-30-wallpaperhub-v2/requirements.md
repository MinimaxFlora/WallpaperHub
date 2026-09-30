# Requirements Document

## Introduction

WallpaperHub v2 是一个自托管的公开壁纸分发服务。图片存放在 GitHub 仓库中，推送后由
GitHub Actions 生成清单并提交回仓库；服务读取清单，从 GitHub 拉取图片字节并写入本地磁盘缓存，
向任意前端（Argon 主题、OpenList、桌面壁纸程序、网页）提供统一的检索与取图接口。

与 v1 的差异：

- 接口从「仿必应专有格式」改为原生 REST 接口，不再做必应兼容路由。
- 取图规则从「每天固定一张」扩展为随机、种子、按日、会话四种模式，可用参数切换。
- 图片带标签与分类，支持筛选与检索。
- 清单由 GitHub Actions 生成并提交，服务不再在运行时扫描目录推算元数据。
- 部署形态为 Docker Compose，可选内置 ACME 自动签发 HTTPS，不依赖 Cloudflare。

## Glossary

- **Service（服务）**: 运行在自托管主机上的 Go HTTP 服务，本系统的主体。
- **Manifest（清单）**: 描述全部图片及其元数据的 JSON 文档，提交在仓库根目录。
- **Manifest Generator（清单生成器）**: 计算宽高、字节数与内容摘要并输出清单的命令。
- **Image Cache（图片缓存）**: 服务在本地磁盘保存的图片字节副本。
- **Image（图片）**: 一张受支持的壁纸文件，具有唯一 id 与一组元数据。
- **Metadata Source（元数据源）**: 仓库中人工维护的元数据文件，为图片补充标题、标签与分类。
- **Tag（标签）**: 描述图片内容的自由文本关键词，一张图片可带多个。
- **Category（分类）**: 图片所属的单一归类，如 `anime`、`landscape`。
- **Selection Mode（取图模式）**: 决定返回哪一张图片的规则，取值为 `random`、`seed`、`daily`、`session`。
- **Seed（种子）**: 使取图结果可复现的输入字符串。
- **Session（会话）**: 由请求方标识推导出的一段时间内稳定的客户端身份。
- **Hotlink Protection（防盗链）**: 依据请求来源决定是否允许获取图片字节的机制。
- **Rate Limit（限流）**: 按客户端标识限制单位时间请求数量的机制。

## Requirements

### Requirement 1: 清单与元数据

**User Story:** AS 图库维护者，I want 每张图片都有完整元数据，so that 客户端能按条件检索并展示有意义的信息。

#### Acceptance Criteria

1. The 清单 SHALL 为每张图片记录唯一 id、文件路径、标题、标签集合、分类、宽度、高度、字节数、格式、方向与内容摘要。
2. WHEN 清单生成器处理一张图片，清单生成器 SHALL 计算该图片的宽度、高度、字节数与内容摘要。
3. IF 元数据源中缺少某张图片的条目，THEN 清单生成器 SHALL 以文件名去除扩展名后作为标题、以图片所在子目录名作为分类、以空集合作为标签。
4. WHERE 图片直接位于图片目录根层，清单生成器 SHALL 以 `uncategorized` 作为分类。
5. The 清单 SHALL 以 JSON 文档形式提交在仓库中，并包含清单版本号与生成时间。
6. WHEN 清单内容未发生变化，清单生成器 SHALL 保持输出文件不变。
7. WHEN 清单在仓库中更新，服务 SHALL 在一个刷新周期内使用新版本清单。

### Requirement 2: 取图模式

**User Story:** AS 一个前端，I want 用参数选择取图规则，so that 不同场景都能拿到合适的图片。

#### Acceptance Criteria

1. WHEN 请求未指定 `mode`，服务 SHALL 以 `random` 模式返回一张图片。
2. WHILE `mode=random`，服务 SHALL 在满足筛选条件的图片集合中等概率选择一张。
3. WHEN 请求携带 `seed`，服务 SHALL 在筛选条件与 `seed` 均相同时返回同一张图片。
4. WHILE `mode=daily`，服务 SHALL 依据配置时区的当天日期确定返回的图片。
5. WHEN `mode=daily` 且请求携带 `date`，服务 SHALL 返回该日期对应的图片。
6. WHILE `mode=session`，服务 SHALL 在同一会话标识下返回同一张图片。
7. WHEN `mode=session` 且请求未携带会话标识，服务 SHALL 下发会话 Cookie。
8. IF 筛选条件没有任何匹配图片，THEN 服务 SHALL 返回 HTTP 404 并附结构化错误信息。
9. IF 请求的 `mode` 取值不在受支持集合内，THEN 服务 SHALL 以 `random` 模式处理。
10. IF 请求的 `date` 格式非法，THEN 服务 SHALL 返回 HTTP 400 并指出期望格式。

### Requirement 3: 检索与筛选

**User Story:** AS 一个前端，I want 按标签、分类与尺寸筛选，so that 我能请求到符合场景的壁纸。

#### Acceptance Criteria

1. WHEN 请求携带 `tags`，服务 SHALL 仅返回包含全部指定标签的图片。
2. WHEN 请求携带 `category`，服务 SHALL 仅返回该分类下的图片。
3. WHEN 请求携带 `min_width` 或 `min_height`，服务 SHALL 仅返回不小于对应尺寸下限的图片。
4. WHEN 请求携带 `orientation`，服务 SHALL 仅返回方向匹配的图片，取值为 `landscape`、`portrait` 或 `square`。
5. WHEN 请求列表接口，服务 SHALL 返回分页结果、当前页号、每页数量与匹配总数。
6. WHEN 请求标签枚举接口，服务 SHALL 返回全部标签及其图片数量。
7. WHEN 请求分类枚举接口，服务 SHALL 返回全部分类及其图片数量。

### Requirement 4: 图片字节分发

**User Story:** AS 一个前端，I want 通过稳定地址直接加载图片，so that 背景图能正常显示。

#### Acceptance Criteria

1. WHEN 客户端请求某 id 的图片字节，服务 SHALL 返回该图片内容。
2. WHERE 图片源为 GitHub，服务 SHALL 在首次请求时下载图片并写入本地缓存，后续请求直接读缓存。
3. The 图片响应 SHALL 携带与格式匹配的 `Content-Type`、内容摘要 `ETag` 与长有效期缓存响应头。
4. The 图片响应 SHALL 支持条件请求与 `Range` 请求。
5. WHEN 客户端请求元数据，服务 SHALL 返回描述该图片的 JSON 文档。
6. WHEN 客户端请求携带 `redirect=1`，服务 SHALL 返回 HTTP 302 并指向图片字节地址。
7. IF 请求的 id 不存在于清单，THEN 服务 SHALL 返回 HTTP 404。
8. IF 清单尚未加载，THEN 服务 SHALL 返回 HTTP 503 并附结构化错误信息。

### Requirement 5: 清单生成流水线

**User Story:** AS 图库维护者，I want 推送图片后自动生效，so that 加图无需登录服务器或改动代码。

#### Acceptance Criteria

1. WHEN 仓库中的图片或元数据文件发生变更，清单生成流水线 SHALL 重新生成清单并提交回仓库。
2. The 清单生成流水线 SHALL 依据受支持扩展名集合过滤待处理文件。
3. IF 出现重复的图片 id，THEN 清单生成流水线 SHALL 校验失败并输出错误信息。
4. WHEN 清单内容未变化，清单生成流水线 SHALL 不产生提交。
5. The 清单生成流水线 SHALL 在无需修改服务代码的前提下完成更新。
6. WHEN 清单生成流水线完成提交，服务 SHALL 在下一个刷新周期内对外提供新清单。

### Requirement 6: 访问控制与滥用防护

**User Story:** AS 服务运营者，I want 限制滥用与盗链，so that 公开服务在被大量使用时保持可用。

#### Acceptance Criteria

1. The 服务 SHALL 按客户端标识在时间窗口内限制请求数量。
2. IF 某客户端在窗口内超过请求上限，THEN 服务 SHALL 返回 HTTP 429 并携带 `Retry-After` 响应头。
3. WHERE 防盗链白名单被配置，服务 SHALL 仅对来自白名单来源的取图请求返回图片字节。
4. IF 取图请求来源不在白名单内，THEN 服务 SHALL 返回 HTTP 403。
5. WHERE 防盗链白名单为空，或请求不带来源信息，服务 SHALL 允许获取图片字节。
6. The 服务 SHALL 对不存在的 id 与超限请求返回结构化错误信息，且不暴露内部实现细节。

### Requirement 7: 可观测与健康检查

**User Story:** AS 服务运营者，I want 查看服务状态与日志，so that 我能判断服务是否正常。

#### Acceptance Criteria

1. WHEN 客户端请求 `/healthz`，服务 SHALL 返回 HTTP 200 与清单图片总数、清单生成时间。
2. The 服务 SHALL 为每个请求记录包含路径、状态码、耗时、客户端标识与缓存命中状态的日志。
3. IF 服务处理请求发生内部错误，THEN 服务 SHALL 返回 HTTP 500 并记录错误日志。

### Requirement 8: 部署与配置

**User Story:** AS 服务运营者，I want 用配置适配不同环境，so that 同一套代码可用于线上与本地。

#### Acceptance Criteria

1. The 服务 SHALL 支持通过 Docker 与 Docker Compose 部署。
2. WHERE 配置了域名，服务 SHALL 自动申请并续期 HTTPS 证书。
3. WHERE 未配置域名，服务 SHALL 以纯 HTTP 模式运行。
4. The 服务 SHALL 通过环境变量声明图片源、仓库、清单位置、缓存目录、公开域名、时区、防盗链白名单与限流参数。
5. WHERE 某项配置未提供，服务 SHALL 使用安全默认值。
6. The 服务 SHALL 支持通过环境变量调整限流阈值与窗口长度。
