package handlers_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"folio/server/cache"
	"folio/server/config"
	"folio/server/db"
	"folio/server/handlers"
	"folio/server/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type testAPI struct {
	handler   http.Handler
	api       *handlers.API
	uploadDir string
}

func newTestAPI(t *testing.T, configure ...func(*config.Config)) *testAPI {
	t.Helper()
	gin.SetMode(gin.TestMode)
	uploadDir := filepath.Join(t.TempDir(), "uploads")
	cfg := config.Config{
		Secret:      "test-secret",
		DBDriver:    "sqlite",
		DBPath:      filepath.Join(t.TempDir(), "folio-test.db"),
		Demo:        false,
		SiteName:    "测试站",
		SiteOwner:   "tester",
		UploadDir:   uploadDir,
		MaxUploadMB: 10,
	}
	for _, fn := range configure {
		fn(&cfg)
	}
	gdb, err := db.Open(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := gdb.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	api := handlers.New(gdb, cfg, cache.New("", "", 0))
	r := gin.New()
	api.Routes(r)
	return &testAPI{handler: r, api: api, uploadDir: uploadDir}
}

func (s *testAPI) do(t *testing.T, method, path, token string, body any) (int, map[string]any) {
	t.Helper()
	var req *http.Request
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("序列化请求失败: %v", err)
		}
		req = httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Logf("非 JSON 响应 (%d): %s", rec.Code, rec.Body.String())
		}
	}
	return rec.Code, out
}

func (s *testAPI) register(t *testing.T, username string) (string, uint) {
	t.Helper()
	status, data := s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": username,
		"nickname": username,
		"email":    username + "@example.com",
		"password": "password123",
	})
	if status != http.StatusCreated {
		t.Fatalf("注册 %s 失败: %d %v", username, status, data)
	}
	token, _ := data["token"].(string)
	user, _ := data["user"].(map[string]any)
	return token, uint(user["id"].(float64))
}

