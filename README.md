# 知微 · Folio

一个人的知识花园：把博客文章与知识库文档合并到同一个站点里，共用一套用户、互动与视觉系统。

这个项目由 `blogx_server` / `blogx_web`（博客）与 `gvd_server` / `gvd_web`（文档库）合并重写而来。原四个项目保持原样，仅作为参考。

## 核心变化

- **一张用户表、一张内容表**：所有内容都在 `documents` 表，用 `kind` 区分 `article`（文章）与 `doc`（文档）。文档用 `parent_id` 组成目录树，文章始终是平的。
- **不做封面上传**：封面与头像由 rough.js 在浏览器里实时绘制。封面是一张内容索引卡，标题、摘要、标签和分类参与排版，再按标签与标题选配手绘线稿；头像是一个手绘圆加昵称首字。同一篇内容永远得到同一张封面。服务端 `/api/art/*` 保留同风格的轻量 SVG/PNG 版本，供接口和分享图使用。
- **CSS 全部手写**：不引入 Arco 或其他组件库，也不使用预处理器。四个 CSS 文件构成设计系统，支持明暗主题。
- **三种数据模式**：SQLite（默认）、MySQL、进程内内存 Mock。Redis 可选，用于登出黑名单与浏览量去重。图片默认落本地磁盘，也可以切换到七牛云。
- **保留内容能力**：正文图片上传与图库、粘贴/拖拽插图、文章分类、文章推荐、收藏夹分组、浏览历史、评论、点赞、收藏、关注、私信、通知都已恢复。
- **邮箱能力**：配置 SMTP 后支持邮箱验证码注册、验证码登录、绑定/更换邮箱和自助找回密码。
- **AI 问答**：登录用户可以使用自己的 API Key 向站内文章、文档和站点帮助提问，可选 Anthropic、DeepSeek、智谱 GLM、Kimi、通义千问；支持分块检索、严格/通用模式、多轮追问和流式回答，回答会带回引用。本站不提供 Key；用户可选择将自己的 Key 加密保存，方便下次使用。
- **关于页与站点帮助**：`/about` 按场景介绍全部功能，目录同时作为 AI 的站点帮助知识库；新增功能只需补充一条目录记录。
- **站点设置与审计**：超级管理员可以在后台修改站点名称、描述、维护者和登录图形验证码；公开注册与 QQ 登录开关在「注册与邀请」；登录、内容、分类、图片、角色等关键动作都会写入操作日志。
- **隐私与消息偏好**：每个人都可以决定是否公开关注/粉丝列表、是否接收私信和点赞/评论通知，并选择公开主页的封面样式。
- **删繁就简**：去掉复杂的角色表、文档权限、文档密码与试看、审核流、Banner、Elasticsearch；只保留一个极简的三级角色字段。
- **保留基本互动**：评论（两级）、评论点赞、内容点赞、收藏、关注/粉丝、私信、站内通知、管理后台；每篇文章或文档都可以单独关闭评论。私信支持图片、表情和 WebSocket 实时推送。管理员可以发布全站公告。
- **个人站点权限**：只有超级管理员和管理员可以发布文章与文档；普通读者可以评论、点赞、收藏、关注和私信。超级管理员可以在管理后台把读者设为管理员。

## 三种运行模式

| 模式 | 数据库 | Redis | 适用场景 | 启动 |
| --- | --- | --- | --- | --- |
| SQLite | 文件 | 可选 | 日常开发、单机部署 | `./dev.sh` |
| MySQL + Redis | Docker 容器 | Docker 容器 | 接近生产的完整环境 | `docker compose up -d --build` 或 `./scripts/dev-mysql.sh` |
| Mock | 进程内内存 | 不需要 | 无数据库、快速体验、演示 | `./scripts/dev-mock.sh` |

Mock 模式每次启动都会重新写入演示数据，退出即清空，适合在没有 Docker、没有数据库的机器上直接看效果。

## 技术栈

| 层 | 选型 |
| --- | --- |
| 后端 | Go 1.24 · Gin · GORM |
| 数据库 | SQLite（默认）/ MySQL 8.4 |
| 缓存 | Redis 7（可选） |
| 图片存储 | 本地磁盘（默认）/ 七牛云对象存储（可选） |
| 登录 | 用户名密码 / 邮箱验证码 / 可选 QQ 登录；可选图形验证码；JWT（30 天）+ 可选 Redis 黑名单 |
| 前端 | Vue 3 · Vite · Vue Router |
| 手绘视觉 | rough.js（Canvas 封面/头像 + SVG 装饰线） |
| 内容渲染 | markdown-it + highlight.js（仅语法标记，主题 CSS 手写） |
| 实时消息 | WebSocket（gorilla/websocket）+ 10 秒轮询兜底 |
| SEO / 订阅 | RSS 2.0、sitemap.xml、robots.txt、服务端注入的 OG/JSON-LD 分享信息 |
| 样式 | 原生 CSS，明暗主题，响应式 |

## 目录结构

```
folio/
├── server/
│   ├── main.go             # 启动、静态资源托管、SPA 回退
│   ├── config/             # 环境变量配置
│   ├── db/                 # SQLite / MySQL / 内存数据库
│   ├── cache/              # Redis：JWT 黑名单、浏览量去重
│   ├── mail/               # SMTP 发信与本地调试邮件
│   ├── models/             # 25 张表的模型
│   ├── art/                # 生成式封面与头像
│   ├── storage/            # 本地磁盘 / 七牛云存储抽象
│   ├── handlers/           # 路由、接口与集成测试
│   └── seed/               # 演示数据
├── web/
│   ├── src/views/          # 15 个页面
│   ├── src/components/     # 卡片、评论、目录树等组件
│   ├── src/styles/         # base / components / pages / markdown
│   └── src/utils/          # API 客户端、Markdown、树结构、格式化
├── scripts/
│   ├── dev-mock.sh         # 内存 Mock 模式
│   ├── dev-mysql.sh        # 本地开发 + Docker MySQL/Redis
│   └── smoke.py            # 运行中实例的端到端冒烟测试
├── docker-compose.yml      # MySQL + Redis + 应用
├── Dockerfile              # 前端构建 + Go 编译 + 运行镜像
├── dev.sh                  # 一键启动前后端
├── TESTING.md              # 自测清单与故障排查
└── README.md
```

