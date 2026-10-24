package seed

import (
	"log"
	"strings"
	"time"

	"folio/server/config"
	"folio/server/models"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// md 让示例正文可以用 § 表示反引号，避免在 Go 原始字符串里出现反引号。
func md(s string) string { return strings.ReplaceAll(s, "§", "`") }

type seedUser struct {
	username, nickname, bio, role string
}

type seedDoc struct {
	key, kind, title, abstract, content, author string
	tags                                        []string
	status, parent                              string
	daysAgo, sort, views                        int
}

func Run(gdb *gorm.DB, cfg config.Config) error {
	var count int64
	if err := gdb.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	log.Println("首次启动，写入演示内容…")
	now := time.Now()
	hash, err := bcrypt.GenerateFromPassword([]byte("folio1234"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	users := map[string]*models.User{}
	defs := []seedUser{
		{"weijia", "微嘉", "在代码与文字之间，搭一座自己的知识花园。", models.RoleSuperAdmin},
		{"lin", "林间", "前端工程师，关心排版、留白与阅读体验。", models.RoleAdmin},
		{"chen", "陈默", "写 Go，做后端，偶尔翻译一些文章。", models.RoleUser},
		{"yue", "月白", "界面设计师，研究字、留白与动效。", models.RoleUser},
		{"zhou", "周行", "产品经理，业余做一档慢访谈播客。", models.RoleUser},
	}
	for i, d := range defs {
		u := models.User{
			Base:     models.Base{CreatedAt: now.AddDate(0, 0, -120+i*10), UpdatedAt: now},
			Username: d.username,
			Nickname: d.nickname,
			Email:    d.username + "@example.com",
			Password: string(hash),
			Bio:      d.bio,
			Role:     d.role,
		}
		if err := gdb.Create(&u).Error; err != nil {
			return err
		}
		users[d.username] = &u
	}

	categories := map[string]*models.Category{}
	for i, name := range []string{"工程", "设计", "阅读", "产品", "写作"} {
		category := models.Category{
			Base: models.Base{CreatedAt: now.AddDate(0, 0, -100), UpdatedAt: now},
			Name: name,
			Sort: i + 1,
		}
		if err := gdb.Create(&category).Error; err != nil {
			return err
		}
		categories[name] = &category
	}

	docs := []seedDoc{
		{
			key: "together", kind: models.KindArticle,
			title:    "为什么我把博客和文档放在一起",
			abstract: "两套系统、两张用户表、两份重复的评论与点赞。合并之后，内容只差一个 kind 字段。",
			author:   "weijia", tags: []string{"知识管理", "写作", "设计"},
			status: models.StatusPublished, daysAgo: 2, views: 386,
			content: md(`很多年里，我的写作分散在两个地方：博客记录想法，文档库存放手册。它们有各自的登录、评论、搜索，甚至各自维护一张用户表。

直到有一天，我想在文章里引用自己写过的一篇手册，才发现两边的内容互相看不见。

## 分开的代价

- 同一个读者要注册两次；
- 同一段评论逻辑写了两遍，修 bug 也要修两遍；
- 搜索只能搜到一半的自己。

> 分类是给读者看的，不是给数据库看的。

## 一个字段就够了

在数据层，博客文章和知识库文档真正的差别只有两点：

1. 文章是扁平的时间线，适合被订阅和讨论；
2. 文档是有层级的目录，适合被查阅和修订。

前者不需要 §parent_id§，后者需要。除此之外，标题、正文、作者、标签、点赞、收藏、评论，完全可以共用。

于是我把两张表合成一张 §documents§，用 §kind§ 区分 §article§ 与 §doc§。列表接口按 §kind§ 过滤，文档树接口只取 §kind = doc§ 的记录。

## 我删掉了什么

删掉的东西比想象中多：角色、文档密码、试看片段、审核流、收藏夹分组。它们曾经让系统看起来"完备"，但也让每一次写作都多出几步。

个人站点不需要一套权限系统，它需要的是：**打开就能写，写完就有人读**。

留下的是最基本的互动：评论、点赞、收藏、私信、关注。足够让读者留下痕迹，也不会把注意力从文字上挪开。`),
		},
		{
			key: "cover", kind: models.KindArticle,
			title:    "一张好的封面，为什么不需要设计",
			abstract: "一个图形、一层淡色、一小笔墨蓝。封面退到文字后面，只负责让列表有一点呼吸。",
			author:   "weijia", tags: []string{"设计", "生成式", "rough.js"},
			status: models.StatusPublished, daysAgo: 6, views: 521,
			content: md(`每次发文章，最费时间的往往不是写作，而是找封面。

图库的图片风格不一致，自己做的图又很难保持统一。于是我换了一个思路：**不选图，画一张**。

## 一条线就够了

现在每张封面只有三样东西：

- 一层很淡的纸色；
- 一个手绘几何图形：圆、波浪、三条短线，或者一个折线；
- 最多一小笔强调色。

没有标题，没有首字印章，也没有复杂纹样。封面退到文字的后面，只负责让列表有一点呼吸。

## 确定性

同一篇内容，应该永远得到同一张封面。前端用 rough.js 在 Canvas 上绘制，种子来自内容编号和类型：

~~~js
const seed = hash(§${doc.id}|${doc.kind}§)
const shape = seed % 4
~~~

rough.js 的 seed 让每一条线都有轻微偏移；同一个种子重绘一百次，图形也完全一样。

## 克制

生成式设计最容易犯的错，是把所有可能性都用上。我的约束是：

1. 一个画面只允许一个主图形；
2. 只从四组淡色纸面里选底色；
3. 除了墨色线条，只允许一小笔墨蓝。

## 头像同理

头像更简单：一个手绘圆，加上昵称的第一个字。底色同样由用户名哈希决定。没有上传，没有裁剪，也就没有不协调。

统一不是让所有东西长得一样，而是让所有东西看起来像来自同一个地方。`),
		},
		{
			key: "goserver", kind: models.KindArticle,
			title:    "用 Go 写一个足够小的个人站点",
			abstract: "一个路由文件、九张表、SQLite 和一个二进制文件，已经足够承载一个人的全部写作。",
			author:   "weijia", tags: []string{"Go", "后端", "工程"},
			status: models.StatusPublished, daysAgo: 10, views: 442,
			content: md(`这个站点没有微服务，没有消息队列，也没有独立的搜索集群。它是一个 Go 二进制文件，加上一个 SQLite 数据库。

## 技术选择

- **Gin** 负责路由，够快也够简单；
- **GORM** 负责九张表之间的琐碎关系；
- **SQLite** 放在一个文件里，备份就是 §cp§ 一下；
- **JWT** 无状态登录，服务端不存会话。

~~~go
func (a *API) Routes(r *gin.Engine) {
    g := r.Group("/api")
    g.GET("/docs", a.OptionalAuth(), a.DocList)
    g.GET("/docs/:id", a.OptionalAuth(), a.DocDetail)
    g.POST("/docs", a.Auth(), a.DocCreate)
}
~~~

## 九张表

§users§、§documents§、§likes§、§favorites§、§comments§、§comment_likes§、§follows§、§messages§、§notifications§。

没有角色表，没有权限表，没有文档与角色的关联表。权限只是 §users.role§ 上的一个字符串：§super_admin§、§admin§ 或 §user§。

## 为什么是 SQLite

个人站点的并发量，用 SQLite 绰绰有余。开启 WAL 之后，读不会阻塞写，写也不会阻塞读。真正的限制不是数据库，而是我一周能写几篇文章。

需要迁移到 MySQL 时，GORM 的模型不用改，换一个驱动即可。`),
		},
		{
			key: "css", kind: models.KindArticle,
			title:    "手写 CSS 的克制与自由",
			abstract: "不用组件库之后，我第一次清楚地知道页面上的每一像素从哪里来。",
			author:   "lin", tags: []string{"CSS", "前端", "设计"},
			status: models.StatusPublished, daysAgo: 4, views: 298,
			content: md(`组件库替我们做了很多决定：间距、圆角、阴影、断点。效率很高，但页面也越来越像别人的页面。

这次重做站点，我做了一个决定：**不引入任何组件库，CSS 全部手写**。

## 先定义语气

在写第一个组件之前，先确定几个问题：

- 这是纸，还是屏幕？——纸。
- 主色是蓝，还是红？——一笔墨蓝。
- 标题用黑体，还是宋体？——宋体，正文黑体。

这些答案最后变成一组变量：

~~~css
:root {
  --paper: #f6f2ea;
  --ink: #23201c;
  --accent: #2f4f9e;
  --serif: "Songti SC", Georgia, serif;
}
~~~

## 再做减法

每加一条规则，都问自己：它解决了什么问题？

- 卡片需要阴影吗？一层 1px 的线就够了，悬停时再加一点位移。
- 按钮需要三种尺寸吗？两种就够，大按钮只出现在首屏。
- 需要五个断点吗？两个，手机和桌面。

手写 CSS 的自由，不是想写什么就写什么，而是可以清楚地拒绝不需要的东西。

## 细节

一些很小的决定，最后决定了整体的气质：

- 正文行高 1.9，中文需要更多呼吸；
- 段落之间用 margin，不用空行；
- 链接的下划线偏离文字 4px；
- 深色模式下，纸色不是纯黑，而是偏暖的墨色。

克制不是少做，而是每一个选择都有理由。`),
		},
		{
			key: "cards", kind: models.KindArticle,
			title:    "读《卡片笔记写作法》",
			abstract: "重要的不是卡片，而是让想法之间发生连接。",
			author:   "chen", tags: []string{"阅读", "笔记"},
			status: models.StatusPublished, daysAgo: 14, views: 176,
			content: md(`这本书被引用得太多，以至于很容易被当成一种工具书。但真正有价值的部分，不是"怎么记卡片"，而是"为什么要连接"。

## 三种笔记

- **闪念笔记**：随手记，之后必须处理，否则就是垃圾；
- **文献笔记**：读完之后用自己的话复述，附上出处；
- **永久笔记**：一张卡片只讲一个想法，并且主动寻找它可以连接的旧卡片。

## 连接比分类重要

传统笔记法喜欢建文件夹：技术、生活、阅读。但一个想法往往同时属于好几个文件夹。

卡片盒的做法是给每张卡片编号，然后用链接把它们串起来。分类是树，连接是网。树适合查找，网适合思考。

## 我的实践

我把这个站点当成一个公开的卡片盒：

- 文章是较长的永久笔记；
- 文档是经过整理的索引；
- 标签是临时的入口，不是最终的分类。

写完一篇文章，我会问自己：它可以连接到哪篇旧文章？如果找不到，也许它还不够成熟。`),
		},
		{
			key: "light", kind: models.KindArticle,
			title:    "把评论、私信和关注做轻一点",
			abstract: "互动功能的目标不是留住用户，而是让一次交流顺畅地发生，然后结束。",
			author:   "weijia", tags: []string{"产品", "交互"},
			status: models.StatusPublished, daysAgo: 8, views: 233,
			content: md(`个人站点最容易被"功能完整"绑架：消息中心、未读红点、通知设置、免打扰、拉黑、举报……最后做出来一个小型社交平台，却没有几个人使用。

这次我给自己定了三条规则。

## 一、只有两层评论

根评论和回复，足够覆盖绝大多数对话。不做无限嵌套，不做楼层引用，不做评论置顶。

回复时显示"回复 @某某"，剩下的交给时间顺序。

## 二、私信不做实时

没有 WebSocket，没有"对方正在输入"。打开对话时拉取最近 50 条，发送后重新拉取。

私信的价值在于内容，不在于即时。真正紧急的事情，人们不会用站内信。

## 三、通知只保留三种

- 有人赞了你的内容；
- 有人评论或回复了你；
- 有人关注了你。

收藏不通知，取消不通知，浏览不通知。通知越多，越没有人看。

## 最后

互动功能应该像标点符号：需要的时候在那里，不需要的时候不会打断阅读。`),
		},
		{
			key: "draft", kind: models.KindArticle,
			title:    "关于知识树的一则草稿",
			abstract: "还没想清楚，先放在这里。",
			author:   "weijia", tags: []string{"草稿"},
			status: models.StatusDraft, daysAgo: 1, views: 0,
			content: md(`知识树是不是一个过于整齐的隐喻？

树有主干、分支、叶子，但真实的知识更像菌丝：到处蔓延，没有中心。

也许文档的层级只是给读者的一条路径，而不是知识本身的结构……`),
		},
		{
			key: "handbook", kind: models.KindDoc, title: "工程手册",
			abstract: "这个站点背后的约定：目录、接口、数据模型与部署。",
			author:   "weijia", tags: []string{"工程", "索引"},
			status: models.StatusPublished, daysAgo: 30, views: 210, sort: 1,
			content: md(`这里记录知微的工程约定。

- **服务端**：Go + Gin + GORM，默认 SQLite。
- **前端**：Vue 3 + Vite，纯手写 CSS。
- **视觉**：前端用 rough.js 绘制封面与头像，服务端提供同风格的 SVG 兜底。

左侧目录按文档层级自动生成。新增文档时，在编辑器里选择"上级文档"即可。`),
		},
		{
			key: "goconv", kind: models.KindDoc, title: "Go 服务端约定", parent: "handbook",
			abstract: "路由、错误返回、鉴权与时间字段的统一约定。",
			author:   "weijia", tags: []string{"Go", "规范"},
			status: models.StatusPublished, daysAgo: 28, views: 168, sort: 1,
			content: md(`## 路由

所有接口挂在 §/api§ 下。公开读取、可选登录、必须登录分成三类：

- §OptionalAuth§：详情页需要知道"我是否点过赞"；
- §Auth§：登录用户的评论、点赞、收藏、私信等互动；
- §StaffOnly§：发布和修改内容，仅超级管理员与管理员可用；
- §SuperAdminOnly§：用户与角色管理。

## 错误返回

统一使用 HTTP 状态码，响应体为 §{"error": "中文说明"}§。

前端在 §api.js§ 里统一拦截，抛出带状态码的 §ApiError§。

## 返回结构

- 列表：§{items, total, page, pageSize}§
- 详情：按资源包一层，如 §{doc, liked, favorited}§
- 动作：§{ok: true}§ 或动作后的状态

## 时间

数据库存 UTC，接口返回 RFC3339，前端统一转成相对时间：刚刚、3 小时前、9 月 12 日。`),
		},
		{
			key: "errors", kind: models.KindDoc, title: "错误处理与返回格式", parent: "goconv",
			abstract: "什么时候用 400，什么时候用 404，以及为什么不要返回 200 包一个 code。",
			author:   "chen", tags: []string{"Go", "API"},
			status: models.StatusPublished, daysAgo: 25, views: 92, sort: 1,
			content: md(`## 状态码

| 场景 | 状态码 |
| --- | --- |
| 参数不合法 | 400 |
| 未登录 | 401 |
| 无权限 | 403 |
| 资源不存在 | 404 |
| 服务端异常 | 500 |

## 不要用 200 包裹错误

§{"code": 401, "msg": "请先登录"}§ 会让浏览器、缓存、监控全部失去判断能力。

HTTP 已经有一套语义，直接用就好。前端的拦截器只需要看 §res.ok§。

## 一层包装

成功时返回资源本身或一个明确的对象，不额外套 §data§。这样接口文档最短，前端解构也最直接。`),
		},
		{
			key: "models", kind: models.KindDoc, title: "数据模型速查", parent: "handbook",
			abstract: "九张表、它们的关系，以及为什么不需要角色表。",
			author:   "weijia", tags: []string{"数据库", "GORM"},
			status: models.StatusPublished, daysAgo: 24, views: 143, sort: 2,
			content: md(`## 核心表

### users

§id, username, nickname, email, password, bio, role, last_login_at§

角色只有三个值：§super_admin§、§admin§ 与 §user§。第一个注册的用户自动成为超级管理员，之后由超级管理员设置管理员。只有前两种角色可以发布内容。

### documents

§id, kind, title, abstract, content, author_id, parent_id, tags, status, pinned, sort§

§kind§ 取 §article§ 或 §doc§。文章的 §parent_id§ 恒为空；文档通过它组成树。

### 互动表

- §likes(user_id, doc_id)§ 唯一索引
- §favorites(user_id, doc_id)§ 唯一索引
- §comments(doc_id, user_id, parent_id)§ 两层
- §comment_likes(user_id, comment_id)§
- §follows(follower_id, followee_id)§
- §messages(sender_id, receiver_id, content, read_at)§
- §notifications(user_id, actor_id, type, doc_id, read)§

## 计数字段

§documents§ 上保存 §view_count§、§like_count§、§comment_count§、§favorite_count§。

写入互动记录后，用一次 §COUNT(*)§ 重新计算对应计数。个人站点的数据量下，这比维护增量更不容易出错。`),
		},
		{
			key: "deploy", kind: models.KindDoc, title: "部署与备份", parent: "handbook",
			abstract: "编译前端、打包二进制、一个 systemd 服务和一个备份脚本。",
			author:   "weijia", tags: []string{"部署", "运维"},
			status: models.StatusPublished, daysAgo: 20, views: 87, sort: 3,
			content: md(`## 构建

~~~bash
cd web && npm install && npm run build
cd ../server && go build -o folio .
~~~

Go 服务会自动托管 §../web/dist§。也可以用 §FOLIO_WEB§ 指定目录。

## 运行

~~~bash
FOLIO_ADDR=:8080 \
FOLIO_DB=/var/lib/folio/folio.db \
FOLIO_SECRET=换成一个足够长的随机串 \
FOLIO_DEMO=0 \
./folio
~~~

## systemd

~~~ini
[Unit]
Description=folio
After=network.target

[Service]
WorkingDirectory=/opt/folio
EnvironmentFile=/opt/folio/.env
ExecStart=/opt/folio/folio
Restart=always

[Install]
WantedBy=multi-user.target
~~~

## 备份

SQLite 开启 WAL 之后，直接复制主库文件之前先执行：

~~~bash
sqlite3 folio.db ".backup '/backup/folio-$(date +%F).db'"
~~~

再用 §rsync§ 同步到另一台机器。每天一次，保留 30 天。`),
		},
		{
			key: "design", kind: models.KindDoc, title: "设计系统", parent: "",
			abstract: "颜色、字体、间距与组件的最小集合。",
			author:   "lin", tags: []string{"设计", "CSS"},
			status: models.StatusPublished, daysAgo: 22, views: 195, sort: 2,
			content: md(`## 语气

纸感、克制、偏文学。留白多，线条细，强调色只出现在需要被点击或需要被记住的地方。

## 颜色

| 名称 | 亮色 | 暗色 |
| --- | --- | --- |
| paper | §#f4f3ee§ | §#1a1a18§ |
| surface | §#fbfaf5§ | §#21211e§ |
| ink | §#1e1e1a§ | §#ebe8de§ |
| accent | §#2f4f9e§ | §#8fa8e0§ |

## 字体

- 标题：Hannotate SC / Kaiti SC
- 正文：PingFang SC / Noto Sans CJK
- 代码：SF Mono / Menlo

正文 15.5px，行高 1.75；标题使用 §clamp()§ 在 28–56px 之间自适应。

## 间距

以 4px 为基数，常用 8、12、16、24、40、64。区块之间的垂直间距不小于 64px。

## 组件

按钮、输入框、标签、卡片、头像、封面、分页、评论、目录、编辑器。

全部手写，不引入第三方组件库。`),
		},
		{
			key: "colors", kind: models.KindDoc, title: "颜色与排版", parent: "design",
			abstract: "为什么选纸色而不是纯白，为什么中文正文需要更大的行高。",
			author:   "lin", tags: []string{"CSS", "排版"},
			status: models.StatusPublished, daysAgo: 21, views: 118, sort: 1,
			content: md(`## 纸色

纯白背景在夜间显得刺眼，也会让文字对比过强。§#f6f2ea§ 是一种偏暖的纸色，长时间阅读更舒服。

暗色模式不用纯黑：§#1b1917§ 带一点棕，像旧书页在灯下的影子。

## 中文行高

中文没有西文那样明显的单词间距，行与行之间需要更多空间。正文行高 1.9，段落间距 1.2em。

## 字重

中文黑体的粗体在屏幕上容易糊成一片。标题用宋体 + 600，正文只用 400，需要强调时改变颜色或加下划线，而不是加粗。

## 行宽

正文最大宽度 720px。超过这个宽度，眼睛从行尾回到行首会变得吃力。

## 对齐

左对齐，不使用两端对齐。中文两端对齐会在标点处产生奇怪的缝隙。`),
		},
		{
			key: "components", kind: models.KindDoc, title: "组件清单", parent: "design",
			abstract: "站点需要的全部组件，以及每一个组件的状态。",
			author:   "lin", tags: []string{"前端", "组件"},
			status: models.StatusPublished, daysAgo: 18, views: 76, sort: 2,
			content: md(`## 基础

- Button：primary / ghost / quiet，尺寸 sm / md
- Input / Textarea / Select
- Tag：可点击、可删除
- Avatar：由用户名生成
- Cover：由标题与类型生成

## 内容

- DocCard：网格与列表两种布局
- Markdown：标题锚点、代码块复制、图片自适应
- CommentList：根评论 + 回复，两层
- Toc：文档右侧目录，滚动高亮

## 反馈

- Toast：成功、错误、普通三种
- Empty：空状态，带一句解释
- Loading：骨架或文字，不出现转圈图标

## 布局

- SiteHeader：吸顶、半透明、移动端抽屉
- SiteFooter：品牌、链接、版权
- DocsLayout：左侧目录 + 右侧正文
- EditorLayout：编辑与预览分栏

所有组件不使用 UI 框架，样式集中在四个 CSS 文件里：base、components、pages、markdown。`),
		},
		{
			key: "reading", kind: models.KindDoc, title: "阅读清单", parent: "",
			abstract: "正在读、读完和想读的书，以及一句短评。",
			author:   "chen", tags: []string{"阅读", "清单"},
			status: models.StatusPublished, daysAgo: 16, views: 64, sort: 3,
			content: md(`## 2026 上半年

- 《卡片笔记写作法》——连接比分类重要。
- 《设计中的设计》——留白也是内容。
- 《Go 语言设计与实现》——读第二遍，终于看懂了调度器。
- 《写作这回事》——把副词删掉。

## 想读

- 《The Art of Noticing》
- 《A Philosophy of Software Design》
- 《中文排版需求》

读完一本，会把它整理成一篇文档，而不是只留一句"已读"。`),
		},
		{
			key: "reading2026", kind: models.KindDoc, title: "2026 上半年", parent: "reading",
			abstract: "四本书，四句短评，以及它们留下的问题。",
			author:   "chen", tags: []string{"阅读"},
			status: models.StatusPublished, daysAgo: 15, views: 41, sort: 1,
			content: md(`## 卡片笔记写作法

留下的问题：如何避免卡片盒变成另一个垃圾场？

## 设计中的设计

原研哉谈"空"，其实谈的是注意力。页面上的留白不是为了好看，而是为了让剩下的东西被看见。

## Go 语言设计与实现

GMP 调度、内存分配、垃圾回收。读完最大的收获不是记住细节，而是知道遇到性能问题时该看哪里。

## 写作这回事

"通往地狱的路是副词铺成的。"写技术文章同样适用：删掉"非常"、"十分"、"其实"。`),
		},
		{
			key: "infoarch", kind: models.KindArticle,
			title:    "给个人站点做一次信息架构复盘",
			abstract: "上线半年后，我把导航从五个入口减到三个，也第一次看清自己到底在写什么。",
			author:   "weijia", tags: []string{"信息架构", "设计", "产品"},
			status: models.StatusPublished, daysAgo: 5, views: 187,
			content: md(`站点上线半年，后台的标签越加越多，导航却越来越难用。

## 三个入口

我把访客的来意分成三类：

1. 想随便读点什么；
2. 想找一篇具体的文章；
3. 想查一份长期维护的资料。

对应三个入口：首页、文章、文档。其他东西全部收进页面内部。

## 我改了什么

- 把"热门""归档""关于"从导航移走；
- 标签从筛选器变成搜索入口；
- 文档树只在知识库内部出现。

## 还没解决

搜索仍然只是关键词匹配，找不到"我记得他写过一篇讲留白的文章"这种模糊记忆。也许下一步该给每篇文档加一句手写索引。`),
		},
		{
			key: "whitespace", kind: models.KindArticle,
			title:    "留白不是空白",
			abstract: "页面上的空隙不是没放东西，而是在替读者分配注意力。",
			author:   "yue", tags: []string{"设计", "排版", "CSS"},
			status: models.StatusPublished, daysAgo: 3, views: 274,
			content: md(`做设计评审时，最常听到的一句话是："这里是不是有点空？"

但空和留白不是一回事。

## 留白在做什么

- 把相关的东西放在一起；
- 把不相关的东西分开；
- 给视线一个可以停下来的地方。

## 三种留白

**结构留白**：区块之间的距离，决定页面的节奏。

**呼吸留白**：行高、段距、内边距，决定阅读的舒适度。

**情绪留白**：首屏大面积的空，决定第一眼的感受。

## 一个练习

把页面上所有装饰删掉，只保留文字和间距。如果此时仍然好读，说明结构成立；如果散掉了，再考虑加回东西。

留白不是没有内容，它是内容的停顿。`),
		},
		{
			key: "reader", kind: models.KindArticle,
			title:    "阅读器、笔记本与第二大脑",
			abstract: "工具换了十几轮之后，我留下了三个盒子：收集、消化、输出。",
			author:   "zhou", tags: []string{"阅读", "工具", "笔记"},
			status: models.StatusPublished, daysAgo: 12, views: 205,
			content: md(`我曾同时用着三个阅读器、两个笔记软件和一个稍后读工具。

## 三个盒子

现在只剩下三个动作：

1. **收集**：看到就丢进一个地方，不分类；
2. **消化**：读完写三句话，自己的话；
3. **输出**：三句话长成一段，或一篇。

## 工具不重要

重要的是盒子之间的通道是否通畅。

如果一条链接从收集到输出需要点击五次、切换三个应用，它最后一定死在收件箱里。

## 我的通道

- 网页 → 稍后读；
- 稍后读 → 摘录三句；
- 摘录 → 这个站点的草稿；
- 草稿 → 文章。

每一步只做一件事，但每一步都必须发生。`),
		},
		{
			key: "podcast", kind: models.KindArticle,
			title:    "做播客之后，我重新学会了提问",
			abstract: "好的问题不是显得聪明，而是让对方愿意多说一句。",
			author:   "zhou", tags: []string{"播客", "写作", "访谈"},
			status: models.StatusPublished, daysAgo: 7, views: 149,
			content: md(`录到第十期，我才意识到自己前九期都在问"准备过的问题"。

## 提问比回答难

准备问题时，容易犯两个错误：

- 问题太大，对方只能给一个概括；
- 问题太满，把答案也一起说了。

## 我现在怎么准备

每期只准备五个问题，每个问题后面留一句"然后呢"。

真正的内容往往出现在第三句之后，而不是第一句。

## 剪辑

剪辑时删掉自己的"嗯""对""是的"，也删掉那些让嘉宾重复问题的开场。留下呼吸，但不要留下犹豫。

一期节目四十分钟，准备四个小时。提问的功夫都在看不见的地方。`),
		},
		{
			key: "draftlin", kind: models.KindArticle,
			title:    "组件库之后",
			abstract: "还没写完的思考。",
			author:   "lin", tags: []string{"前端", "草稿"},
			status: models.StatusDraft, daysAgo: 2, views: 0,
			content: md(`组件库解决了重复劳动，也把审美一起标准化了。

手写 CSS 之后，我才开始注意按钮的高度为什么是 40 而不是 42。

这篇还没想清楚，先放着。`),
		},
		{
			key: "motion", kind: models.KindDoc, title: "动效与节奏", parent: "design",
			abstract: "什么时候该动，动多久，以及什么情况下不要动。",
			author:   "yue", tags: []string{"设计", "动效"},
			status: models.StatusPublished, daysAgo: 11, views: 61, sort: 3,
			content: md(`## 动效的三个作用

1. 说明因果关系：点了一个按钮，页面为什么变了；
2. 保持空间连续：从列表进入详情，读者知道自己在哪；
3. 提供反馈：操作被接收了。

如果一个动画同时做不到这三件事中的一件，它就不该存在。

## 时长

- 微反馈：80–120ms；
- 面板展开：180–240ms；
- 页面转场：260–320ms。

超过 400ms 的动画，只有在讲故事时才合理。

## 缓动

进入用 ease-out，离开用 ease-in。对称的 ease-in-out 适合循环动画，不适合单向转场。

## 不要动

- 尊重 §prefers-reduced-motion§；
- 滚动时的视差效果，除非内容真的需要；
- 任何超过屏幕三分之一的位移。`),
		},
		{
			key: "interaction", kind: models.KindDoc, title: "交互札记", parent: "",
			abstract: "那些不值得写成长文，但值得记下来的小决定。",
			author:   "yue", tags: []string{"交互", "设计"},
			status: models.StatusPublished, daysAgo: 13, views: 84, sort: 4,
			content: md(`这里记录一些零散的交互观察。

- 按钮文案尽量用动词；
- 删除操作要么可撤销，要么二次确认，不要两者都没有；
- 空状态不是"暂无数据"，而是"接下来可以做什么"；
- 加载超过 300ms 才需要 loading，超过 1s 才需要骨架屏。`),
		},
		{
			key: "emptystate", kind: models.KindDoc, title: "空状态怎么写", parent: "interaction",
			abstract: "三句话：发生了什么、为什么、下一步。",
			author:   "yue", tags: []string{"交互", "文案"},
			status: models.StatusPublished, daysAgo: 9, views: 46, sort: 1,
			content: md(`## 三句话

1. **发生了什么**：你还没有收藏。
2. **为什么**：收藏的内容只有自己能看到，用来稍后重读。
3. **下一步**：去文章列表看看，点一下书签图标。

## 反例

- "暂无数据"——读者不知道是没有，还是加载失败；
- "赶紧去收藏吧！"——命令式，且没有解释；
- 一大段功能说明——读者此刻只想离开这个页面。

## 语气

空状态是产品少有的、可以安静说话的地方。用完整的句子，不用感叹号。`),
		},
		{
			key: "reading2025", kind: models.KindDoc, title: "2025 冬天", parent: "reading",
			abstract: "那个冬天读得很少，但记住了三句话。",
			author:   "zhou", tags: []string{"阅读"},
			status: models.StatusPublished, daysAgo: 40, views: 38, sort: 2,
			content: md(`## 三句话

- "注意力是唯一不可再生的资源。"
- "写作是把模糊的感觉逼成明确的句子。"
- "慢下来不是效率的反面，而是效率的前提。"

## 书目

- 《深度工作》
- 《写作这回事》
- 《置身事内》

那个冬天没有读完任何一本，但这三句话留下来了。`),
		},
	}

	byKey := map[string]*models.Document{}
	for i, sd := range seedDocList(docs) {
		created := now.AddDate(0, 0, -sd.daysAgo).Add(-time.Duration(i) * 37 * time.Minute)
		d := models.Document{
			Base:      models.Base{CreatedAt: created, UpdatedAt: created},
			Kind:      sd.kind,
			Title:     sd.title,
			Abstract:  sd.abstract,
			Content:   sd.content,
			AuthorID:  users[sd.author].ID,
			Tags:      sd.tags,
			Status:    sd.status,
			Sort:      sd.sort,
			ViewCount: sd.views,
			Pinned:    sd.key == "together",
		}
		if sd.status == models.StatusPublished {
			t := created
			d.PublishedAt = &t
		}
		if sd.parent != "" {
			if p, ok := byKey[sd.parent]; ok {
				pid := p.ID
				d.ParentID = &pid
			}
		}
		if err := gdb.Create(&d).Error; err != nil {
			return err
		}
		byKey[sd.key] = &d
	}

	articleCategories := map[string]string{
		"together": "写作", "cover": "设计", "goserver": "工程", "css": "设计",
		"cards": "阅读", "light": "产品", "infoarch": "产品", "whitespace": "设计",
		"reader": "阅读", "podcast": "写作",
	}
	for key, categoryName := range articleCategories {
		article, ok := byKey[key]
		if !ok {
			continue
		}
		category, ok := categories[categoryName]
		if !ok {
			continue
		}
		article.CategoryID = &category.ID
		gdb.Model(article).UpdateColumn("category_id", category.ID)
	}

	doc := func(key string) *models.Document { return byKey[key] }
	user := func(name string) *models.User { return users[name] }

	type likeSeed struct{ user, doc string }
	likes := []likeSeed{
		{"lin", "together"}, {"lin", "cover"}, {"lin", "goserver"}, {"lin", "css"},
		{"chen", "together"}, {"chen", "cards"}, {"chen", "light"},
		{"weijia", "css"}, {"weijia", "design"},
		{"lin", "design"}, {"lin", "handbook"}, {"chen", "models"},
		{"lin", "whitespace"}, {"chen", "whitespace"}, {"weijia", "whitespace"},
		{"weijia", "reader"}, {"yue", "reader"}, {"lin", "reader"},
		{"zhou", "podcast"}, {"weijia", "infoarch"}, {"yue", "infoarch"},
		{"weijia", "motion"}, {"yue", "design"}, {"zhou", "together"},
		{"chen", "emptystate"}, {"lin", "emptystate"},
	}
	for _, l := range likes {
		d := doc(l.doc)
		if d == nil {
			continue
		}
		gdb.Create(&models.Like{Base: models.Base{CreatedAt: now.Add(-48 * time.Hour)}, UserID: user(l.user).ID, DocID: d.ID})
	}
	favs := []likeSeed{
		{"lin", "cover"}, {"chen", "together"}, {"chen", "light"}, {"weijia", "components"},
		{"yue", "reader"}, {"zhou", "reader"}, {"lin", "whitespace"}, {"chen", "motion"},
		{"weijia", "reader"},
	}
	for _, f := range favs {
		d := doc(f.doc)
		if d == nil {
			continue
		}
		gdb.Create(&models.Favorite{Base: models.Base{CreatedAt: now.Add(-24 * time.Hour)}, UserID: user(f.user).ID, DocID: d.ID})
	}

	// 评论：一篇根评论 + 两条回复
	root := models.Comment{
		Base: models.Base{CreatedAt: now.Add(-40 * time.Hour)}, DocID: doc("together").ID,
		UserID: user("lin").ID, Content: "看到“分类是给读者看的，不是给数据库看的”这句，突然想通了自己项目里那些奇怪的分类字段。",
	}
	gdb.Create(&root)
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-38 * time.Hour)}, DocID: doc("together").ID,
		UserID: user("weijia").ID, Content: "对，很多结构其实是历史遗留，不是内容真的需要。",
		ParentID: &root.ID, ReplyToName: "林间",
	})
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-30 * time.Hour)}, DocID: doc("together").ID,
		UserID: user("chen").ID, Content: "从后端角度看，少一张表真的能少很多同步逻辑。",
	})
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-22 * time.Hour)}, DocID: doc("cover").ID,
		UserID: user("lin").ID, Content: "“统一不是让所有东西长得一样，而是让所有东西看起来像来自同一个地方。”收藏了。",
	})

	spaceRoot := models.Comment{
		Base: models.Base{CreatedAt: now.Add(-18 * time.Hour)}, DocID: doc("whitespace").ID,
		UserID: user("lin").ID, Content: "留白和空状态确实是同一件事的两面：一个处理空间，一个处理预期。",
	}
	gdb.Create(&spaceRoot)
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-16 * time.Hour)}, DocID: doc("whitespace").ID,
		UserID: user("yue").ID, Content: "对，空状态其实是“内容缺席时的留白”。",
		ParentID: &spaceRoot.ID, ReplyToName: "林间",
	})
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-14 * time.Hour)}, DocID: doc("reader").ID,
		UserID: user("weijia").ID, Content: "“盒子之间的通道”这个说法很准确。我的问题往往不是盒子太少，而是通道太长。",
	})
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-11 * time.Hour)}, DocID: doc("infoarch").ID,
		UserID: user("zhou").ID, Content: "三个入口的划分很清楚。我自己的站还停在“全部塞进导航”的阶段。",
	})
	gdb.Create(&models.Comment{
		Base: models.Base{CreatedAt: now.Add(-9 * time.Hour)}, DocID: doc("motion").ID,
		UserID: user("lin").ID, Content: "时长那组数字很实用，尤其是微反馈 80–120ms。",
	})

	// 关注
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-72 * time.Hour)}, FollowerID: user("lin").ID, FolloweeID: user("weijia").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-60 * time.Hour)}, FollowerID: user("chen").ID, FolloweeID: user("weijia").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-50 * time.Hour)}, FollowerID: user("weijia").ID, FolloweeID: user("lin").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-44 * time.Hour)}, FollowerID: user("yue").ID, FolloweeID: user("weijia").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-36 * time.Hour)}, FollowerID: user("zhou").ID, FolloweeID: user("weijia").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-32 * time.Hour)}, FollowerID: user("weijia").ID, FolloweeID: user("yue").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-28 * time.Hour)}, FollowerID: user("lin").ID, FolloweeID: user("yue").ID})
	gdb.Create(&models.Follow{Base: models.Base{CreatedAt: now.Add(-24 * time.Hour)}, FollowerID: user("zhou").ID, FolloweeID: user("yue").ID})

	// 私信
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-6 * time.Hour)}, SenderID: user("lin").ID, ReceiverID: user("weijia").ID,
		Content: "封面生成器那段代码有开源吗？想用在自己的站点里。",
	})
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-5 * time.Hour)}, SenderID: user("weijia").ID, ReceiverID: user("lin").ID,
		Content: "就在这个项目里，server/art/art.go。可以直接拿走，记得换一组调色板。",
	})
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-3 * time.Hour)}, SenderID: user("lin").ID, ReceiverID: user("weijia").ID,
		Content: "好，周末试试。另外文档树的侧边栏也很好用。",
	})
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-26 * time.Hour)}, SenderID: user("chen").ID, ReceiverID: user("weijia").ID,
		Content: "数据模型那篇我补充了一条评论，有空看看。",
	})
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-4 * time.Hour)}, SenderID: user("yue").ID, ReceiverID: user("weijia").ID,
		Content: "首页的排版我很喜欢，动效那篇要不要一起写？",
	})
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-2 * time.Hour)}, SenderID: user("weijia").ID, ReceiverID: user("yue").ID,
		Content: "好啊，我先把结构列出来，你来补动效部分。",
	})
	gdb.Create(&models.Message{
		Base: models.Base{CreatedAt: now.Add(-8 * time.Hour)}, SenderID: user("zhou").ID, ReceiverID: user("weijia").ID,
		Content: "播客那篇可以转载到我的 newsletter 吗？会附上原文链接。",
	})

	// 通知
	docTogether := doc("together")
	gdb.Create(&models.Notification{
		Base: models.Base{CreatedAt: now.Add(-39 * time.Hour)}, UserID: user("weijia").ID,
		ActorID: user("lin").ID, Type: models.NotifyComment, DocID: &docTogether.ID,
		DocKind: docTogether.Kind, DocTitle: docTogether.Title, Content: "看到“分类是给读者看的”这句……",
	})
	gdb.Create(&models.Notification{
		Base: models.Base{CreatedAt: now.Add(-20 * time.Hour)}, UserID: user("weijia").ID,
		ActorID: user("chen").ID, Type: models.NotifyFollow,
	})
	docCover := doc("cover")
	gdb.Create(&models.Notification{
		Base: models.Base{CreatedAt: now.Add(-12 * time.Hour)}, UserID: user("weijia").ID,
		ActorID: user("lin").ID, Type: models.NotifyLike, DocID: &docCover.ID,
		DocKind: docCover.Kind, DocTitle: docCover.Title, Read: true,
	})
	docWhitespace := doc("whitespace")
	gdb.Create(&models.Notification{
		Base: models.Base{CreatedAt: now.Add(-18 * time.Hour)}, UserID: user("yue").ID,
		ActorID: user("lin").ID, Type: models.NotifyComment, DocID: &docWhitespace.ID,
		DocKind: docWhitespace.Kind, DocTitle: docWhitespace.Title, Content: "留白和空状态确实是同一件事的两面……",
	})
	gdb.Create(&models.Notification{
		Base: models.Base{CreatedAt: now.Add(-30 * time.Hour)}, UserID: user("weijia").ID,
		ActorID: user("zhou").ID, Type: models.NotifyFollow,
	})
	docReader := doc("reader")
	gdb.Create(&models.Notification{
		Base: models.Base{CreatedAt: now.Add(-14 * time.Hour)}, UserID: user("zhou").ID,
		ActorID: user("weijia").ID, Type: models.NotifyComment, DocID: &docReader.ID,
		DocKind: docReader.Kind, DocTitle: docReader.Title, Content: "“盒子之间的通道”这个说法很准确……",
	})

	// 重新计算计数，保证互动与数字一致
	for _, d := range byKey {
		var likeCount, favCount, commentCount int64
		gdb.Model(&models.Like{}).Where("doc_id = ?", d.ID).Count(&likeCount)
		gdb.Model(&models.Favorite{}).Where("doc_id = ?", d.ID).Count(&favCount)
		gdb.Model(&models.Comment{}).Where("doc_id = ?", d.ID).Count(&commentCount)
		gdb.Model(&models.Document{}).Where("id = ?", d.ID).Updates(map[string]any{
			"like_count":     likeCount,
			"favorite_count": favCount,
			"comment_count":  commentCount,
			"updated_at":     d.UpdatedAt,
		})
	}
	log.Printf("演示数据已写入：%d 位用户、%d 篇内容；账号 weijia / folio1234", len(users), len(byKey))
	return nil
}