func (s *testAPI) upload(t *testing.T, token, field, filename string, content []byte) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("创建上传表单失败: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("写入上传内容失败: %v", err)
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/api/images", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	out := map[string]any{}
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func obj(m map[string]any, key string) map[string]any {
	v, _ := m[key].(map[string]any)
	return v
}

func list(m map[string]any, key string) []any {
	v, _ := m[key].([]any)
	return v
}

func number(v any) float64 {
	n, _ := v.(float64)
	return n
}

func TestRolePermissions(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, ownerID := s.register(t, "owner")
	writerToken, writerID := s.register(t, "writer")
	viewerToken, _ := s.register(t, "viewer")

	status, _ := s.do(t, "POST", "/api/docs", writerToken, map[string]any{
		"kind": "article", "title": "普通用户不能写", "content": "x", "status": "published",
	})
	if status != http.StatusForbidden {
		t.Fatalf("普通用户发布应返回 403，实际 %d", status)
	}

	status, _ = s.do(t, "PUT", "/api/admin/users/"+itoa(writerID)+"/role", ownerToken, map[string]any{"role": "admin"})
	if status != http.StatusOK {
		t.Fatalf("超级管理员设置管理员失败: %d", status)
	}

	status, created := s.do(t, "POST", "/api/docs", writerToken, map[string]any{
		"kind": "article", "title": "管理员可以写", "content": "x", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("管理员发布失败: %d %v", status, created)
	}

	status, _ = s.do(t, "GET", "/api/admin/users", writerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("管理员不应能管理用户，实际 %d", status)
	}
	status, _ = s.do(t, "GET", "/api/admin/stats", writerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("管理员应能查看后台概览，实际 %d", status)
	}
	status, _ = s.do(t, "GET", "/api/admin/stats", viewerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("普通用户不应能查看后台概览，实际 %d", status)
	}

	status, _ = s.do(t, "PUT", "/api/admin/users/"+itoa(ownerID)+"/role", ownerToken, map[string]any{"role": "user"})
	if status != http.StatusBadRequest {
		t.Fatalf("超级管理员不应能修改自己的角色，实际 %d", status)
	}
	status, _ = s.do(t, "PUT", "/api/admin/users/"+itoa(writerID)+"/role", ownerToken, map[string]any{"role": "super_admin"})
	if status != http.StatusBadRequest {
		t.Fatalf("不应能创建第二个超级管理员，实际 %d", status)
	}

	status, _ = s.do(t, "PUT", "/api/admin/users/"+itoa(writerID)+"/role", ownerToken, map[string]any{"role": "user"})
	if status != http.StatusOK {
		t.Fatalf("取消管理员失败: %d", status)
	}
	status, _ = s.do(t, "POST", "/api/docs", writerToken, map[string]any{
		"kind": "article", "title": "降级后不能写", "content": "x", "status": "draft",
	})
	if status != http.StatusForbidden {
		t.Fatalf("降级后发布应返回 403，实际 %d", status)
	}
}

func TestUnifiedDocumentFlow(t *testing.T) {
	s := newTestAPI(t)
	aliceToken, _ := s.register(t, "alice")
	bobToken, bobID := s.register(t, "bob")

	title := "统一内容测试"
	status, created := s.do(t, "POST", "/api/docs", aliceToken, map[string]any{
		"kind":    "article",
		"title":   title,
		"content": "## 正文\n\n测试内容。",
		"status":  "published",
		"tags":    []string{"测试", "统一"},
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d %v", status, created)
	}
	docID := uint(number(obj(created, "doc")["id"]))

	status, detail := s.do(t, "GET", "/api/docs/"+itoa(docID), bobToken, nil)
	if status != http.StatusOK || detail["doc"].(map[string]any)["title"] != title {
		t.Fatalf("读取文章失败: %d %v", status, detail)
	}
	if detail["liked"] != false {
		t.Fatalf("新文章不应已点赞: %v", detail["liked"])
	}

	status, liked := s.do(t, "POST", "/api/docs/"+itoa(docID)+"/like", bobToken, nil)
	if status != http.StatusOK || liked["liked"] != true || number(liked["likeCount"]) != 1 {
		t.Fatalf("点赞失败: %d %v", status, liked)
	}
	status, faved := s.do(t, "POST", "/api/docs/"+itoa(docID)+"/favorite", bobToken, nil)
	if status != http.StatusOK || faved["favorited"] != true || number(faved["favoriteCount"]) != 1 {
		t.Fatalf("收藏失败: %d %v", status, faved)
	}

	status, commented := s.do(t, "POST", "/api/docs/"+itoa(docID)+"/comments", bobToken, map[string]any{"content": "第一条评论"})
	if status != http.StatusCreated {
		t.Fatalf("评论失败: %d %v", status, commented)
	}
	commentID := uint(number(obj(commented, "comment")["id"]))
	status, _ = s.do(t, "POST", "/api/docs/"+itoa(docID)+"/comments", aliceToken, map[string]any{
		"content":  "回复评论",
		"parentID": commentID,
	})
	if status != http.StatusCreated {
		t.Fatalf("回复失败: %d", status)
	}
	status, comments := s.do(t, "GET", "/api/docs/"+itoa(docID)+"/comments", "", nil)
	items := list(comments, "items")
	if status != http.StatusOK || len(items) != 1 {
		t.Fatalf("评论列表异常: %d %v", status, comments)
	}
	replies, _ := items[0].(map[string]any)["replies"].([]any)
	if len(replies) != 1 {
		t.Fatalf("回复未挂到根评论: %v", items[0])
	}

	status, follow := s.do(t, "POST", "/api/users/alice/follow", bobToken, nil)
	if status != http.StatusOK || follow["following"] != true {
		t.Fatalf("关注失败: %d %v", status, follow)
	}
	status, _ = s.do(t, "POST", "/api/messages/with/alice", bobToken, map[string]any{"content": "你好"})
	if status != http.StatusCreated {
		t.Fatalf("发送私信失败: %d", status)
	}
	status, inbox := s.do(t, "GET", "/api/inbox", aliceToken, nil)
	if status != http.StatusOK || number(inbox["messages"]) < 1 || number(inbox["notifications"]) < 1 {
		t.Fatalf("收件箱异常: %d %v", status, inbox)
	}
	status, notifications := s.do(t, "GET", "/api/notifications", aliceToken, nil)
	if status != http.StatusOK || len(list(notifications, "items")) == 0 {
		t.Fatalf("通知列表异常: %d %v", status, notifications)
	}

	status, search := s.do(t, "GET", "/api/docs?q="+url.QueryEscape(title), "", nil)
	if status != http.StatusOK || !containsDoc(list(search, "items"), docID) {
		t.Fatalf("搜索未找到新文章: %d %v", status, search)
	}

	status, _ = s.do(t, "PUT", "/api/docs/"+itoa(docID), bobToken, map[string]any{
		"title": "越权修改", "content": "x", "status": "published",
	})
	if status != http.StatusForbidden {
		t.Fatalf("越权编辑应返回 403，实际 %d", status)
	}

	status, _ = s.do(t, "DELETE", "/api/docs/"+itoa(docID), aliceToken, nil)
	if status != http.StatusOK {
		t.Fatalf("删除文章失败: %d", status)
	}
	status, _ = s.do(t, "GET", "/api/docs/"+itoa(docID), "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("删除后应返回 404，实际 %d", status)
	}

	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lK3Q6wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}
	status, bobImage := s.upload(t, bobToken, "file", "bob.png", pngData)
	if status != http.StatusCreated {
		t.Fatalf("测试用户上传图片失败: %d %v", status, bobImage)
	}
	bobImagePath := filepath.Join(s.uploadDir, strings.TrimPrefix(bobImage["url"].(string), "/uploads/"))

	status, _ = s.do(t, "DELETE", "/api/admin/users/"+itoa(bobID), aliceToken, nil)
	if status != http.StatusOK {
		t.Fatalf("管理员删除用户失败: %d", status)
	}
	status, _ = s.do(t, "GET", "/api/users/bob", "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("删除用户后应返回 404，实际 %d", status)
	}
	for _, model := range []any{&models.FavoriteFolder{}, &models.ViewHistory{}, &models.UserSetting{}, &models.NoticeRead{}, &models.Image{}} {
		var remaining int64
		s.api.DB.Model(model).Where("user_id = ?", bobID).Count(&remaining)
		if remaining != 0 {
			t.Fatalf("删除用户后仍残留 %T: %d", model, remaining)
		}
	}
	if _, err := os.Stat(bobImagePath); !os.IsNotExist(err) {
		t.Fatalf("删除用户后图片文件仍然存在: %v", err)
	}
}

func TestDocumentTreeCascade(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "carol")

	status, parent := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "doc", "title": "父文档", "content": "父", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建父文档失败: %d %v", status, parent)
	}
	parentID := uint(number(obj(parent, "doc")["id"]))
	status, child := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "doc", "title": "子文档", "content": "子", "status": "published", "parentID": parentID,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建子文档失败: %d %v", status, child)
	}
	childID := uint(number(obj(child, "doc")["id"]))
	status, tree := s.do(t, "GET", "/api/doc-tree", "", nil)
	if status != http.StatusOK || len(list(tree, "items")) != 2 {
		t.Fatalf("文档树异常: %d %v", status, tree)
	}

	status, _ = s.do(t, "DELETE", "/api/docs/"+itoa(parentID), token, nil)
	if status != http.StatusOK {
		t.Fatalf("删除父文档失败: %d", status)
	}
	status, _ = s.do(t, "GET", "/api/docs/"+itoa(childID), "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("子文档应被级联删除，实际 %d", status)
	}
	status, tree = s.do(t, "GET", "/api/doc-tree", "", nil)
	if status != http.StatusOK || len(list(tree, "items")) != 0 {
		t.Fatalf("文档树未清空: %d %v", status, tree)
	}
}