## 快速开始

### 1. 完全无依赖：Mock 模式

```bash
cd /Users/weijia/Desktop/blog/folio
./scripts/dev-mock.sh
```

- 前端：<http://localhost:5173>
- 后端：<http://localhost:8080>
- 数据只存在进程内存里，重启即恢复初始演示数据。

### 2. Docker 全栈（MySQL + Redis + 应用）

```bash
docker compose up -d --build
```

- 站点：<http://localhost:8080>（Go 直接托管构建好的前端）
- MySQL：`localhost:3306`，Redis：`localhost:6379`
- 应用容器内的环境变量已经配置好 MySQL 与 Redis。

只启动数据库，应用在本地跑：

```bash
docker compose up -d db redis
./scripts/dev-mysql.sh
```

### 3. SQLite 本地开发

```bash
./dev.sh
```

数据文件在 `server/data/folio.db`。脚本会自动读取项目根目录的 `.env`（如果存在）。

也可以分开启动：

```bash
cd server && go run .
```

```bash
cd web && npm install && npm run dev
```

### 端口与地址

- 开发模式前端：<http://localhost:5173>
- 后端 / Docker 全栈：<http://localhost:8080>

> 三种本地启动方式默认都占用 `8080`，与 Docker 全栈冲突。切换模式前先执行 `docker compose stop app`，或者修改 `FOLIO_ADDR` 并同步调整 `web/vite.config.js` 中的代理地址。

### 演示账号

| 用户名 | 密码 | 角色 |
| --- | --- | --- |
| `weijia` | `folio1234` | 超级管理员 |
| `lin` | `folio1234` | 管理员 |
| `chen` | `folio1234` | 读者 |
| `yue` | `folio1234` | 读者 |
| `zhou` | `folio1234` | 读者 |

演示数据包含 5 位用户、10 篇已发布文章、14 篇文档、2 篇草稿、5 个文章分类，以及分布在文章与文档上的点赞、收藏、评论、关注、私信、通知和浏览历史。只有当用户表为空时才会写入演示数据。第一位注册的用户会自动成为超级管理员。

## 角色与权限

| 能力 | 超级管理员 | 管理员 | 读者 |
| --- | --- | --- | --- |
| 阅读、评论、点赞、收藏、关注、私信 | ✓ | ✓ | ✓ |
| 使用自己的 API Key 进行 AI 问答 | ✓ | ✓ | ✓ |
| 上传图片（正文或私信） | ✓ | ✓ | ✓ |
| 发布、编辑、删除自己的内容 | ✓ | ✓ | — |
| 编辑、删除任何人的内容 | ✓ | — | — |
| 进入管理后台 | ✓ | ✓（只看自己的内容） | — |
| 删除用户、设置或取消管理员、创建用户、生成邀请码、修改站点设置、查看操作日志 | ✓ | — | — |

规则很简单：站点只有一个超级管理员，通常是第一位注册用户；超级管理员在 `/admin` 的「用户与权限」里把读者设为管理员，或把管理员降回读者。管理员降级后不能继续发布，但已经发布的内容和互动会保留。

关闭注册后，超级管理员仍然可以在「注册与邀请」里：

- 打开「允许公开注册」恢复公开注册，或保持关闭；
- 生成一次性邀请链接，朋友通过 `/register?invite=...` 创建普通读者账号；
- 邀请码可设置 1 / 7 / 30 / 90 天有效期，使用后立即失效，未使用的可以撤销；
- 「用户与权限」里仍然可以直接创建用户，并指定是读者还是管理员。

QQ 登录也由这里控制：没有配置 QQ 互联凭据时开关不可开启；配置后默认开启，可以随时关闭，关闭后登录页会隐藏 QQ 按钮。

管理后台入口在登录后的顶栏齿轮图标、用户下拉菜单「管理后台」，以及「我的空间」侧栏。

### 文档私密

文档可以在编辑器里勾选「私密文档」：

- 私密文档只有作者本人和超级管理员可以打开；其他读者在文章/文档列表、目录树、搜索、收藏、sitemap 和 AI 检索中都看不到；
- 父级文档设为私密后，整棵子树都会按私密处理，即使子文档自己没有勾选私密；
- 子文档设为私密不会影响父文档，父文档可以继续保持公开；
- 私密文档不写入公开阅读统计，也不出现在公开标签和首页统计中；
- 私密状态会随历史版本保存和恢复。
- 没有权限的登录读者打开私密文档链接时，可以填写说明申请阅读；
- 作者会在「管理后台 → 阅读申请」看到申请，也可以从站内通知进入；
- 通过后申请人可以阅读该私密范围及其所有子文档，拒绝或撤回后可补充说明重新申请；
- 申请人可以在「我的空间 → 阅读申请」查看进度并撤回待处理申请；
- 作者可以在后台撤销已通过的访问，权限会立即失效。

## 自测

完整的自测步骤、检查清单和故障排查见 [TESTING.md](TESTING.md)。最短路径：

```bash
cd server && go test ./...
```

```bash
python3 scripts/smoke.py
```

- `go test ./...` 使用临时 SQLite，无需任何外部服务，覆盖注册登录、内容 CRUD、点赞收藏、两级评论、关注、私信、通知、搜索、权限、级联删除和健康检查。
- `scripts/smoke.py` 对运行中的实例做端到端检查，会临时创建一个用户和一篇内容，测试结束后自动删除。Redis 在线时还会额外验证**登出黑名单**和**浏览量去重**。

