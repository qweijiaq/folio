# 知微 · Folio

一个人的知识花园：把博客文章与知识库文档合并到同一个站点里，共用一套用户、互动与视觉系统。

这个项目由 `blogx_server` / `blogx_web`（博客）与 `gvd_server` / `gvd_web`（文档库）合并重写而来。原四个项目保持原样，仅作为参考。

## 核心变化

- **一张用户表、一张内容表**：所有内容都在 `documents` 表，用 `kind` 区分 `article`（文章）与 `doc`（文档）。文档用 `parent_id` 组成目录树，文章始终是平的。
- **不做封面上传**：封面与头像由 rough.js 在浏览器里实时绘制。封面是一条极简的几何线稿，头像是一个手绘圆加昵称首字；同一篇内容永远得到同一张封面。服务端 `/api/art/*` 保留同风格的轻量 SVG 版本，供接口和分享图使用。
- **CSS 全部手写**：不引入 Arco 或其他组件库，也不使用预处理器。四个 CSS 文件构成设计系统，支持明暗主题。
- **三种数据模式**：SQLite（默认）、MySQL、进程内内存 Mock。Redis 可选，用于登出黑名单与浏览量去重。
- **保留内容能力**：正文图片上传与图库、粘贴/拖拽插图、文章分类、文章推荐、收藏夹分组、浏览历史、评论、点赞、收藏、关注、私信、通知都已恢复。
- **删繁就简**：去掉复杂的角色表、文档权限、文档密码与试看、审核流、QQ 与邮箱登录、验证码、Elasticsearch；只保留一个极简的三级角色字段。
- **保留基本互动**：评论（两级）、评论点赞、内容点赞、收藏、关注/粉丝、私信、站内通知、管理后台。
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
| 登录 | JWT（30 天）+ 可选 Redis 黑名单 |
| 前端 | Vue 3 · Vite · Vue Router |
| 手绘视觉 | rough.js（Canvas 封面/头像 + SVG 装饰线） |
| 内容渲染 | markdown-it + highlight.js（仅语法标记，主题 CSS 手写） |
| 样式 | 原生 CSS，明暗主题，响应式 |

## 目录结构