func TestAuthGuards(t *testing.T) {
	s := newTestAPI(t)
	status, _ := s.do(t, "GET", "/api/me", "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("未登录访问 /me 应返回 401，实际 %d", status)
	}
	status, _ = s.do(t, "POST", "/api/docs", "", map[string]any{"title": "x", "content": "x"})
	if status != http.StatusUnauthorized {
		t.Fatalf("未登录创建文档应返回 401，实际 %d", status)
	}
	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "x", "password": "password123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("非法用户名应返回 400，实际 %d", status)
	}
	s.register(t, "dave")
	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "dave", "password": "password123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("重复用户名应返回 400，实际 %d", status)
	}
}

func TestImageUpload(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "imageowner")
	readerToken, _ := s.register(t, "imagereader")

	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lK3Q6wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}

	readerData, err := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
	if err != nil {
		t.Fatalf("解码读者测试图片失败: %v", err)
	}
	status, readerImage := s.upload(t, readerToken, "file", "reader.gif", readerData)
	if status != http.StatusCreated {
		t.Fatalf("读者应能上传聊天图片，实际 %d %v", status, readerImage)
	}

	status, first := s.upload(t, ownerToken, "file", "tiny.png", pngData)
	if status != http.StatusCreated {
		t.Fatalf("上传图片失败: %d %v", status, first)
	}
	image, _ := first["image"].(map[string]any)
	imageURL, _ := first["url"].(string)
	imageID := uint(number(image["id"]))
	if image["provider"] != "local" {
		t.Fatalf("默认图片应存本地，实际 %v", image["provider"])
	}
	if !strings.HasPrefix(imageURL, "/uploads/") {
		t.Fatalf("图片 URL 异常: %s", imageURL)
	}
	diskPath := filepath.Join(s.uploadDir, strings.TrimPrefix(imageURL, "/uploads/"))
	if _, err := os.Stat(diskPath); err != nil {
		t.Fatalf("图片文件未落盘: %v", err)
	}

	status, second := s.upload(t, ownerToken, "file", "tiny-copy.png", pngData)
	if status != http.StatusOK || second["url"] != imageURL {
		t.Fatalf("重复图片应命中秒传: %d %v", status, second)
	}

	status, list := s.do(t, "GET", "/api/images", ownerToken, nil)
	if status != http.StatusOK || len(list["items"].([]any)) != 2 {
		t.Fatalf("超级管理员应能看到两张图片: %d %v", status, list)
	}
	status, _ = s.do(t, "GET", "/api/images", readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能看到图片库，实际 %d", status)
	}

	status, _ = s.do(t, "DELETE", "/api/images/"+itoa(imageID), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("删除图片失败: %d", status)
	}
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatalf("删除图片后文件仍然存在: %v", err)
	}
}

func TestFavoriteFolders(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "folderuser")

	status, folders := s.do(t, "GET", "/api/favorite-folders", token, nil)
	items := list(folders, "items")
	if status != http.StatusOK || len(items) != 1 {
		t.Fatalf("注册后应只有一个默认收藏夹: %d %v", status, folders)
	}
	defaultID := uint(number(items[0].(map[string]any)["id"]))
	if items[0].(map[string]any)["isDefault"] != true {
		t.Fatalf("第一个收藏夹应为默认收藏夹: %v", items[0])
	}

	status, _ = s.do(t, "POST", "/api/favorite-folders", token, map[string]any{"name": "待读"})
	if status != http.StatusCreated {
		t.Fatalf("创建收藏夹失败: %d", status)
	}
	status, folders = s.do(t, "GET", "/api/favorite-folders", token, nil)
	items = list(folders, "items")
	var customID uint
	for _, item := range items {
		folder := item.(map[string]any)
		if folder["name"] == "待读" {
			customID = uint(number(folder["id"]))
		}
	}
	if customID == 0 {
		t.Fatalf("新收藏夹未返回: %v", folders)
	}

	status, doc := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "article", "title": "收藏夹测试", "content": "x", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d", status)
	}
	docID := uint(number(obj(doc, "doc")["id"]))
	status, faved := s.do(t, "POST", "/api/docs/"+itoa(docID)+"/favorite", token, map[string]any{"folderID": customID})
	if status != http.StatusOK || faved["folderID"] != float64(customID) {
		t.Fatalf("收藏到指定收藏夹失败: %d %v", status, faved)
	}
	status, listInCustom := s.do(t, "GET", "/api/me/favorites?folderID="+itoa(customID), token, nil)
	if status != http.StatusOK || len(list(listInCustom, "items")) != 1 {
		t.Fatalf("按收藏夹筛选失败: %d %v", status, listInCustom)
	}

	status, _ = s.do(t, "DELETE", "/api/favorite-folders/"+itoa(defaultID), token, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("默认收藏夹不应能删除，实际 %d", status)
	}
	status, _ = s.do(t, "DELETE", "/api/favorite-folders/"+itoa(customID), token, nil)
	if status != http.StatusOK {
		t.Fatalf("删除收藏夹失败: %d", status)
	}
	status, listInDefault := s.do(t, "GET", "/api/me/favorites?folderID="+itoa(defaultID), token, nil)
	if status != http.StatusOK || len(list(listInDefault, "items")) != 1 {
		t.Fatalf("删除收藏夹后收藏未移动到默认收藏夹: %d %v", status, listInDefault)
	}
}