## 生产构建

```bash
cd web && npm install && npm run build
cd ../server && go build -o folio .
FOLIO_SECRET="换成一个足够长的随机串" FOLIO_DEMO=0 ./folio
```

Go 服务会自动托管 `../web/dist`，访问 <http://localhost:8080> 即可。前端路由已做 SPA 回退。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `FOLIO_ADDR` | `:8080` | 监听地址 |
| `FOLIO_SECRET` | 开发用默认值 | JWT 签名密钥，同时用于加密后台七牛密钥和用户选择保存的 AI Key；**部署时必须修改，修改后需重新填写这些密钥** |
| `FOLIO_DB_DRIVER` | `sqlite` | `sqlite` / `mysql` / `memory` |
| `FOLIO_DB` | `./data/folio.db` | SQLite 文件路径 |
| `FOLIO_MYSQL_DSN` | 空 | MySQL DSN，例如 `folio:folio_pass@tcp(127.0.0.1:3306)/folio?charset=utf8mb4&parseTime=True&loc=Local` |
| `FOLIO_MEMORY` | `0` | 设为 `1` 等价于 `FOLIO_DB_DRIVER=memory` |
| `FOLIO_REDIS_ADDR` | 空 | Redis 地址，留空则关闭 Redis 功能 |
| `FOLIO_REDIS_PASSWORD` | 空 | Redis 密码 |
| `FOLIO_REDIS_DB` | `0` | Redis 库编号 |
| `FOLIO_STORAGE` | `local` | 图片存储默认值：`local` 或 `qiniu`；后台保存过存储设置后以网页配置为准 |
| `FOLIO_UPLOAD_DIR` | `./data/uploads` | 本地模式的正文图片保存目录 |
| `FOLIO_QINIU_ACCESS_KEY` | 空 | 七牛 AccessKey 的初始值；后台保存后以网页配置为准 |
| `FOLIO_QINIU_SECRET_KEY` | 空 | 七牛 SecretKey 的初始值；后台保存后以网页配置为准 |
| `FOLIO_QINIU_BUCKET` | 空 | 七牛空间名 |
| `FOLIO_QINIU_DOMAIN` | 空 | 七牛绑定域名，例如 `https://image.example.com` |
| `FOLIO_QINIU_REGION` | 空 | 空间区域；留空自动查询 |
| `FOLIO_QINIU_PREFIX` | `folio` | 对象键前缀 |
| `FOLIO_UPLOAD_MAX_MB` | `10` | 单张图片大小上限（MB） |
| `FOLIO_AI_API_KEY` | 空 | Anthropic API Key；仅用于编辑器 AI 摘要，留空则关闭该功能 |
| `FOLIO_AI_MODEL` | `claude-opus-5-5` | AI 摘要使用的模型 |
| `FOLIO_AI_BASE_URL` | 空 | 可选，Anthropic 兼容网关地址（AI 摘要与 Anthropic 问答共用） |
| `FOLIO_DEEPSEEK_BASE_URL` | 空 | 可选，DeepSeek 兼容网关地址；留空使用官方地址 |
| `FOLIO_GLM_BASE_URL` | 空 | 可选，智谱 GLM 兼容网关地址；留空使用官方地址 |
| `FOLIO_KIMI_BASE_URL` | 空 | 可选，Kimi 兼容网关地址；留空使用官方地址 |
| `FOLIO_QWEN_BASE_URL` | 空 | 可选，通义千问兼容网关地址；留空使用官方地址 |
| `FOLIO_SMTP_HOST` | 空 | SMTP 服务器；留空则关闭邮箱验证码 |
| `FOLIO_SMTP_PORT` | `587` | SMTP 端口，`465` 时自动使用隐式 TLS |
| `FOLIO_SMTP_USER` | 空 | SMTP 用户名，通常就是邮箱地址 |
| `FOLIO_SMTP_PASSWORD` | 空 | SMTP 密码或授权码 |
| `FOLIO_SMTP_FROM` | 空 | 发件邮箱；默认与用户名相同 |
| `FOLIO_SMTP_FROM_NAME` | 站点名 | 发件人显示名 |
| `FOLIO_SMTP_TLS` | `465` 端口为 `1` | 是否使用隐式 TLS（465）；587 端口走 STARTTLS |
| `FOLIO_MAIL_DEBUG` | `0` | 设为 `1` 时不发邮件，把验证码写进日志并在界面显示，仅用于自测 |
| `FOLIO_LOGIN_CAPTCHA` | `0` | 设为 `1` 时，密码登录和发送邮箱验证码需要图形验证码 |
| `FOLIO_CAPTCHA_DEBUG` | `0` | 设为 `1` 时接口返回图形验证码答案并在页面自动填入，**仅本地自测** |
| `FOLIO_QQ_APP_ID` | 空 | QQ 互联应用 ID；三项都填后可在「注册与邀请」开启 QQ 登录 |
| `FOLIO_QQ_APP_KEY` | 空 | QQ 互联应用密钥 |
| `FOLIO_QQ_REDIRECT` | 空 | QQ 回调地址，例如 `http://localhost:8080/api/auth/qq/callback` |
| `FOLIO_QQ_API_BASE` | `https://graph.qq.com` | QQ 接口地址，一般无需修改 |
| `FOLIO_WEB_URL` | 空 | QQ 登录成功后的前端地址；Vite 开发填 `http://localhost:5173`，同源部署留空 |
| `FOLIO_WEB` | `../web/dist` | 前端构建产物目录 |
| `FOLIO_DEMO` | `1` | 是否在空库时写入演示数据；`0` 关闭 |
| `FOLIO_SITE_NAME` | `知微` | 站点名 |
| `FOLIO_SITE_DESC` | 一句话简介 | 站点描述 |
| `FOLIO_SITE_OWNER` | `weijia` | 页脚署名 |
| `FOLIO_SITE_URL` | 空 | 站点公开地址，例如 `https://example.com`；留空时按请求 Host 推导，生产环境建议显式配置 |