```
folio/
├── server/
│   ├── main.go             # 启动、静态资源托管、SPA 回退
│   ├── config/             # 环境变量配置
│   ├── db/                 # SQLite / MySQL / 内存数据库
│   ├── cache/              # Redis：JWT 黑名单、浏览量去重
│   ├── models/             # 13 张表的模型
│   ├── art/                # 生成式封面与头像
│   ├── handlers/           # 路由、接口与集成测试
│   └── seed/               # 演示数据
├── web/
│   ├── src/views/          # 13 个页面
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
| 发布、编辑、删除自己的内容 | ✓ | ✓ | — |
| 编辑、删除任何人的内容 | ✓ | — | — |
| 进入管理后台 | ✓ | ✓（只看自己的内容） | — |
| 删除用户、设置或取消管理员 | ✓ | — | — |

规则很简单：站点只有一个超级管理员，通常是第一位注册用户；超级管理员在 `/admin` 的「用户与权限」里把读者设为管理员，或把管理员降回读者。管理员降级后不能继续发布，但已经发布的内容和互动会保留。

管理后台入口在登录后的顶栏齿轮图标、用户下拉菜单「管理后台」，以及「我的空间」侧栏。

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
| `FOLIO_SECRET` | 开发用默认值 | JWT 签名密钥，**部署时必须修改** |
| `FOLIO_DB_DRIVER` | `sqlite` | `sqlite` / `mysql` / `memory` |
| `FOLIO_DB` | `./data/folio.db` | SQLite 文件路径 |
| `FOLIO_MYSQL_DSN` | 空 | MySQL DSN，例如 `folio:folio_pass@tcp(127.0.0.1:3306)/folio?charset=utf8mb4&parseTime=True&loc=Local` |
| `FOLIO_MEMORY` | `0` | 设为 `1` 等价于 `FOLIO_DB_DRIVER=memory` |
| `FOLIO_REDIS_ADDR` | 空 | Redis 地址，留空则关闭 Redis 功能 |
| `FOLIO_REDIS_PASSWORD` | 空 | Redis 密码 |
| `FOLIO_REDIS_DB` | `0` | Redis 库编号 |
| `FOLIO_UPLOAD_DIR` | `./data/uploads` | 正文图片的保存目录 |
| `FOLIO_UPLOAD_MAX_MB` | `10` | 单张图片大小上限（MB） |
| `FOLIO_WEB` | `../web/dist` | 前端构建产物目录 |
| `FOLIO_DEMO` | `1` | 是否在空库时写入演示数据；`0` 关闭 |
| `FOLIO_SITE_NAME` | `知微` | 站点名 |
| `FOLIO_SITE_DESC` | 一句话简介 | 站点描述 |
| `FOLIO_SITE_OWNER` | `weijia` | 页脚署名 |

复制 `.env.example` 为 `.env` 即可让 `./dev.sh` 读取。DSN 中含 `&`，在 `.env` 里请用引号包住。

## 数据模型

只有 13 张表：

| 表 | 用途 |
| --- | --- |
| `users` | 用户名、昵称、邮箱、密码哈希、简介、角色、最后登录 |
| `categories` | 文章分类 |
| `documents` | 文章与文档共用：`kind`、`title`、`abstract`、`content`、`parent_id`、`category_id`、`tags`、`status`、计数字段 |
| `likes` | 用户与内容点赞（唯一索引） |
| `favorite_folders` | 用户自定义收藏夹 |
| `favorites` | 用户与内容收藏（唯一索引，带 `folder_id`） |
| `comments` | 两级评论：`parent_id` 为空是根评论，否则指向根评论 |
| `comment_likes` | 评论点赞 |
| `follows` | 关注关系 |
| `messages` | 私信 |
| `notifications` | 点赞 / 评论 / 关注通知 |
| `images` | 上传图片：文件名、路径、MIME、大小、内容哈希、上传者 |
| `view_histories` | 登录用户的浏览历史（用户 + 内容唯一） |

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

- 种子仍然是内容编号与类型，同一篇内容永远得到同一张封面；
- 每一张封面只有：一层淡色纸面 + 一个几何线稿（圆、波浪、三条短线或折线）+ 最多一小笔墨蓝；
- 不再包含标题、首字印章或复杂纹样，封面只作为安静的装饰。

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

## 图片

正文图片使用本站上传，不再要求外链：

- 编辑器支持**选择文件、粘贴剪贴板图片、拖拽图片**三种方式；
- 上传后自动插入 Markdown，例如 `![图片](/uploads/2026/10/xxxx.png)`；
- 支持 JPG、PNG、GIF、WebP，默认单张上限 10MB；
- 文件按内容哈希命名，同一张图重复上传只会存一份；
- 管理后台的「图片管理」可以预览、复制链接、下载和删除图片；超级管理员看全站，管理员只看自己上传的；
- 封面与头像仍然由 rough.js 自动生成，不提供上传。

Docker 全栈使用独立的 `uploads-data` 卷保存图片，重建容器不会丢失。

## 页面与功能

- `/` 首页：首屏、置顶文章、最近文章、知识库入口、标签与最近加入
- `/articles` 文章列表：标签、分类筛选，最新/最多阅读/最多点赞排序，分页
- `/articles/:id` 文章详情：自动封面、Markdown 正文、目录、点赞、收藏、分享、作者卡、相关文章、评论
- `/docs` 知识库总览：左侧目录树 + 文档卡片
- `/docs/:id` 文档详情：目录树、面包屑、正文与右侧目录、上一篇/下一篇、点赞收藏、评论
- `/search` 全站搜索：关键词搜索；点击任意标签进入该标签的全站聚合
- `/u/:username` 公开主页：文章、文档、关注、粉丝
- `/write`、`/write/:id` 编辑器：Markdown 分栏预览、图片上传/粘贴/拖拽、分类、自动封面预览、标签、草稿/发布、文档树父级与排序（仅超级管理员和管理员）
- `/messages` 私信与通知：会话列表、消息轮询、通知已读
- `/me` 我的空间：资料、密码、收藏夹、浏览历史；管理员额外看到自己的文章与文档
- `/admin` 管理后台：概览、内容管理、分类管理、图片管理；超级管理员额外拥有用户与权限管理
- `/login`、`/register` 登录注册

互动说明：

- 私信没有 WebSocket，页面每 10 秒拉取一次，并在打开会话时标记已读；
- 通知只针对点赞、评论/回复、关注三种行为；
- 收藏是私密的，只有自己能在「我的收藏」里看到；收藏可以分到不同收藏夹，默认收藏夹不能删除；
- 评论只有两级，回复会显示「回复 @某某」。

## API 概要

所有接口以 `/api` 开头，成功时返回资源本身（列表为 `{items, total, page, pageSize}`），失败时使用 HTTP 状态码并返回 `{"error": "中文说明"}`。

| 分组 | 主要接口 |
| --- | --- |
| 认证 | `POST /auth/register`、`POST /auth/login`、`POST /auth/logout`、`GET /me` |
| 用户 | `GET /users/:username`、`POST|DELETE /users/:username/follow`、`GET /users/:username/followers|following` |
| 内容 | `GET /docs`、`GET /docs/:id`、`GET /docs/:id/related`、`POST /docs`、`PUT /docs/:id`、`DELETE /docs/:id`、`GET /doc-tree` |
| 分类 | `GET /categories`、`POST /categories`、`PUT /categories/:id`、`DELETE /categories/:id` |
| 图片 | `POST /images`、`GET /images`、`DELETE /images/:id`，文件通过 `/uploads/*` 访问 |
| 互动 | `POST /docs/:id/like`、`POST /docs/:id/favorite`、`GET|POST /docs/:id/comments`、`POST /comments/:id/like` |
| 消息 | `GET /messages/conversations`、`GET|POST /messages/with/:username`、`GET /notifications` |
| 我的 | `GET /me/docs`、`GET /me/favorites`、`GET|DELETE /me/history`、`GET|POST /favorite-folders`、`PUT|DELETE /favorite-folders/:id`、`PUT /me`、`PUT /me/password`、`GET /inbox` |
| 视觉 | `GET /art/cover.svg`、`GET /art/avatar.svg` |
| 系统 | `GET /health` |
| 管理 | `GET /admin/stats`、`GET /admin/docs`、`GET /admin/users`、`DELETE /admin/users/:id`、`PUT /admin/users/:id/role` |

## 一些实现约定

- **摘要**：不填时自动从 Markdown 正文提取前 120 字。
- **计数**：点赞、收藏、评论写入后重新 `COUNT(*)` 回写，避免增量漂移。
- **删除**：删除文档会递归删除子文档及其评论、点赞、收藏、通知；删除根评论会连同回复一起删除；删除用户会清理其全部内容、互动、图片和浏览历史。
- **图片**：支持上传、粘贴与拖拽，文件保存在 `FOLIO_UPLOAD_DIR`；封面与头像仍然是自动生成，不能上传。
- **备份**：SQLite 可用 `sqlite3 folio.db ".backup 'backup.db'"`；MySQL 使用 `mysqldump`；图片目录/`uploads-data` 卷需要一起备份。
- **主题**：默认跟随系统明暗偏好，切换后记住选择。

## 已知边界

- 没有实时推送，私信与通知靠轮询。
- 没有邮件服务，注册与找回密码不需要邮箱验证。
- 搜索使用 `LIKE`，适合个人规模的数据量；MySQL 下同样可用。
- 演示数据默认开启；正式使用请设置 `FOLIO_DEMO=0` 并修改 `FOLIO_SECRET`。
- 旧四个项目的 MySQL 数据尚未迁移到新表结构；需要的话可以再写一个导入脚本。
