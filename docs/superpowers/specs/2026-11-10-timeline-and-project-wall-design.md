# 时间线与项目墙设计

## 目标

为知微增加两个公开入口：

1. `/timeline` 发文时间线：按时间回看已经发布的内容。
2. `/projects` 项目墙：展示项目，并可导流到项目主页或 GitHub 仓库。

同时保留上一轮已完成的阅读布局优化：收起目录或侧栏后，正文宽度从阅读宽度上限扩展到更宽的可读宽度。

## 非目标

- 不把项目做成文章或文档，不复用 `Document` 表。
- 不引入复杂项目管理（成员、里程碑、版本等）。
- 不强制项目上传封面；没有封面时使用站内纸质卡片样式。

## 时间线

- 新接口 `GET /api/timeline?kind=all|article|doc&page=1&pageSize=30`。
- 只返回已发布内容；文档额外排除 `search_hidden`、私密文档和无权限内容。
- 按 `COALESCE(published_at, created_at)` 倒序。
- 复用 `Document` 数据和作者信息，不新增表。
- 页面按年份、月份分组；顶部可切换全部 / 文章 / 文档；支持加载更多。
- 导航栏、页脚加入口，sitemap 增加 `/timeline`。

## 项目墙

- 新表 `projects`：标题、简介、项目主页、仓库地址、可选封面、标签、是否公开、排序。
- 公开接口 `GET /api/projects` 只返回公开项目，按排序和 ID 排序。
- 后台接口：
  - `GET /api/admin/projects`
  - `POST /api/admin/projects`
  - `PUT /api/admin/projects/:id`
  - `DELETE /api/admin/projects/:id`
- 后台新增「项目管理」标签，支持增删改、排序和公开开关。
- 公开页为卡片墙；有封面用封面，没有封面使用带编号的纸质卡片。
- 每张卡片按需显示「访问项目」和「GitHub」按钮。
- 导航栏、页脚加入口，sitemap 增加 `/projects`。

## 实现约束

- 沿用现有 Go + Gin + GORM + Vue 3 结构。
- 项目 URL 只接受 `http://` 或 `https://`，空值允许。
- 标签复用现有 `normalizeTags`，最多 6 个。
- 所有后台写操作记录操作日志。
- 不改变现有内容表、搜索、AI 检索和阅读权限逻辑。