复制 `.env.example` 为 `.env` 即可让 `./dev.sh` 读取。DSN 中含 `&`，在 `.env` 里请用引号包住。

## 数据模型

只有 25 张表：

| 表 | 用途 |
| --- | --- |
| `users` | 用户名、昵称、邮箱、密码哈希、简介、角色、最后登录 |
| `user_settings` | 隐私与消息偏好：关注/粉丝可见性、私信开关、点赞/评论通知、主页样式 |
| `categories` | 文章分类 |
| `documents` | 文章与文档共用：`kind`、`title`、`abstract`、`content`、`parent_id`、`category_id`、`tags`、`status`、`private`、`open_comment`、计数字段 |
| `likes` | 用户与内容点赞（唯一索引） |
| `favorite_folders` | 用户自定义收藏夹 |
| `favorites` | 用户与内容收藏（唯一索引，带 `folder_id`） |
| `comments` | 两级评论：`parent_id` 为空是根评论，否则指向根评论 |
| `comment_likes` | 评论点赞 |
| `follows` | 关注关系 |
| `messages` | 私信：类型（文字/图片）、正文、图片地址、已读时间 |
| `notifications` | 点赞 / 评论 / 关注通知 |
| `images` | 上传图片：文件名、存储提供方、对象键、URL、MIME、大小、内容哈希、上传者 |
| `ai_provider_keys` | 用户选择记住的 AI Key：按用户和服务商唯一，密钥加密保存 |
| `view_histories` | 登录用户的浏览历史（用户 + 内容唯一） |
| `doc_daily_stats` | 内容每日阅读量（内容 + 日期唯一），用于数据看板 |
| `invite_codes` | 一次性邀请码：备注、有效期、创建者、使用者和使用时间 |
| `document_revisions` | 内容历史版本：标题、摘要、正文、标签、状态、私密标记、编辑者与时间 |
| `doc_access_requests` | 私密文档阅读申请：私密范围、申请人、所有者、说明、状态与处理时间 |
| `email_codes` | 邮箱验证码：用途、HMAC 摘要、过期时间、尝试次数、消费时间 |
| `site_settings` | 单行站点设置：名称、描述、维护者、注册开关、QQ 登录开关、图形验证码 |
| `storage_settings` | 单行图片存储设置：提供方、加密后的七牛密钥、空间、域名、区域与对象前缀 |
| `operation_logs` | 操作日志：操作者、动作、对象、详情、IP、User-Agent |
| `global_notices` | 全站公告：标题、内容、链接、发布者 |
| `notice_reads` | 每个用户对全站公告的已读记录（公告 + 用户唯一） |

`documents.status` 只有 `draft` 与 `published`。草稿只有作者本人和超级管理员可见。分类只作用于文章，文档仍然使用 `parent_id` 目录树。

MySQL 与 SQLite 共用同一套模型和迁移代码；Redis 不保存业务数据，只保存 JWT 黑名单与浏览量去重键，因此可以随时清空。

## Redis 用来做什么

- **登出黑名单**：JWT 是无状态的，登出时把 token 的 `jti` 写入 Redis，直到它自然过期。没有 Redis 时，登出只清理前端状态。
- **浏览量去重**：同一用户（未登录时按 IP）在 30 分钟内重复打开同一篇内容只计一次浏览。
- **自动降级**：`FOLIO_REDIS_ADDR` 为空，或启动时连接失败，站点仍可完整运行，只是上述两项失效。
- **健康检查**：`GET /api/health` 会返回 `db`、`dbStatus`、`redis`（`up` / `off` / `down`）。

## 生成式视觉

界面采用「纸面 + 铅笔线」的极简手绘风格：冷白纸色、墨色文字、单一墨蓝强调色。rough.js 负责所有手绘线条，CSS 负责不规则的纸片边框与留白。

### 封面

前端 `CoverImage` 组件用 Canvas + rough.js 绘制，服务端 `GET /api/art/cover.svg` 提供同风格的 SVG 兜底。

- 种子是内容编号与类型，同一篇内容得到稳定的版式与配色，修改标题不会让整张封面跳变；
- 左侧或右侧的纸面索引卡会排入分类与类型、标题、摘要片段和最多 3 个标签；标题超过两行会省略；
- 另一侧根据标签、分类和标题中的关键词选择线稿：技术/代码配方格，知识/写作配纸页，设计/排版配拱门，AI/数据配轨道，旅行/自然配山脊，情绪/思考配植物；
- 标题和标签较长时会自动截断，保证不同宽度下都不溢出；
- 印章取第一个标签（没有标签时取分类或标题）的首字，与卡片上的标签形成呼应。

### 头像

前端 `UserAvatar` 组件用 Canvas 绘制，服务端 `GET /api/art/avatar.svg?seed=<用户名>` 提供 SVG 兜底。

- 一个手绘圆 + 昵称的第一个字；
- 底色从四组淡色纸面中按用户名哈希选择；
- 不存储、不上传、不需要裁剪。

### 手绘细节

- 品牌标识是一个 rough.js 画的圆和一道斜线；
- 首屏关键词下方是 rough.js 的手绘下划线，引用区上方是手绘波浪线；
- 登录/注册页的纸面注记使用 rough.js 的圆与波浪线；
- 卡片、按钮、输入框使用 CSS 不规则圆角和虚线边框，避免整齐划一的组件库质感。

封面、头像、装饰线都会在切换明暗主题时重新绘制，颜色从当前主题的 CSS 变量读取。

## 图片与对象存储