func TestCategories(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "catowner")
	readerToken, _ := s.register(t, "catreader")

	status, _ := s.do(t, "POST", "/api/categories", readerToken, map[string]any{"name": "读者分类"})
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能创建分类，实际 %d", status)
	}
	status, created := s.do(t, "POST", "/api/categories", ownerToken, map[string]any{"name": "工程", "sort": 1})
	if status != http.StatusCreated {
		t.Fatalf("创建分类失败: %d %v", status, created)
	}
	categoryID := uint(number(obj(created, "category")["id"]))
	status, _ = s.do(t, "POST", "/api/categories", ownerToken, map[string]any{"name": "工程"})
	if status != http.StatusBadRequest {
		t.Fatalf("重复分类应返回 400，实际 %d", status)
	}

	status, article := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "article", "title": "带分类的文章", "content": "x", "status": "published",
		"tags": []string{"分类测试"}, "categoryID": categoryID,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建带分类文章失败: %d %v", status, article)
	}
	docID := uint(number(obj(article, "doc")["id"]))
	status, detail := s.do(t, "GET", "/api/docs/"+itoa(docID), "", nil)
	if status != http.StatusOK || obj(detail, "doc")["category"].(map[string]any)["name"] != "工程" {
		t.Fatalf("文章分类未返回: %d %v", status, detail)
	}
	status, filtered := s.do(t, "GET", "/api/docs?kind=article&categoryID="+itoa(categoryID), "", nil)
	if status != http.StatusOK || len(list(filtered, "items")) != 1 {
		t.Fatalf("分类筛选异常: %d %v", status, filtered)
	}

	status, _ = s.do(t, "DELETE", "/api/categories/"+itoa(categoryID), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("删除分类失败: %d", status)
	}
	status, detail = s.do(t, "GET", "/api/docs/"+itoa(docID), "", nil)
	if status != http.StatusOK || obj(detail, "doc")["categoryID"] != nil {
		t.Fatalf("删除分类后文章应变为未分类: %d %v", status, detail)
	}
}

func TestHistoryAndRelated(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "histowner")
	readerToken, _ := s.register(t, "histreader")

	status, first := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "article", "title": "第一篇", "content": "x", "status": "published",
		"tags": []string{"共同标签"},
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d", status)
	}
	firstID := uint(number(obj(first, "doc")["id"]))
	status, second := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "article", "title": "第二篇", "content": "x", "status": "published",
		"tags": []string{"共同标签"},
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d", status)
	}
	secondID := uint(number(obj(second, "doc")["id"]))

	s.do(t, "GET", "/api/docs/"+itoa(firstID), readerToken, nil)
	status, history := s.do(t, "GET", "/api/me/history", readerToken, nil)
	items := list(history, "items")
	if status != http.StatusOK || len(items) == 0 {
		t.Fatalf("浏览历史为空: %d %v", status, history)
	}
	status, related := s.do(t, "GET", "/api/docs/"+itoa(firstID)+"/related", "", nil)
	found := false
	for _, item := range list(related, "items") {
		if uint(number(item.(map[string]any)["id"])) == secondID {
			found = true
		}
	}
	if status != http.StatusOK || !found {
		t.Fatalf("相关文章未命中: %d %v", status, related)
	}

	status, _ = s.do(t, "DELETE", "/api/me/history", readerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("清空历史失败: %d", status)
	}
	status, history = s.do(t, "GET", "/api/me/history", readerToken, nil)
	if status != http.StatusOK || len(list(history, "items")) != 0 {
		t.Fatalf("清空后历史不为空: %d %v", status, history)
	}
}