// seedDocList 保证父文档先于子文档创建。
func seedDocList(list []seedDoc) []seedDoc {
	out := make([]seedDoc, 0, len(list))
	added := map[string]bool{}
	var add func(d seedDoc)
	add = func(d seedDoc) {
		if added[d.key] {
			return
		}
		if d.parent != "" && !added[d.parent] {
			for _, p := range list {
				if p.key == d.parent {
					add(p)
					break
				}
			}
		}
		added[d.key] = true
		out = append(out, d)
	}
	for _, d := range list {
		add(d)
	}
	return out
}

// EnsureDailyStats 在演示模式下为历史阅读量补一份近 30 天的展示数据。
// 只在统计表为空时执行，不会覆盖真实访问数据；FOLIO_DEMO=0 时不会调用。
func EnsureDailyStats(gdb *gorm.DB) error {
	var count int64
	if err := gdb.Model(&models.DocDailyStat{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var docs []models.Document
	if err := gdb.Where("status = ? AND view_count > 0", models.StatusPublished).Find(&docs).Error; err != nil {
		return err
	}
	now := time.Now()
	stats := make([]models.DocDailyStat, 0, len(docs)*30)
	for _, doc := range docs {
		weights := make([]int, 30)
		sum := 0
		for i := 0; i < 30; i++ {
			weight := 20 + int((uint64(doc.ID)*37+uint64(i)*19+uint64(doc.ViewCount))%53)
			if i > 22 {
				weight += 18
			}
			weights[i] = weight
			sum += weight
		}
		remaining := doc.ViewCount
		for i := 0; i < 30; i++ {
			views := doc.ViewCount * weights[i] / sum
			if i == 29 {
				views = remaining
			}
			if views > remaining {
				views = remaining
			}
			remaining -= views
			if views <= 0 {
				continue
			}
			stats = append(stats, models.DocDailyStat{
				DocID: doc.ID,
				Day:   now.AddDate(0, 0, i-29).Format("2006-01-02"),
				Views: views,
			})
		}
	}
	if len(stats) == 0 {
		return nil
	}
	return gdb.CreateInBatches(&stats, 100).Error
}