默认使用本地磁盘，也可以把新图片切换到七牛云。两种模式共用同一个上传接口、去重逻辑和管理后台图库。

### 在网页里配置（推荐）

超级管理员进入「管理后台 → 存储设置」，可以直接：

- 在本地磁盘和七牛云之间切换，保存后立即生效，不需要重启；
- 填写 AccessKey、SecretKey、空间名、绑定域名、区域和对象前缀；
- 点「测试连接」会上传一个临时文件并立即删除，用来验证密钥、空间和写入权限；
- 密钥不会回显：读取接口只返回「是否已配置」，留空表示保持原密钥；
- 七牛密钥用 `FOLIO_SECRET` 派生出的 AES-GCM 密钥加密后写入数据库；如果更换了 `FOLIO_SECRET`，需要在后台重新填写七牛密钥；
- 后台保存过配置后，数据库配置优先于环境变量；环境变量只作为首次启动或未保存过配置时的默认值。

两种模式的行为：

- 本地模式：文件写到 `FOLIO_UPLOAD_DIR`，通过 `/uploads/*` 访问；
- 七牛模式：文件由服务端上传到七牛，返回绑定域名下的 URL；
- 已存在的图片继续按各自记录的存储方式访问和删除，切换模式不会丢历史图片；
- 删除图片时会先检查是否还有其他记录引用同一个对象，没有引用才删除七牛或本地文件；
- 目前面向公开读取的 Bucket；私有 Bucket 需要额外的签名代理，暂未启用。

### 用环境变量提供初始配置

不经过网页时，也可以在 `.env` 里提供初始值；后台保存后会以网页配置为准。

```bash
FOLIO_STORAGE=qiniu
FOLIO_QINIU_ACCESS_KEY=你的AccessKey
FOLIO_QINIU_SECRET_KEY=你的SecretKey
FOLIO_QINIU_BUCKET=your-bucket
FOLIO_QINIU_DOMAIN=https://image.example.com
FOLIO_QINIU_REGION=z0
```

正文图片使用本站上传，不再要求外链：

- 编辑器支持**选择文件、粘贴剪贴板图片、拖拽图片**三种方式；
- 上传后自动插入 Markdown：本地模式是 `/uploads/2026/10/xxxx.png`，七牛模式是完整的云存储 URL；
- 支持 JPG、PNG、GIF、WebP，默认单张上限 10MB；
- 文件按内容哈希命名，同一张图重复上传只会存一份；
- 管理后台的「图片管理」可以预览、复制链接、下载和删除图片；超级管理员看全站，管理员只看自己上传的；
- 封面与头像仍然由 rough.js 自动生成，不提供上传。

Docker 全栈使用独立的 `uploads-data` 卷保存本地图片，重建容器不会丢失；切换到七牛后新图不再落这个卷。私信里的图片也走同一套上传接口，登录读者即可上传聊天图片。

## AI 摘要（可选）

编辑器里的「AI 摘要」按钮会用正文生成标题、摘要和标签。它通过官方 Anthropic Go SDK 调用 Messages API，默认模型是 `claude-opus-5-5`。

- 配置 `FOLIO_AI_API_KEY` 后启用；不配置时按钮不会出现，其他功能不受影响；
- 这里的站点 Key 只服务编辑器摘要，`/ask` 问答始终要求用户自带 Key；
- 只会补空字段，不会覆盖你已经写好的标题、摘要或标签；
- 正文会发送到 Anthropic（或你通过 `FOLIO_AI_BASE_URL` 指定的兼容网关）；
- 正文少于 20 字或超过 6 万字时会拒绝请求，避免误触和过长计费；
- 只有超级管理员和管理员可以使用。

```bash
FOLIO_AI_API_KEY=sk-ant-... FOLIO_AI_MODEL=claude-opus-5-5 ./dev.sh
```

## QQ 登录（可选）