func TestAIAnalysis(t *testing.T) {
	payloadText := `{"title":"测试标题","abstract":"测试摘要","tags":["Go","测试"]}`
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id":"msg_test","type":"message","role":"assistant","model":"claude-opus-5-5","content":[{"type":"text","text":%s}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":20}}`, strconv.Quote(payloadText))
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.AIAPIKey = "test-key"
		cfg.AIBaseURL = fake.URL
		cfg.AIModel = "claude-opus-5-5"
	})
	ownerToken, _ := s.register(t, "aiowner")
	readerToken, _ := s.register(t, "aireader")

	status, aiStatus := s.do(t, "GET", "/api/ai/status", ownerToken, nil)
	if status != http.StatusOK || aiStatus["enabled"] != true {
		t.Fatalf("AI 状态异常: %d %v", status, aiStatus)
	}
	status, _ = s.do(t, "GET", "/api/ai/status", readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能访问 AI 状态，实际 %d", status)
	}
	status, result := s.do(t, "POST", "/api/ai/analyze", ownerToken, map[string]any{
		"title":   "",
		"content": "这是一篇关于 Go 与个人知识库的文章，正文足够长，可以用来测试 AI 摘要接口是否正常工作。",
	})
	if status != http.StatusOK || result["title"] != "测试标题" || len(result["tags"].([]any)) != 2 {
		t.Fatalf("AI 摘要结果异常: %d %v", status, result)
	}
	status, _ = s.do(t, "POST", "/api/ai/analyze", readerToken, map[string]any{
		"content": "这是一篇关于 Go 与个人知识库的文章，正文足够长，可以用来测试 AI 摘要接口是否正常工作。",
	})
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能使用 AI 摘要，实际 %d", status)
	}

	plain := newTestAPI(t)
	plainToken, _ := plain.register(t, "plainowner")
	status, plainStatus := plain.do(t, "GET", "/api/ai/status", plainToken, nil)
	if status != http.StatusOK || plainStatus["enabled"] != false {
		t.Fatalf("未配置时应为 disabled: %d %v", status, plainStatus)
	}
}

func TestHealth(t *testing.T) {
	s := newTestAPI(t)
	status, health := s.do(t, "GET", "/api/health", "", nil)
	if status != http.StatusOK || health["ok"] != true || health["db"] != "sqlite" || health["redis"] != "off" {
		t.Fatalf("健康检查异常: %d %v", status, health)
	}
}

func containsDoc(items []any, id uint) bool {
	for _, item := range items {
		doc, _ := item.(map[string]any)
		if uint(number(doc["id"])) == id {
			return true
		}
	}
	return false
}

func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

func TestEmailFlows(t *testing.T) {
	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.MailDebug = true
		cfg.SiteName = "测试站"
	})

	status, data := s.do(t, "GET", "/api/auth/email/status", "", nil)
	if status != http.StatusOK || data["enabled"] != true {
		t.Fatalf("邮件调试模式应启用邮箱能力: %d %v", status, data)
	}

	send := func(email, purpose string, tokens ...string) string {
		t.Helper()
		token := ""
		if len(tokens) > 0 {
			token = tokens[0]
		}
		status, body := s.do(t, "POST", "/api/auth/email/code", token, map[string]string{
			"email": email, "purpose": purpose,
		})
		if status != http.StatusOK {
			t.Fatalf("发送 %s 验证码失败: %d %v", purpose, status, body)
		}
		code, _ := body["debugCode"].(string)
		if len(code) != 6 {
			t.Fatalf("调试模式应返回 6 位验证码: %v", body)
		}
		return code
	}

	email := "newbie@example.com"
	code := send(email, "register")

	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "newbie", "nickname": "新来的", "email": email, "password": "password123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("缺少验证码时注册应失败, 实际 %d", status)
	}
	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "newbie", "nickname": "新来的", "email": email, "password": "password123", "code": "000000",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("错误验证码注册应失败, 实际 %d", status)
	}
	status, created := s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "newbie", "nickname": "新来的", "email": email, "password": "password123", "code": code,
	})
	if status != http.StatusCreated {
		t.Fatalf("验证码注册失败: %d %v", status, created)
	}
	token, _ := created["token"].(string)
	if token == "" {
		t.Fatal("注册后应返回 token")
	}
	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "newbie2", "nickname": "新来的", "email": email, "password": "password123", "code": code,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("验证码不能重复使用, 实际 %d", status)
	}

	// 邮箱验证码登录
	loginCode := send(email, "login")
	status, logged := s.do(t, "POST", "/api/auth/email/login", "", map[string]string{"email": email, "code": loginCode})
	if status != http.StatusOK {
		t.Fatalf("邮箱验证码登录失败: %d %v", status, logged)
	}
	if logged["token"] == "" || obj(logged, "user")["username"] != "newbie" {
		t.Fatalf("邮箱登录返回不正确: %v", logged)
	}

	// 绑定新邮箱
	bindEmail := "bound@example.com"
	bindCode := send(bindEmail, "bind", token)
	status, bound := s.do(t, "PUT", "/api/me/email", token, map[string]string{"email": bindEmail, "code": bindCode})
	if status != http.StatusOK {
		t.Fatalf("绑定邮箱失败: %d %v", status, bound)
	}
	if obj(bound, "user")["email"] != bindEmail {
		t.Fatalf("绑定后邮箱不正确: %v", bound)
	}

	// 找回密码
	resetCode := send(bindEmail, "reset")
	status, reset := s.do(t, "POST", "/api/auth/password/reset", "", map[string]string{
		"email": bindEmail, "code": resetCode, "password": "newpassword123",
	})
	if status != http.StatusOK {
		t.Fatalf("重置密码失败: %d %v", status, reset)
	}
	status, _ = s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "newbie", "password": "newpassword123",
	})
	if status != http.StatusOK {
		t.Fatalf("新密码登录失败: %d", status)
	}

	// 同一邮箱 60 秒内不能重复发送
	status, _ = s.do(t, "POST", "/api/auth/email/code", "", map[string]string{
		"email": bindEmail, "purpose": "reset",
	})
	if status != http.StatusTooManyRequests {
		t.Fatalf("重复发送应被限流, 实际 %d", status)
	}
}

func TestEmailDisabled(t *testing.T) {
	s := newTestAPI(t)
	status, data := s.do(t, "GET", "/api/auth/email/status", "", nil)
	if status != http.StatusOK || data["enabled"] != false {
		t.Fatalf("未配置邮件时应关闭邮箱能力: %d %v", status, data)
	}
	status, _ = s.do(t, "POST", "/api/auth/email/code", "", map[string]string{
		"email": "someone@example.com", "purpose": "login",
	})
	if status != http.StatusServiceUnavailable {
		t.Fatalf("未配置邮件时发送验证码应返回 503, 实际 %d", status)
	}
}

func TestCaptchaProtection(t *testing.T) {
	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.LoginCaptcha = true
	})

	status, data := s.do(t, "GET", "/api/auth/captcha", "", nil)
	if status != http.StatusOK || data["enabled"] != true {
		t.Fatalf("图形验证码应启用: %d %v", status, data)
	}
	id, _ := data["captchaId"].(string)
	image, _ := data["image"].(string)
	if id == "" || !strings.HasPrefix(image, "data:image/png;base64,") {
		t.Fatalf("图形验证码返回不完整: %v", data)
	}

	status, _ = s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "nobody", "password": "password123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("缺少图形验证码应返回 400，实际 %d", status)
	}
	status, _ = s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "nobody", "password": "password123", "captchaId": id, "captchaCode": "zzzz",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("错误图形验证码应返回 400，实际 %d", status)
	}

	status, data = s.do(t, "GET", "/api/auth/captcha", "", nil)
	id, _ = data["captchaId"].(string)
	code := s.api.Captcha.Get(id, false)
	if code == "" {
		t.Fatal("测试存储中应能取到图形验证码答案")
	}
	status, _ = s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "nobody", "password": "password123", "captchaId": id, "captchaCode": code,
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("验证码正确后应继续校验账号并返回 401，实际 %d", status)
	}
}

func TestCaptchaDisabled(t *testing.T) {
	s := newTestAPI(t)
	status, data := s.do(t, "GET", "/api/auth/captcha", "", nil)
	if status != http.StatusOK || data["enabled"] != false {
		t.Fatalf("未开启时图形验证码应关闭: %d %v", status, data)
	}
}

func TestSiteSettingsAndLogs(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "siteowner")
	adminToken, adminID := s.register(t, "plainadmin")
	status, _ := s.do(t, "PUT", "/api/admin/users/"+itoa(adminID)+"/role", ownerToken, map[string]any{"role": "admin"})
	if status != http.StatusOK {
		t.Fatalf("提升管理员失败: %d", status)
	}

	status, siteData := s.do(t, "GET", "/api/site", "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取站点失败: %d", status)
	}
	info := obj(siteData, "site")
	if info["name"] != "测试站" || info["allowRegister"] != true {
		t.Fatalf("默认站点设置不正确: %v", info)
	}

	status, updated := s.do(t, "PUT", "/api/admin/settings", ownerToken, map[string]any{
		"siteName":      "新站点",
		"siteDesc":      "新的描述",
		"siteOwner":     "新维护者",
		"allowRegister": false,
		"loginCaptcha":  true,
	})
	if status != http.StatusOK {
		t.Fatalf("保存站点设置失败: %d %v", status, updated)
	}
	status, siteData = s.do(t, "GET", "/api/site", "", nil)
	info = obj(siteData, "site")
	if info["name"] != "新站点" || info["allowRegister"] != false {
		t.Fatalf("站点设置未生效: %v", info)
	}

	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "closed", "nickname": "关闭注册", "email": "closed@example.com", "password": "password123",
	})
	if status != http.StatusForbidden {
		t.Fatalf("关闭注册后应返回 403，实际 %d", status)
	}

	status, captcha := s.do(t, "GET", "/api/auth/captcha", "", nil)
	if status != http.StatusOK || captcha["enabled"] != true {
		t.Fatalf("后台开启图形验证码后应生效: %d %v", status, captcha)
	}

	status, logData := s.do(t, "GET", "/api/admin/logs", ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取操作日志失败: %d", status)
	}
	actions := map[string]bool{}
	for _, item := range list(logData, "items") {
		entry, _ := item.(map[string]any)
		actions[fmt.Sprint(entry["action"])] = true
	}
	if !actions["注册账号"] || !actions["更新设置"] {
		t.Fatalf("操作日志缺少关键动作: %v", actions)
	}

	// 管理员不能修改站点设置
	status, _ = s.do(t, "PUT", "/api/admin/settings", adminToken, map[string]any{
		"siteName": "越权", "siteDesc": "", "siteOwner": "", "allowRegister": true, "loginCaptcha": false,
	})
	if status != http.StatusForbidden {
		t.Fatalf("管理员不应能修改站点设置，实际 %d", status)
	}
}

func TestCommentToggle(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "commentowner")

	status, created := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "article", "title": "关闭评论的文章", "content": "正文", "status": "published", "openComment": false,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建关闭评论的文章失败: %d %v", status, created)
	}
	doc := obj(created, "doc")
	if doc["openComment"] != false {
		t.Fatalf("创建时应保留关闭评论设置: %v", doc)
	}
	docID := number(doc["id"])

	status, _ = s.do(t, "POST", "/api/docs/"+itoa(uint(docID))+"/comments", token, map[string]string{"content": "能评论吗"})
	if status != http.StatusForbidden {
		t.Fatalf("关闭评论后应返回 403，实际 %d", status)
	}

	status, updated := s.do(t, "PUT", "/api/docs/"+itoa(uint(docID)), token, map[string]any{
		"title": "重新打开评论", "content": "正文", "status": "published", "openComment": true,
	})
	if status != http.StatusOK || obj(updated, "doc")["openComment"] != true {
		t.Fatalf("重新打开评论失败: %d %v", status, updated)
	}
	status, comment := s.do(t, "POST", "/api/docs/"+itoa(uint(docID))+"/comments", token, map[string]string{"content": "现在可以了"})
	if status != http.StatusCreated {
		t.Fatalf("打开评论后应能评论: %d %v", status, comment)
	}

	status, defaultCreated := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "article", "title": "默认允许评论", "content": "正文", "status": "published",
	})
	if status != http.StatusCreated || obj(defaultCreated, "doc")["openComment"] != true {
		t.Fatalf("默认应允许评论: %d %v", status, defaultCreated)
	}
}

func TestMessageImagesAndWebSocket(t *testing.T) {
	s := newTestAPI(t)
	senderToken, _ := s.register(t, "msg_sender")
	receiverToken, _ := s.register(t, "msg_receiver")
	otherToken, _ := s.register(t, "msg_other")

	pngData, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAFgwJ/lK3Q6wAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatalf("解码测试图片失败: %v", err)
	}
	status, uploaded := s.upload(t, senderToken, "file", "chat.png", pngData)
	if status != http.StatusCreated {
		t.Fatalf("上传聊天图片失败: %d %v", status, uploaded)
	}
	imageURL, _ := uploaded["url"].(string)

	server := httptest.NewServer(s.handler)
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/ws?token=" + url.QueryEscape(receiverToken)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("建立 WebSocket 失败: %v", err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var ready map[string]any
	if err := conn.ReadJSON(&ready); err != nil || ready["type"] != "ready" {
		t.Fatalf("没有收到 WebSocket 就绪事件: %v %v", ready, err)
	}

	status, sent := s.do(t, "POST", "/api/messages/with/msg_receiver", senderToken, map[string]string{
		"type": "image", "imageURL": imageURL, "content": "看这张图",
	})
	if status != http.StatusCreated {
		t.Fatalf("发送图片私信失败: %d %v", status, sent)
	}
	var event map[string]any
	if err := conn.ReadJSON(&event); err != nil {
		t.Fatalf("没有收到实时私信: %v", err)
	}
	message := obj(event, "message")
	if event["type"] != "message" || message["type"] != "image" || message["imageURL"] != imageURL {
		t.Fatalf("实时私信内容不正确: %v", event)
	}

	status, conversations := s.do(t, "GET", "/api/messages/conversations", receiverToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取会话失败: %d", status)
	}
	items := list(conversations, "items")
	if len(items) != 1 || !strings.HasPrefix(fmt.Sprint(items[0].(map[string]any)["lastMessage"]), "[图片]") {
		t.Fatalf("会话预览应显示图片标记: %v", items)
	}

	// 不能盗用别人上传的图片
	otherData, err := base64.StdEncoding.DecodeString("R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7")
	if err != nil {
		t.Fatalf("解码第二张测试图片失败: %v", err)
	}
	status, otherUpload := s.upload(t, otherToken, "file", "other.gif", otherData)
	if status != http.StatusCreated {
		t.Fatalf("其他用户上传图片失败: %d", status)
	}
	otherURL, _ := otherUpload["url"].(string)
	status, _ = s.do(t, "POST", "/api/messages/with/msg_receiver", senderToken, map[string]string{
		"type": "image", "imageURL": otherURL,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("不应允许发送别人的图片，实际 %d", status)
	}
}

func TestUserPrivacySettings(t *testing.T) {
	s := newTestAPI(t)
	aliceToken, aliceID := s.register(t, "alice")
	bobToken, _ := s.register(t, "bob")

	status, settings := s.do(t, "GET", "/api/me/settings", aliceToken, nil)
	initial := obj(settings, "settings")
	if status != http.StatusOK || initial["openFans"] != true || initial["homeStyle"] != "cover" {
		t.Fatalf("默认隐私设置不正确: %d %v", status, settings)
	}

	status, created := s.do(t, "POST", "/api/docs", aliceToken, map[string]any{
		"kind": "article", "title": "隐私测试文章", "content": "正文", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d %v", status, created)
	}
	docID := uint(number(obj(created, "doc")["id"]))

	status, _ = s.do(t, "POST", "/api/users/alice/follow", bobToken, nil)
	if status != http.StatusOK {
		t.Fatalf("关注失败: %d", status)
	}
	status, notificationData := s.do(t, "GET", "/api/notifications", aliceToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取通知失败: %d", status)
	}
	before := len(list(notificationData, "items"))

	status, _ = s.do(t, "PUT", "/api/me/settings", aliceToken, map[string]any{
		"openFollow": false, "openFans": false, "openPrivateChat": false,
		"openCommentMessage": false, "openDiggMessage": false, "homeStyle": "plain",
	})
	if status != http.StatusOK {
		t.Fatalf("保存隐私设置失败: %d", status)
	}

	status, profileData := s.do(t, "GET", "/api/users/alice", bobToken, nil)
	profileSettings := obj(profileData, "settings")
	if status != http.StatusOK || profileSettings["openFans"] != false || profileSettings["homeStyle"] != "plain" {
		t.Fatalf("公开主页没有反映隐私设置: %d %v", status, profileData)
	}
	if number(obj(profileData, "counts")["followers"]) != 0 || number(obj(profileData, "counts")["following"]) != 0 {
		t.Fatalf("隐藏后计数应归零: %v", profileData)
	}

	status, hidden := s.do(t, "GET", "/api/users/alice/followers", bobToken, nil)
	if status != http.StatusOK || hidden["hidden"] != true || len(list(hidden, "items")) != 0 {
		t.Fatalf("粉丝列表应被隐藏: %d %v", status, hidden)
	}

	status, _ = s.do(t, "POST", "/api/docs/"+itoa(docID)+"/like", bobToken, nil)
	if status != http.StatusOK {
		t.Fatalf("点赞失败: %d", status)
	}
	status, comment := s.do(t, "POST", "/api/docs/"+itoa(docID)+"/comments", bobToken, map[string]string{"content": "关闭通知后的评论"})
	if status != http.StatusCreated {
		t.Fatalf("评论失败: %d %v", status, comment)
	}
	_, notificationData = s.do(t, "GET", "/api/notifications", aliceToken, nil)
	if after := len(list(notificationData, "items")); after != before {
		t.Fatalf("关闭通知后不应新增通知: before=%d after=%d", before, after)
	}

	status, _ = s.do(t, "POST", "/api/messages/with/alice", bobToken, map[string]string{"content": "你好"})
	if status != http.StatusForbidden {
		t.Fatalf("关闭私信后应返回 403，实际 %d", status)
	}

	status, _ = s.do(t, "PUT", "/api/me/settings", aliceToken, map[string]any{
		"openFollow": true, "openFans": true, "openPrivateChat": true,
		"openCommentMessage": true, "openDiggMessage": true, "homeStyle": "compact",
	})
	if status != http.StatusOK {
		t.Fatalf("重新打开隐私设置失败: %d", status)
	}
	status, _ = s.do(t, "POST", "/api/messages/with/alice", bobToken, map[string]string{"content": "现在可以了"})
	if status != http.StatusCreated {
		t.Fatalf("打开私信后应可发送，实际 %d", status)
	}
	status, profileData = s.do(t, "GET", "/api/users/alice", bobToken, nil)
	if status != http.StatusOK || obj(profileData, "settings")["homeStyle"] != "compact" {
		t.Fatalf("主页样式没有更新: %v", profileData)
	}
	_ = aliceID
}

func TestMyLoginRecords(t *testing.T) {
	s := newTestAPI(t)
	token, userID := s.register(t, "loginlog")
	status, loginData := s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "loginlog", "password": "password123",
	})
	if status != http.StatusOK {
		t.Fatalf("登录失败: %d %v", status, loginData)
	}
	loginToken, _ := loginData["token"].(string)

	status, records := s.do(t, "GET", "/api/me/login-records", loginToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取登录记录失败: %d", status)
	}
	actions := map[string]bool{}
	for _, item := range list(records, "items") {
		entry := item.(map[string]any)
		if uint(number(entry["userID"])) != userID {
			t.Fatalf("登录记录不应包含其他用户: %v", entry)
		}
		actions[fmt.Sprint(entry["action"])] = true
	}
	if !actions["登录"] || !actions["注册账号"] {
		t.Fatalf("登录记录缺少关键动作: %v", actions)
	}
	_ = token
}

func TestQQLogin(t *testing.T) {
	qqServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth2.0/token":
			_, _ = w.Write([]byte(`{"access_token":"fake-token","expires_in":3600}`))
		case "/oauth2.0/me":
			_, _ = w.Write([]byte(`{"client_id":"test-app","openid":"openid-123"}`))
		case "/user/get_user_info":
			_, _ = w.Write([]byte(`{"ret":0,"nickname":"QQ 小友"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer qqServer.Close()

	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.QQAppID = "test-app"
		cfg.QQAppKey = "test-key"
		cfg.QQRedirect = "http://localhost:8080/api/auth/qq/callback"
		cfg.QQAPIBase = qqServer.URL
	})
	s.register(t, "qqbase")

	status, data := s.do(t, "GET", "/api/auth/qq/status", "", nil)
	if status != http.StatusOK || data["enabled"] != true {
		t.Fatalf("QQ 登录应启用: %d %v", status, data)
	}
	status, data = s.do(t, "GET", "/api/auth/qq/url", "", nil)
	if status != http.StatusOK {
		t.Fatalf("获取 QQ 登录地址失败: %d %v", status, data)
	}
	authURL, _ := data["url"].(string)
	parsed, err := url.Parse(authURL)
	if err != nil || parsed.Query().Get("client_id") != "test-app" || parsed.Query().Get("state") == "" {
		t.Fatalf("QQ 授权地址不正确: %s", authURL)
	}
	state := parsed.Query().Get("state")

	callbackPath := "/api/auth/qq/callback?code=fake-code&state=" + url.QueryEscape(state)
	req := httptest.NewRequest("GET", callbackPath, nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("QQ 回调应重定向，实际 %d: %s", rec.Code, rec.Body.String())
	}
	location, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatalf("解析回调地址失败: %v", err)
	}
	ticket := location.Query().Get("qq_ticket")
	if ticket == "" {
		t.Fatalf("回调没有返回登录凭证: %s", location.String())
	}

	status, exchanged := s.do(t, "POST", "/api/auth/qq/exchange", "", map[string]string{"ticket": ticket})
	if status != http.StatusOK {
		t.Fatalf("兑换 QQ 登录凭证失败: %d %v", status, exchanged)
	}
	user := obj(exchanged, "user")
	if !strings.HasPrefix(fmt.Sprint(user["username"]), "qq_") || user["nickname"] != "QQ 小友" {
		t.Fatalf("QQ 用户信息不正确: %v", user)
	}

	// 同一个 openid 再次登录应复用账号
	req = httptest.NewRequest("GET", callbackPath, nil)
	rec = httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	location, _ = url.Parse(rec.Header().Get("Location"))
	ticket = location.Query().Get("qq_ticket")
	status, exchangedAgain := s.do(t, "POST", "/api/auth/qq/exchange", "", map[string]string{"ticket": ticket})
	if status != http.StatusOK || obj(exchangedAgain, "user")["id"] != user["id"] {
		t.Fatalf("QQ 重复登录应复用账号: %d %v", status, exchangedAgain)
	}

	status, _ = s.do(t, "GET", "/api/me", exchangedAgain["token"].(string), nil)
	if status != http.StatusOK {
		t.Fatalf("QQ 登录 token 不可用: %d", status)
	}

	// 非法 state 应被拒绝
	req = httptest.NewRequest("GET", "/api/auth/qq/callback?code=fake&state=bad", nil)
	rec = httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	badLocation, _ := url.Parse(rec.Header().Get("Location"))
	if badLocation == nil || badLocation.Query().Get("qq_error") == "" {
		t.Fatalf("非法 state 应返回错误: %s", rec.Header().Get("Location"))
	}
}

