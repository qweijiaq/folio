# Timeline and Project Wall Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use subagent-driven-development (recommended) or executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a public publishing timeline and a project wall with admin management.

**Architecture:** Timeline is a read-only projection of existing `Document` rows. Projects get a small `projects` table and staff CRUD. Both get public Vue pages, navigation entries, SEO metadata and sitemap entries.

**Tech Stack:** Go, Gin, GORM, SQLite/MySQL, Vue 3, Vue Router, Vitest.

**Spec:** `docs/superpowers/specs/2026-11-10-timeline-and-project-wall-design.md`

## Global Constraints

- Public timeline only shows published content.
- `search_hidden` docs and private docs never appear on the timeline.
- Project URLs only accept `http://` or `https://`; empty is allowed.
- Project tags use existing `normalizeTags`.
- All project writes call `logAction`.
- Mobile navigation and desktop navigation must both expose the new pages.
- Reading-width expansion for collapsed sidebars remains intact.

## Review Focus

- Timeline query must not leak draft, private, or search-hidden content.
- Project CRUD must reject invalid external URLs and empty titles.
- Project wall must degrade gracefully when a project has no cover or no links.
- Admin project form must reset correctly after edit/delete.
- New pages must render without JavaScript errors on mobile widths.

---

### Task 1: Timeline API

**Files:**
- Create: `server/handlers/timeline.go`
- Modify: `server/handlers/routes.go`
- Test: `server/handlers/api_test.go`

**Interfaces:**
- Produces: `func (a *API) Timeline(c *gin.Context)`, route `GET /api/timeline`.

- [ ] Write a test creating published article/doc, a hidden doc, and a draft; assert `/api/timeline` returns only published visible items newest first.
- [ ] Add `Timeline` handler with `kind` filter, page params, `hiddenDocIDs(nil)`, and `blankContent`.
- [ ] Register `g.GET("/timeline", a.Timeline)`.
- [ ] Run `go test ./handlers -run TestTimeline`.

### Task 2: Project model and CRUD API

**Files:**
- Modify: `server/models/models.go`
- Modify: `server/db/db.go`
- Create: `server/handlers/projects.go`
- Modify: `server/handlers/routes.go`
- Test: `server/handlers/api_test.go`

**Interfaces:**
- Produces: `models.Project`, `a.ProjectList`, `a.AdminProjectList`, `a.ProjectCreate`, `a.ProjectUpdate`, `a.ProjectDelete`.

- [ ] Add `Project` model with `Title`, `Summary`, `URL`, `RepoURL`, `CoverURL`, `Tags`, `Visible`, `Sort`.
- [ ] Add `&models.Project{}` to AutoMigrate.
- [ ] Write tests for public visibility, create/update/delete and invalid URLs.
- [ ] Implement handlers and routes.
- [ ] Run project tests.

### Task 3: Timeline page

**Files:**
- Create: `web/src/views/Timeline.vue`
- Modify: `web/src/router/index.js`
- Modify: `web/src/App.vue`
- Modify: `web/src/styles/pages.css`

- [ ] Build page with filter, year/month grouping, load more and empty state.
- [ ] Add `/timeline` route and nav/footer links.
- [ ] Add responsive timeline styles.
- [ ] Run `npm run build`.

### Task 4: Project wall page

**Files:**
- Create: `web/src/views/Projects.vue`
- Modify: `web/src/router/index.js`
- Modify: `web/src/App.vue`
- Modify: `web/src/components/Icon.vue`
- Modify: `web/src/styles/pages.css`

- [ ] Build card wall with optional cover, fallback index tile, tags and external links.
- [ ] Add `/projects` route and nav/footer links.
- [ ] Add GitHub icon and responsive styles.
- [ ] Run `npm run build`.

### Task 5: Admin project management

**Files:**
- Modify: `web/src/views/Admin.vue`
- Modify: `web/src/styles/pages.css`

- [ ] Add project state, load/save/delete/reset functions.
- [ ] Add `projects` tab and panel with fields and list.
- [ ] Load projects when tab is active.
- [ ] Run `npm run build`.

### Task 6: SEO and site help

**Files:**
- Modify: `server/handlers/seo.go`
- Modify: `server/handlers/site_help.json`
- Modify: `web/src/views/Timeline.vue`
- Modify: `web/src/views/Projects.vue`

- [ ] Add `/timeline` and `/projects` to sitemap.
- [ ] Add both features to site help catalog and bump version.
- [ ] Add page metadata in both views.
- [ ] Run `go test ./...` and `npm test`.

### Task 7: Verification and deploy

- [ ] Run `go test ./...`, `npm test`, `npm run build`.
- [ ] Playwright check timeline, project wall, admin project form, and collapsed-width behavior.
- [ ] Commit, push, build Linux binary and deploy.
- [ ] Verify production pages and assets.