在 [QQ 互联](https://connect.qq.com/) 创建网站应用后，配置：

```bash
FOLIO_QQ_APP_ID=你的 AppID
FOLIO_QQ_APP_KEY=你的 AppKey
FOLIO_QQ_REDIRECT=http://localhost:8080/api/auth/qq/callback
```

回调地址必须与 QQ 互联后台的「回调地址」完全一致。第一次用 QQ 登录会自动创建账号（用户名形如 `qq_ab12cd34`，昵称取 QQ 昵称，密码随机），之后用同一个 QQ 号会回到同一账号。QQ 用户可以在「账号安全」里绑定邮箱，再用邮箱验证码设置密码。

配置好凭据并重启后，QQ 登录默认开启；超级管理员可以在「管理后台 → 注册与邀请」随时关闭或重新开启。还没有申请 QQ 互联应用时保持关闭即可。

流程是服务端回调：QQ 授权后跳回 `/api/auth/qq/callback`，服务端校验签名与 state，再用 5 分钟有效的一次性票据把浏览器送回 `/login` 并换取登录 token，token 不出现在 URL 里。本地 Vite 开发时把 `FOLIO_WEB_URL` 设为 `http://localhost:5173`；Docker 或同源部署留空即可。

## AI 问答（用户自备 Key）

`/ask` 页面允许任何登录用户向站内知识库提问：

- 用户必须自己填写所选服务商的 API Key，**本站不提供 Key，也不会用站点的 `FOLIO_AI_API_KEY` 代替**；
- 可选服务商：Anthropic、DeepSeek、智谱 GLM、Kimi、通义千问；每个服务商自带常用模型，也可以手动填写新模型名称；
- 默认只在当前浏览器标签页的 sessionStorage 中记住 Key（按服务商分别记忆）；
- 勾选「记住 Key」后，Key 会使用 `FOLIO_SECRET` 加密保存到 `ai_provider_keys` 表，下次留空即可直接使用；接口只返回是否已保存，不回显明文，不写操作日志，可以随时清除；
- 知识库由已发布的文章、文档和「站点帮助」目录共同组成。文章会切成分块，标题、摘要、标签、分类和正文一起参与检索，取最相关的片段而不是整篇塞给模型；
- 「站点帮助」是一份可扩展的功能目录，同时驱动公开的 `/about` 关于页；每条功能带标题、摘要、步骤、路由、关键词和适用角色，管理员专属功能只对管理员检索；
- 回答支持两种模式：严格站内（只依据资料）和通用知识（资料不足时补充站外常识并明确标注）；
- 支持多轮追问、SSE 流式输出、来源片段展开和知识库覆盖统计；对话只保存在当前标签页；
- 回答要求优先引用站内资料，并返回来源列表；资料不足时会明确说明；
- 如果通过反向代理部署，请使用 HTTPS，避免 Key 在传输过程中暴露。

关于页与站点帮助来自同一份目录文件 `server/handlers/site_help.json`。新增功能时补充一条记录，关于页、AI 知识库和覆盖统计会一起更新。记录支持 `screenshots` 和 `video` 字段，关于页会自动渲染；目前首页、文章、文档、编辑器和问 AI 已配有截图。

如果某个服务商需要通过代理或私有网关访问，可以在部署环境里覆盖它的地址（留空则使用官方地址）：

| 环境变量 | 默认地址 |
| --- | --- |
| `FOLIO_AI_BASE_URL` | `https://api.anthropic.com` |
| `FOLIO_DEEPSEEK_BASE_URL` | `https://api.deepseek.com/v1` |
| `FOLIO_GLM_BASE_URL` | `https://open.bigmodel.cn/api/paas/v4` |
| `FOLIO_KIMI_BASE_URL` | `https://api.moonshot.cn/v1` |
| `FOLIO_QWEN_BASE_URL` | `https://dashscope.aliyuncs.com/compatible-mode/v1` |

## 隐私与消息偏好

「我的空间 → 隐私与消息」里可以设置：

- 是否公开关注列表、粉丝列表（关闭后列表与数量都会隐藏，自己仍然看得到）；
- 是否允许别人发送私信（关闭后新私信会被拒绝，已有对话仍可查看）；
- 是否公开我的收藏夹（默认私密，开启后访客可以在公开主页按收藏夹浏览已发布内容）；
- 是否生成评论/回复通知、点赞通知；
- 公开主页头部样式：`cover`（封面）、`compact`（紧凑）、`plain`（纯文字）。

在「我的文章」里可以把一篇已发布文章置顶到公开主页；每位作者最多一篇。

「账号安全」里还会显示最近 12 条登录与安全动作（登录、注册、改密、找回密码、绑定邮箱），数据来自操作日志，只对本人可见。

这些设置保存在 `user_settings`，没有记录时按全部公开、全部通知处理，老用户不需要迁移。

## 邮箱验证码（可选）

配置 SMTP 后，站点会启用邮箱相关能力：

- 注册时先给邮箱发 6 位验证码，验证通过才能创建账号；
- 登录页多一个「邮箱验证码」标签，可以不用密码登录；
- 「我的空间 → 账号安全」里可以绑定或更换邮箱，绑定同样要验证；
- 忘记密码时用邮箱验证码设置新密码；
- 验证码 10 分钟有效，只存 HMAC 摘要，验证成功或用错 5 次后立即作废；
- 同一邮箱 60 秒内只能发一次，24 小时最多 10 封。

如果站点对外开放，建议同时开启登录图形验证码：

```bash
FOLIO_LOGIN_CAPTCHA=1
```

开启后密码登录和发送邮箱验证码都需要先输入图片上的 4 位字符；验证码一次性使用，5 分钟内有效。图形验证码由服务端生成 PNG，不依赖前端组件库。本地没有 OCR 时可以临时设置 `FOLIO_CAPTCHA_DEBUG=1`，接口和页面会直接显示答案；**部署时不要开启**。

本地没有 SMTP 也可以自测，把 `FOLIO_MAIL_DEBUG` 设为 `1`：

```bash
FOLIO_MAIL_DEBUG=1 ./dev.sh
```

此时不会真的发信，验证码会打印在服务端日志里，并自动填入页面上的「开发模式验证码」提示。**部署时不要开启这个开关。**

常见的 QQ 邮箱配置：

```bash
FOLIO_SMTP_HOST=smtp.qq.com
FOLIO_SMTP_PORT=587
FOLIO_SMTP_USER=you@qq.com
FOLIO_SMTP_PASSWORD=你的 SMTP 授权码
FOLIO_SMTP_FROM=you@qq.com
```

## 全站通知

管理员在 `/admin` 的「全站通知」里发布公告，内容会显示在每个登录用户的「消息 → 通知」顶部，并计入顶栏未读徽标。公告支持一个站内或站外链接，已读状态按用户分别记录在 `notice_reads`。删除公告时会同时清理对应的已读记录。

## 版本历史与导出

编辑已有内容后，编辑器顶部会出现「历史版本」：

- 每次实际修改并保存时，保存前的版本会写入 `document_revisions`；
- 历史面板可以查看旧标题、状态、正文和编辑者，并一键恢复；
- 恢复前会先把当前内容再存一条历史，不会覆盖掉唯一副本；
- 删除内容时会一并清理它的历史版本。

管理后台「内容管理」提供「导出 Markdown」：

- 下载 zip，按 `articles/` 与 `docs/` 分目录；
- 每篇带 YAML frontmatter（标题、类型、状态、作者、分类、标签、时间、上级文档）和正文；
- 图片不会被复制进 zip，仍引用站内 `/uploads` 或对象存储地址，备份时需要连同图片一起保存。

## 数据看板

管理后台的「数据看板」提供 7 / 30 / 90 天视图：

- 每日阅读、内容新增、读者新增、登录次数趋势；
- 区间阅读、累计阅读、新增内容、新增读者、今日登录、图片存储概览；
- 热门内容排行和最近管理动态；
- 数据库、Redis、图片存储、Go 版本、goroutine、内存和运行时间。

每日阅读写入 `doc_daily_stats`，同一篇内容同一天只保留一行；演示模式会在统计表为空时补一份近 30 天的展示数据，真实部署设置 `FOLIO_DEMO=0` 后只记录实际访问。登录趋势来自操作日志，因此从本版本开始累计。

## 站点设置与操作日志

超级管理员进入 `/admin` 后有两个额外入口：

- **站点设置**：修改站点名称、描述、页脚维护者，决定是否开放注册，以及是否要求登录图形验证码。保存后页头、页脚与首页立即更新；
- **操作日志**：记录登录、注册、创建/更新/删除内容、分类、图片、角色调整、站点设置等动作，包含操作者、时间、IP 与对象编号，可按关键词搜索并分页。

`FOLIO_SITE_NAME`、`FOLIO_SITE_DESC`、`FOLIO_SITE_OWNER` 和 `FOLIO_LOGIN_CAPTCHA` 只在数据库还没有站点设置时作为初始值。第一次启动后，以管理后台里的值为准。

## SEO、RSS 与分享卡片

站点不依赖前端 JS 也能提供基础 SEO 信息：

- `/feed.xml`：最近 30 篇已发布文章的 RSS 2.0 订阅；
- `/sitemap.xml`：首页、文章/文档列表、全部已发布内容与公开读者主页；
- `/robots.txt`：屏蔽后台、登录、私信、编辑与搜索页，并指向 sitemap；
- `/api/art/cover.png`：1200×630 的分享卡片底图，沿用站内封面的纸面与线稿语言；
- Go 服务在返回 `index.html` 时会根据当前路径注入标题、description、canonical、Open Graph、Twitter Card 和 JSON-LD；爬虫不执行 JS 也能读到；
- 文章/文档页的「分享」按钮会打开一张可视化分享卡片，支持复制链接和系统分享。

生产环境请设置：

```bash
FOLIO_SITE_URL=https://your-domain.com
```

不设置时会按请求的 `Host` 推导，效果在本地可用，但反向代理后可能出现错误域名。

## 页面与功能

- `/` 首页：首屏、置顶文章、最近文章、知识库入口、标签与最近加入
- `/articles` 文章列表：标签、分类筛选，最新/最多阅读/最多点赞排序，分页
- `/articles/:id` 文章详情：自动封面、Markdown 正文、固定目录、移动端目录抽屉、图片灯箱、右侧点赞/收藏/分享、阅读进度、回到顶部、作者卡、相关文章、评论
- `/docs` 知识库总览：左侧目录树 + 文档卡片
- `/docs/:id` 文档详情：目录树、面包屑、正文与固定右侧目录、移动端目录抽屉、图片灯箱、上一篇/下一篇、右侧点赞收藏/分享、评论；私密文档只有作者和超级管理员可访问
- `/search` 全站搜索：关键词搜索；点击任意标签进入该标签的全站聚合
- `/about` 关于：按场景系统介绍站点全部功能，支持页面内搜索、角色标记和截图，也是 AI 问答的站点帮助目录
- `/u/:username` 公开主页：文章、文档、关注、粉丝、公开收藏夹和作者置顶文章；可按隐私设置隐藏关注/粉丝/收藏，并支持封面、紧凑、纯文字三种头部样式
- `/write`、`/write/:id` 编辑器：Markdown 分栏预览、图片上传/粘贴/拖拽、分类、自动封面预览、标签、评论开关、历史版本、草稿/发布、文档树父级与排序（仅超级管理员和管理员）
- `/messages` 私信与通知：会话列表、WebSocket 实时消息、图片与表情、轮询兜底、全站公告与通知已读
- `/ask` 问 AI：用户自带 API Key（Anthropic、DeepSeek、智谱 GLM、Kimi、通义千问），检索已发布内容与站点帮助，支持严格/通用模式、多轮追问和流式回答
- `/me` 我的空间：资料、密码、隐私与消息、最近登录记录、收藏夹、浏览历史；管理员额外看到自己的文章与文档，并可以把一篇文章置顶到公开主页
- `/admin` 管理后台：概览、数据看板、内容管理、分类管理、图片管理、阅读申请、全站通知；内容管理支持导出全部 Markdown；超级管理员额外拥有用户与权限、注册与邀请、存储设置、站点设置和操作日志
- `/login`、`/register` 登录注册：密码登录、邮箱验证码登录、QQ 登录、注册；关闭注册后可通过一次性邀请链接注册
- `/forgot` 通过邮箱验证码重置密码

交互细节：

- 按 `/` 或 `⌘K` 唤起快速搜索与导航，支持键盘上下选择、回车打开；
- 文章与文档页顶部有阅读进度条，右侧目录会跟随滚动高亮当前章节；
- 列表和详情页使用骨架屏，页面切换有轻量过渡，遵循系统的减少动态效果设置；
- 编辑器会提示「未保存」并在离开页面前确认；每 3 秒自动保存本地草稿，刷新或重新打开时可以恢复；密码输入框可以切换显示；
- 移动端导航自动收起管理入口，避免顶栏拥挤。

互动说明：

- 私信通过 WebSocket 实时推送；连接不可用或断开时，页面每 10 秒轮询兜底。打开会话时会标记已读；会话超过 50 条时可以继续加载更早消息；
- 私信支持发送图片和常用表情，图片走同一套上传接口，按当前存储模式返回本地或七牛 URL；
- 普通通知只针对点赞、评论/回复、关注三种行为；管理员还可以发布全站公告，公告单独显示在通知页顶部。
- 收藏是私密的，只有自己能在「我的收藏」里看到；收藏可以分到不同收藏夹，默认收藏夹不能删除；
- 评论只有两级，回复会显示「回复 @某某」；编辑器里关掉「允许读者评论」后，接口会拒绝新评论，已有评论仍可阅读和点赞。

## API 概要

所有接口以 `/api` 开头，成功时返回资源本身（列表为 `{items, total, page, pageSize}`），失败时使用 HTTP 状态码并返回 `{"error": "中文说明"}`。

| 分组 | 主要接口 |
| --- | --- |
| 认证 | `POST /auth/register`、`POST /auth/login`、`POST /auth/email/login`、`POST /auth/password/reset`、`POST /auth/email/code`、`GET /auth/email/status`、`GET /auth/captcha`、`GET /auth/qq/status`、`GET /auth/qq/url`、`GET /auth/qq/callback`、`POST /auth/qq/exchange`、`POST /auth/logout`、`GET /me`、`PUT /me/email` |
| 用户 | `GET /users/:username`、`POST|DELETE /users/:username/follow`、`GET /users/:username/followers|following`、`GET /users/:username/favorite-folders`、`GET /users/:username/favorites` |
| 内容 | `GET /docs`、`GET /docs/:id`、`GET /docs/:id/access`、`POST /docs/:id/access-requests`、`GET /docs/:id/related`、`GET /docs/:id/revisions`、`GET /docs/:id/revisions/:revisionID`、`POST /docs/:id/revisions/:revisionID/restore`、`POST /docs`、`PUT /docs/:id`、`DELETE /docs/:id`、`GET /doc-tree` |
| 分类 | `GET /categories`、`POST /categories`、`PUT /categories/:id`、`DELETE /categories/:id` |
| 图片 | `POST /images`、`GET /images`、`DELETE /images/:id`；本地模式通过 `/uploads/*` 访问，七牛模式返回云存储 URL |
| AI | `GET /ai/status`、`POST /ai/analyze`（仅管理员）、`GET /ai/providers`、`GET /ai/meta`、`GET|PUT /ai/keys`、`DELETE /ai/keys/:provider`、`POST /ai/ask`、`POST /ai/ask/stream`（SSE，登录用户需自带 Key） |
| 互动 | `POST /docs/:id/like`、`POST /docs/:id/favorite`、`GET|POST /docs/:id/comments`、`POST /comments/:id/like` |
| 消息 | `GET /messages/conversations`、`GET|POST /messages/with/:username`、`GET /notifications`、`GET /notices`、`POST /notices/read`、`GET /ws`（WebSocket，token 查询参数） |
| 我的 | `GET /me/docs`、`GET /me/favorites`、`GET|DELETE /me/history`、`GET /me/access-requests`、`GET /me/access-requests/outgoing`、`GET|POST /favorite-folders`、`PUT|DELETE /favorite-folders/:id`、`PUT /me`、`PUT /me/password`、`GET|PUT /me/settings`、`PUT /me/featured-doc`、`GET /me/login-records`、`GET /inbox` |
| 视觉 | `GET /art/cover.svg`、`GET /art/cover.png`、`GET /art/avatar.svg` |
| 站点 | `GET /site`、`GET /site/features`（公开的功能目录，关于页使用） |
| SEO | `GET /feed.xml`、`GET /sitemap.xml`、`GET /robots.txt` |
| 系统 | `GET /health` |
| 管理 | `PUT|DELETE /access-requests/:id`、`GET /admin/stats`、`GET /admin/dashboard`、`GET /admin/docs`、`GET /admin/users`、`POST /admin/users`、`DELETE /admin/users/:id`、`PUT /admin/users/:id/role`、`GET|POST /admin/invites`、`DELETE /admin/invites/:id`、`GET|PUT /admin/access`、`GET|PUT /admin/settings`、`GET|PUT /admin/storage`、`POST /admin/storage/test`、`GET /admin/logs`、`GET|POST /admin/notices`、`DELETE /admin/notices/:id`、`GET /admin/export` |

## 一些实现约定

- **摘要**：不填时自动从 Markdown 正文提取前 120 字。
- **计数**：点赞、收藏、评论写入后重新 `COUNT(*)` 回写，避免增量漂移。
- **删除**：删除文档会递归删除子文档及其评论、点赞、收藏、通知；删除根评论会连同回复一起删除；删除用户会清理其全部内容、互动、图片和浏览历史。
- **图片**：支持上传、粘贴与拖拽；默认保存在 `FOLIO_UPLOAD_DIR`，可以在管理后台「存储设置」切换到七牛，也可以用环境变量提供初始配置；封面与头像仍然是自动生成，不能上传。
- **备份**：SQLite 可用 `sqlite3 folio.db ".backup 'backup.db'"`；MySQL 使用 `mysqldump`；本地图片目录/`uploads-data` 卷需要一起备份，七牛图片由对象存储侧负责备份与生命周期。
- **主题**：默认跟随系统明暗偏好，切换后记住选择。

## 已知边界

- 通知仍然靠 10 秒轮询；私信有 WebSocket 实时推送与轮询兜底。
- 邮件是可选的：未配置 SMTP 时邮箱验证码、邮箱登录和找回密码会自动关闭，用户名密码登录不受影响。
- 搜索使用 `LIKE`，适合个人规模的数据量；MySQL 下同样可用。
- 演示数据默认开启；正式使用请设置 `FOLIO_DEMO=0` 并修改 `FOLIO_SECRET`。
- 旧四个项目的 MySQL 数据尚未迁移到新表结构；需要的话可以再写一个导入脚本。
- 操作日志没有自动归档；个人站点数据量下通常无需处理，数据量大时可以按时间手动清理 `operation_logs`。
- 按需求移除了 Banner、文章审核流、文档权限/密码；收藏保持私密，没有公开收藏页。