func TestQQDisabledByDefault(t *testing.T) {
	s := newTestAPI(t)
	status, data := s.do(t, "GET", "/api/auth/qq/status", "", nil)
	if status != http.StatusOK || data["enabled"] != false {
		t.Fatalf("默认应关闭 QQ 登录: %d %v", status, data)
	}
	status, _ = s.do(t, "GET", "/api/auth/qq/url", "", nil)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("未配置时获取 QQ 地址应返回 503，实际 %d", status)
	}
}

func TestCaptchaDebugMode(t *testing.T) {
	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.LoginCaptcha = true
		cfg.CaptchaDebug = true
	})
	status, data := s.do(t, "GET", "/api/auth/captcha", "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取调试验证码失败: %d", status)
	}
	id, _ := data["captchaId"].(string)
	code, _ := data["debugCode"].(string)
	if id == "" || code == "" {
		t.Fatalf("调试模式应返回验证码答案: %v", data)
	}
	status, _ = s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "nobody", "password": "password123", "captchaId": id, "captchaCode": code,
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("调试答案应能通过图形验证码，实际 %d", status)
	}
}

func TestGlobalNotices(t *testing.T) {
	s := newTestAPI(t)
	adminToken, _ := s.register(t, "noticeadmin")
	userToken, _ := s.register(t, "noticeuser")

	status, created := s.do(t, "POST", "/api/admin/notices", adminToken, map[string]string{
		"title": "站点维护", "content": "今晚十点进行例行维护。", "link": "/articles",
	})
	if status != http.StatusCreated {
		t.Fatalf("发布公告失败: %d %v", status, created)
	}
	noticeID := uint(number(obj(created, "notice")["id"]))

	status, notices := s.do(t, "GET", "/api/notices", userToken, nil)
	if status != http.StatusOK || len(list(notices, "items")) != 1 || notices["unread"] != float64(1) {
		t.Fatalf("公告列表不正确: %d %v", status, notices)
	}
	if list(notices, "items")[0].(map[string]any)["read"] != false {
		t.Fatalf("公告初始应为未读: %v", notices)
	}
	_, inbox := s.do(t, "GET", "/api/inbox", userToken, nil)
	if inbox["notices"] != float64(1) {
		t.Fatalf("收件箱应包含公告未读数: %v", inbox)
	}
	status, notificationData := s.do(t, "GET", "/api/notifications", userToken, nil)
	if status != http.StatusOK || len(list(notificationData, "notices")) != 1 || notificationData["noticeUnread"] != float64(1) {
		t.Fatalf("通知页应包含公告: %d %v", status, notificationData)
	}

	request := s.do
	status, _ = request(t, "POST", "/api/notices/read", userToken, nil)
	if status != http.StatusOK {
		t.Fatalf("标记公告已读失败: %d", status)
	}
	_, notices = s.do(t, "GET", "/api/notices", userToken, nil)
	if notices["unread"] != float64(0) || list(notices, "items")[0].(map[string]any)["read"] != true {
		t.Fatalf("公告已读状态不正确: %v", notices)
	}

	status, _ = s.do(t, "POST", "/api/admin/notices", userToken, map[string]string{"title": "越权", "content": "x"})
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能发布公告，实际 %d", status)
	}
	status, _ = s.do(t, "DELETE", "/api/admin/notices/"+itoa(noticeID), adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("删除公告失败: %d", status)
	}
	_, notices = s.do(t, "GET", "/api/notices", userToken, nil)
	if len(list(notices, "items")) != 0 {
		t.Fatalf("删除后公告仍存在: %v", notices)
	}
}
