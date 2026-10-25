package handlers_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	"folio/server/secure"

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

func (s *testAPI) raw(t *testing.T, method, path, token string) (int, http.Header, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	req.Host = "example.com"
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	return rec.Code, rec.Header(), rec.Body.String()
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

func TestSEOEndpoints(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "seoowner")

	status, created := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "article", "title": "分享卡片测试文章", "abstract": "这是摘要，用于 RSS 与分享卡片。",
		"content": "正文内容", "tags": []string{"SEO", "测试"}, "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d %v", status, created)
	}
	articleID := uint(number(obj(created, "doc")["id"]))
	status, docCreated := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "doc", "title": "分享卡片测试文档", "content": "文档正文", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文档失败: %d %v", status, docCreated)
	}
	docID := uint(number(obj(docCreated, "doc")["id"]))
	status, _ = s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "article", "title": "不应出现在订阅里的草稿", "content": "草稿", "status": "draft",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建草稿失败: %d", status)
	}

	status, headers, feed := s.raw(t, "GET", "/feed.xml", "")
	if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "rss+xml") {
		t.Fatalf("RSS 响应异常: %d %s", status, headers.Get("Content-Type"))
	}
	if !strings.Contains(feed, "分享卡片测试文章") || !strings.Contains(feed, fmt.Sprintf("/articles/%d", articleID)) {
		t.Fatalf("RSS 缺少文章: %s", feed)
	}
	if strings.Contains(feed, "不应出现在订阅里的草稿") {
		t.Fatalf("RSS 不应包含草稿: %s", feed)
	}

	status, _, sitemap := s.raw(t, "GET", "/sitemap.xml", "")
	if status != http.StatusOK {
		t.Fatalf("sitemap 状态异常: %d", status)
	}
	for _, want := range []string{"/articles", "/docs", "/about", fmt.Sprintf("/articles/%d", articleID), fmt.Sprintf("/docs/%d", docID), "/u/seoowner"} {
		if !strings.Contains(sitemap, want) {
			t.Fatalf("sitemap 缺少 %s: %s", want, sitemap)
		}
	}
	if strings.Contains(sitemap, "不应出现在订阅里的草稿") {
		t.Fatalf("sitemap 不应包含草稿")
	}

	status, _, robots := s.raw(t, "GET", "/robots.txt", "")
	if status != http.StatusOK || !strings.Contains(robots, "Sitemap: http://example.com/sitemap.xml") || !strings.Contains(robots, "Disallow: /admin") {
		t.Fatalf("robots.txt 异常: %d %s", status, robots)
	}

	status, headers, image := s.raw(t, "GET", "/api/art/cover.png?seed=1&kind=article", "")
	if status != http.StatusOK || headers.Get("Content-Type") != "image/png" || !strings.HasPrefix(image, "\x89PNG") {
		t.Fatalf("分享图异常: %d %s", status, headers.Get("Content-Type"))
	}

	dist := t.TempDir()
	index := `<!doctype html><html><head><!--folio:meta--><title>旧标题</title></head><body></body></html>`
	if err := os.WriteFile(filepath.Join(dist, "index.html"), []byte(index), 0o644); err != nil {
		t.Fatalf("写入测试 index.html 失败: %v", err)
	}
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	req := httptest.NewRequest("GET", fmt.Sprintf("/articles/%d", articleID), nil)
	req.Host = "example.com"
	ctx.Request = req
	s.api.ServeIndex(ctx, dist)
	page := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(page, "分享卡片测试文章 · 测试站") {
		t.Fatalf("服务端标题注入异常: %d %s", rec.Code, page)
	}
	if !strings.Contains(page, `property="og:image"`) || !strings.Contains(page, "application/ld+json") {
		t.Fatalf("分享元信息不完整: %s", page)
	}

	rec = httptest.NewRecorder()
	ctx, _ = gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest("GET", "/about", nil)
	ctx.Request.Host = "example.com"
	s.api.ServeIndex(ctx, dist)
	aboutPage := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(aboutPage, "关于 · 测试站") || !strings.Contains(aboutPage, "AboutPage") {
		t.Fatalf("关于页元信息不完整: %d %s", rec.Code, aboutPage)
	}
}

func TestAdminDashboard(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "dashboardowner")
	readerToken, _ := s.register(t, "dashboardreader")
	status, created := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "article", "title": "看板测试文章", "content": "正文", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建看板测试文章失败: %d %v", status, created)
	}
	docID := uint(number(obj(created, "doc")["id"]))
	status, _ = s.do(t, "GET", "/api/docs/"+itoa(docID), readerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取文章失败: %d", status)
	}

	status, dashboard := s.do(t, "GET", "/api/admin/dashboard?range=7", ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取数据看板失败: %d %v", status, dashboard)
	}
	trend := obj(dashboard, "trend")
	dates := list(trend, "dates")
	if len(dates) != 7 || len(list(trend, "views")) != 7 {
		t.Fatalf("7 天趋势长度不正确: %v", trend)
	}
	if number(obj(dashboard, "summary")["views"]) < 1 {
		t.Fatalf("阅读总量应至少为 1: %v", dashboard)
	}
	top := list(dashboard, "top")
	if len(top) == 0 || top[0].(map[string]any)["id"] != float64(docID) {
		t.Fatalf("热门内容不正确: %v", top)
	}
	if number(obj(dashboard, "system")["goroutines"]) <= 0 {
		t.Fatalf("系统信息不正确: %v", dashboard["system"])
	}

	var daily int64
	s.api.DB.Model(&models.DocDailyStat{}).Where("doc_id = ?", docID).Count(&daily)
	if daily != 1 {
		t.Fatalf("应写入一条每日阅读统计，实际 %d", daily)
	}

	status, dashboard = s.do(t, "GET", "/api/admin/dashboard?range=30", ownerToken, nil)
	if status != http.StatusOK || len(list(obj(dashboard, "trend"), "dates")) != 30 {
		t.Fatalf("30 天看板异常: %d %v", status, dashboard)
	}
	status, _ = s.do(t, "GET", "/api/admin/dashboard", readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能查看看板，实际 %d", status)
	}

	status, _ = s.do(t, "DELETE", "/api/docs/"+itoa(docID), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("删除看板测试文章失败: %d", status)
	}
	s.api.DB.Model(&models.DocDailyStat{}).Where("doc_id = ?", docID).Count(&daily)
	if daily != 0 {
		t.Fatalf("删除内容后每日统计应一并清理，实际 %d", daily)
	}
}

func TestInvitesAndAdminCreateUser(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "inviteowner")

	status, _ := s.do(t, "PUT", "/api/admin/settings", ownerToken, map[string]any{
		"siteName": "测试站", "siteDesc": "测试", "siteOwner": "tester",
		"allowRegister": false, "loginCaptcha": false,
	})
	if status != http.StatusOK {
		t.Fatalf("关闭注册失败: %d", status)
	}
	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "closeduser", "password": "password123",
	})
	if status != http.StatusForbidden {
		t.Fatalf("关闭注册后普通注册应失败, 实际 %d", status)
	}

	status, inviteData := s.do(t, "POST", "/api/admin/invites", ownerToken, map[string]any{
		"note": "朋友邀请", "expiresInDays": 7,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建邀请码失败: %d %v", status, inviteData)
	}
	invite := obj(inviteData, "invite")
	code, _ := invite["code"].(string)
	if code == "" {
		t.Fatalf("邀请码为空: %v", inviteData)
	}
	status, registered := s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "inviteduser", "nickname": "受邀用户", "password": "password123", "invite": code,
	})
	if status != http.StatusCreated {
		t.Fatalf("邀请码注册失败: %d %v", status, registered)
	}
	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "inviteduser2", "password": "password123", "invite": code,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("邀请码应只能用一次, 实际 %d", status)
	}

	status, invites := s.do(t, "GET", "/api/admin/invites", ownerToken, nil)
	if status != http.StatusOK || len(list(invites, "items")) != 1 {
		t.Fatalf("邀请码列表异常: %d %v", status, invites)
	}
	if list(invites, "items")[0].(map[string]any)["status"] != "used" {
		t.Fatalf("邀请码状态应为 used: %v", invites)
	}

	status, created := s.do(t, "POST", "/api/admin/users", ownerToken, map[string]any{
		"username": "createdbyadmin", "nickname": "后台创建", "password": "password123", "role": "admin",
	})
	if status != http.StatusCreated {
		t.Fatalf("管理员创建用户失败: %d %v", status, created)
	}
	status, _ = s.do(t, "POST", "/api/auth/login", "", map[string]string{
		"username": "createdbyadmin", "password": "password123",
	})
	if status != http.StatusOK {
		t.Fatalf("后台创建的用户不能登录: %d", status)
	}

	readerToken, _ := registered["token"].(string)
	status, _ = s.do(t, "POST", "/api/admin/invites", readerToken, map[string]any{"note": "越权"})
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能创建邀请码, 实际 %d", status)
	}
}

func TestMessagePagination(t *testing.T) {
	s := newTestAPI(t)
	senderToken, _ := s.register(t, "page_sender")
	receiverToken, _ := s.register(t, "page_receiver")
	for i := 0; i < 55; i++ {
		status, _ := s.do(t, "POST", "/api/messages/with/page_receiver", senderToken, map[string]string{
			"content": fmt.Sprintf("分页消息 %d", i+1),
		})
		if status != http.StatusCreated {
			t.Fatalf("发送第 %d 条消息失败: %d", i+1, status)
		}
	}
	status, first := s.do(t, "GET", "/api/messages/with/page_sender", receiverToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取消息失败: %d", status)
	}
	items := list(first, "items")
	if len(items) != 50 || first["hasMore"] != true {
		t.Fatalf("首批消息分页不正确: len=%d hasMore=%v", len(items), first["hasMore"])
	}
	oldest := uint(number(items[0].(map[string]any)["id"]))
	status, older := s.do(t, "GET", "/api/messages/with/page_sender?before="+itoa(oldest), receiverToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取更早消息失败: %d", status)
	}
	olderItems := list(older, "items")
	if len(olderItems) != 5 || older["hasMore"] != false {
		t.Fatalf("更早消息分页不正确: len=%d hasMore=%v", len(olderItems), older["hasMore"])
	}
}

func TestFeaturedAndPublicCollections(t *testing.T) {
	s := newTestAPI(t)
	aliceToken, _ := s.register(t, "collectalice")
	bobToken, _ := s.register(t, "collectbob")

	status, created := s.do(t, "POST", "/api/docs", aliceToken, map[string]any{
		"kind": "article", "title": "置顶文章", "content": "正文", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建文章失败: %d %v", status, created)
	}
	docID := uint(number(obj(created, "doc")["id"]))

	status, _ = s.do(t, "PUT", "/api/me/featured-doc", aliceToken, map[string]any{"docID": docID})
	if status != http.StatusOK {
		t.Fatalf("设置置顶文章失败: %d", status)
	}
	status, profile := s.do(t, "GET", "/api/users/collectalice", bobToken, nil)
	if status != http.StatusOK || obj(profile, "topDoc")["id"] != float64(docID) {
		t.Fatalf("主页置顶文章不正确: %d %v", status, profile)
	}
	status, _ = s.do(t, "PUT", "/api/me/featured-doc", bobToken, map[string]any{"docID": docID})
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能设置置顶文章, 实际 %d", status)
	}

	status, _ = s.do(t, "POST", "/api/docs/"+itoa(docID)+"/favorite", aliceToken, map[string]any{})
	if status != http.StatusOK {
		t.Fatalf("收藏失败: %d", status)
	}
	status, folders := s.do(t, "GET", "/api/users/collectalice/favorite-folders", bobToken, nil)
	if status != http.StatusOK || folders["hidden"] != true {
		t.Fatalf("未公开收藏时应隐藏: %d %v", status, folders)
	}
	status, _ = s.do(t, "PUT", "/api/me/settings", aliceToken, map[string]any{
		"openFollow": true, "openFans": true, "openPrivateChat": true,
		"openCommentMessage": true, "openDiggMessage": true, "openCollect": true, "homeStyle": "cover",
	})
	if status != http.StatusOK {
		t.Fatalf("公开收藏设置失败: %d", status)
	}
	status, profile = s.do(t, "GET", "/api/users/collectalice", bobToken, nil)
	if status != http.StatusOK || obj(profile, "topDoc")["id"] != float64(docID) {
		t.Fatalf("修改隐私设置后置顶文章丢失: %v", profile)
	}
	if number(obj(profile, "counts")["favorites"]) != 1 {
		t.Fatalf("公开收藏后收藏数不正确: %v", profile)
	}
	status, folders = s.do(t, "GET", "/api/users/collectalice/favorite-folders", bobToken, nil)
	if status != http.StatusOK || folders["hidden"] != false || len(list(folders, "items")) == 0 {
		t.Fatalf("公开收藏夹列表不正确: %d %v", status, folders)
	}
	folderID := uint(number(list(folders, "items")[0].(map[string]any)["id"]))
	status, favorites := s.do(t, "GET", "/api/users/collectalice/favorites?folderID="+itoa(folderID), "", nil)
	if status != http.StatusOK || len(list(favorites, "items")) != 1 || list(favorites, "items")[0].(map[string]any)["id"] != float64(docID) {
		t.Fatalf("公开收藏列表不正确: %d %v", status, favorites)
	}

	status, _ = s.do(t, "PUT", "/api/me/settings", aliceToken, map[string]any{
		"openFollow": true, "openFans": true, "openPrivateChat": true,
		"openCommentMessage": true, "openDiggMessage": true, "openCollect": false, "homeStyle": "cover",
	})
	if status != http.StatusOK {
		t.Fatalf("关闭公开收藏失败: %d", status)
	}
	status, favorites = s.do(t, "GET", "/api/users/collectalice/favorites", "", nil)
	if status != http.StatusOK || favorites["hidden"] != true {
		t.Fatalf("关闭后收藏应隐藏: %d %v", status, favorites)
	}
}

func TestRevisionsAndExport(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "revowner")
	readerToken, _ := s.register(t, "revreader")
	status, created := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "article", "title": "版本测试", "content": "第一版内容", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建版本测试文章失败: %d %v", status, created)
	}
	docID := uint(number(obj(created, "doc")["id"]))
	status, _ = s.do(t, "PUT", "/api/docs/"+itoa(docID), ownerToken, map[string]any{
		"title": "版本测试", "content": "第二版内容", "status": "published",
	})
	if status != http.StatusOK {
		t.Fatalf("更新文章失败: %d", status)
	}

	status, revisions := s.do(t, "GET", "/api/docs/"+itoa(docID)+"/revisions", ownerToken, nil)
	if status != http.StatusOK || len(list(revisions, "items")) != 1 {
		t.Fatalf("历史版本列表不正确: %d %v", status, revisions)
	}
	revisionID := uint(number(list(revisions, "items")[0].(map[string]any)["id"]))
	status, detail := s.do(t, "GET", "/api/docs/"+itoa(docID)+"/revisions/"+itoa(revisionID), ownerToken, nil)
	if status != http.StatusOK || obj(detail, "revision")["content"] != "第一版内容" {
		t.Fatalf("历史版本详情不正确: %d %v", status, detail)
	}
	status, _ = s.do(t, "GET", "/api/docs/"+itoa(docID)+"/revisions", readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能查看历史版本, 实际 %d", status)
	}

	status, restored := s.do(t, "POST", "/api/docs/"+itoa(docID)+"/revisions/"+itoa(revisionID)+"/restore", ownerToken, nil)
	if status != http.StatusOK || obj(restored, "doc")["content"] != "第一版内容" {
		t.Fatalf("恢复历史版本失败: %d %v", status, restored)
	}
	status, current := s.do(t, "GET", "/api/docs/"+itoa(docID), "", nil)
	if status != http.StatusOK || obj(current, "doc")["content"] != "第一版内容" {
		t.Fatalf("恢复后的内容不正确: %d %v", status, current)
	}

	status, headers, archive := s.raw(t, "GET", "/api/admin/export", ownerToken)
	if status != http.StatusOK || !strings.Contains(headers.Get("Content-Type"), "application/zip") || !strings.HasPrefix(archive, "PK") {
		t.Fatalf("导出内容异常: %d %s", status, headers.Get("Content-Type"))
	}
	if !strings.Contains(headers.Get("Content-Disposition"), "folio-export-") {
		t.Fatalf("导出文件名异常: %s", headers.Get("Content-Disposition"))
	}
	status, _, _ = s.raw(t, "GET", "/api/admin/export", readerToken)
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能导出内容, 实际 %d", status)
	}
}

func TestAIAskUsesUserKey(t *testing.T) {
	var seenKey, seenBody string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenKey = r.Header.Get("x-api-key")
		body, _ := io.ReadAll(r.Body)
		seenBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_qa","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"这是基于知识库的回答。"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":10}}`)
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.AIAPIKey = "site-key-must-not-be-used"
		cfg.AIBaseURL = fake.URL
	})
	ownerToken, _ := s.register(t, "qaowner")
	readerToken, _ := s.register(t, "qareader")
	status, created := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "article", "title": "知识库使用指南", "content": "这篇内容介绍知识库的整理方式。", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建问答资料失败: %d %v", status, created)
	}

	status, _ = s.do(t, "POST", "/api/ai/ask", readerToken, map[string]string{
		"question": "知识库怎么整理？", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("未提供用户 Key 时应拒绝，实际 %d", status)
	}
	if seenKey != "" {
		t.Fatalf("本站 Key 不应被用于问答: %s", seenKey)
	}

	status, answer := s.do(t, "POST", "/api/ai/ask", readerToken, map[string]string{
		"question": "知识库怎么整理？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusOK {
		t.Fatalf("AI 问答失败: %d %v", status, answer)
	}
	if answer["answer"] != "这是基于知识库的回答。" || len(list(answer, "sources")) == 0 {
		t.Fatalf("AI 问答结果不正确: %v", answer)
	}
	foundArticle := false
	for _, item := range list(answer, "sources") {
		source := item.(map[string]any)
		if source["kind"] == models.KindArticle && source["title"] == "知识库使用指南" {
			foundArticle = true
		}
	}
	if !foundArticle {
		t.Fatalf("引用来源应包含命中的文章: %v", answer["sources"])
	}
	if seenKey != "user-secret-key" {
		t.Fatalf("应使用用户提供的 Key，实际 %q", seenKey)
	}
	if !strings.Contains(seenBody, "知识库") {
		t.Fatalf("请求上下文没有检索到资料: %s", seenBody)
	}

	status, _ = s.do(t, "POST", "/api/ai/ask", "", map[string]string{
		"question": "知识库怎么整理？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("未登录不应能使用 AI 问答，实际 %d", status)
	}
}

func TestAIAskOpenAICompatibleProviders(t *testing.T) {
	var seenAuth, seenBody, seenPath string
	var calls int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		seenAuth = r.Header.Get("Authorization")
		seenPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		seenBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"这是兼容接口的回答。"}}]}`)
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.AIAPIKey = "site-key-must-not-be-used"
		cfg.DeepSeekBaseURL = fake.URL
		cfg.GLMBaseURL = fake.URL
		cfg.KimiBaseURL = fake.URL
		cfg.QwenBaseURL = fake.URL
	})
	token, _ := s.register(t, "compatreader")
	status, created := s.do(t, "POST", "/api/docs", token, map[string]any{
		"kind": "article", "title": "兼容接口知识库", "content": "这篇内容介绍多服务商问答的检索方式。", "status": "published",
	})
	if status != http.StatusCreated {
		t.Fatalf("创建问答资料失败: %d %v", status, created)
	}

	status, data := s.do(t, "GET", "/api/ai/providers", token, nil)
	if status != http.StatusOK {
		t.Fatalf("读取服务商失败: %d %v", status, data)
	}
	providers := list(data, "providers")
	if len(providers) != 5 {
		t.Fatalf("应有 5 个服务商，实际 %d", len(providers))
	}
	ids := map[string]bool{}
	for _, item := range providers {
		provider := item.(map[string]any)
		ids[provider["id"].(string)] = true
		if provider["baseURL"] != nil {
			t.Fatalf("服务商信息不应暴露内部地址: %v", provider)
		}
	}
	for _, id := range []string{"anthropic", "deepseek", "glm", "kimi", "qwen"} {
		if !ids[id] {
			t.Fatalf("缺少服务商 %s", id)
		}
	}

	cases := []struct {
		provider string
		model    string
	}{
		{"deepseek", "deepseek-chat"},
		{"glm", "glm-4.6"},
		{"kimi", "kimi-k2-0905-preview"},
		{"qwen", "qwen3-max"},
	}
	for _, tc := range cases {
		calls = 0
		status, answer := s.do(t, "POST", "/api/ai/ask", token, map[string]string{
			"question": "多服务商怎么检索？", "apiKey": "user-secret-key",
			"provider": tc.provider, "model": tc.model,
		})
		if status != http.StatusOK {
			t.Fatalf("%s 问答失败: %d %v", tc.provider, status, answer)
		}
		if answer["answer"] != "这是兼容接口的回答。" || answer["provider"] != tc.provider {
			t.Fatalf("%s 返回不正确: %v", tc.provider, answer)
		}
		if calls != 1 || seenAuth != "Bearer user-secret-key" {
			t.Fatalf("%s 应使用用户 Key，实际 calls=%d auth=%q", tc.provider, calls, seenAuth)
		}
		if seenPath != "/chat/completions" {
			t.Fatalf("%s 请求路径不正确: %s", tc.provider, seenPath)
		}
		if !strings.Contains(seenBody, tc.model) || !strings.Contains(seenBody, "多服务商问答") {
			t.Fatalf("%s 请求缺少模型或检索资料: %s", tc.provider, seenBody)
		}
	}

	calls = 0
	status, answer := s.do(t, "POST", "/api/ai/ask", token, map[string]string{
		"question": "多服务商怎么检索？", "apiKey": "user-secret-key",
		"provider": "deepseek", "model": "deepseek-next-preview",
	})
	if status != http.StatusOK {
		t.Fatalf("自定义模型应允许: %d %v", status, answer)
	}

	calls = 0
	status, _ = s.do(t, "POST", "/api/ai/ask", token, map[string]string{
		"question": "多服务商怎么检索？", "apiKey": "user-secret-key", "provider": "unknown",
	})
	if status != http.StatusBadRequest || calls != 0 {
		t.Fatalf("未知服务商应在本地拒绝，实际 %d calls=%d", status, calls)
	}

	calls = 0
	status, _ = s.do(t, "POST", "/api/ai/ask", token, map[string]string{
		"question": "多服务商怎么检索？", "provider": "deepseek", "model": "deepseek-chat",
	})
	if status != http.StatusBadRequest || calls != 0 {
		t.Fatalf("未提供用户 Key 时不应调用上游，实际 %d calls=%d", status, calls)
	}
}

func TestAdminStorageSettings(t *testing.T) {
	s := newTestAPI(t)
	adminToken, _ := s.register(t, "storageadmin")

	status, data := s.do(t, "GET", "/api/admin/storage", adminToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取存储设置失败: %d %v", status, data)
	}
	storage := obj(data, "storage")
	if storage["provider"] != "local" || storage["active"] != "local" || storage["source"] != "none" {
		t.Fatalf("默认存储状态不正确: %v", storage)
	}

	payload := map[string]any{
		"provider":       "qiniu",
		"qiniuAccessKey": "storage-access",
		"qiniuSecretKey": "storage-secret",
		"qiniuBucket":    "bucket-one",
		"qiniuDomain":    "cdn.example.com",
		"qiniuRegion":    "z0",
		"qiniuPrefix":    "folio",
	}
	status, data = s.do(t, "PUT", "/api/admin/storage", adminToken, payload)
	if status != http.StatusOK {
		t.Fatalf("保存七牛配置失败: %d %v", status, data)
	}
	storage = obj(data, "storage")
	if storage["provider"] != "qiniu" || storage["active"] != "qiniu" || storage["source"] != "database" {
		t.Fatalf("七牛配置未生效: %v", storage)
	}
	if storage["hasAccessKey"] != true || storage["hasSecretKey"] != true {
		t.Fatalf("应显示密钥已配置: %v", storage)
	}
	encoded, _ := json.Marshal(data)
	if strings.Contains(string(encoded), "storage-access") || strings.Contains(string(encoded), "storage-secret") {
		t.Fatalf("接口不应回显密钥: %s", encoded)
	}

	var row models.StorageSetting
	if err := s.api.DB.First(&row, 1).Error; err != nil {
		t.Fatalf("读取存储设置记录失败: %v", err)
	}
	if !strings.HasPrefix(row.QiniuAccessKey, "enc:v1:") || strings.Contains(row.QiniuAccessKey, "storage-access") {
		t.Fatalf("AccessKey 应加密保存: %s", row.QiniuAccessKey)
	}
	access, err := secure.Decrypt("test-secret", row.QiniuAccessKey)
	if err != nil || access != "storage-access" {
		t.Fatalf("AccessKey 解密不正确: %q %v", access, err)
	}

	// 留空密钥表示保持不变。
	status, data = s.do(t, "PUT", "/api/admin/storage", adminToken, map[string]any{
		"provider":    "qiniu",
		"qiniuBucket": "bucket-two",
		"qiniuDomain": "https://cdn2.example.com",
		"qiniuRegion": "z0",
		"qiniuPrefix": "folio",
	})
	if status != http.StatusOK {
		t.Fatalf("留空密钥更新失败: %d %v", status, data)
	}
	row = models.StorageSetting{}
	if err := s.api.DB.First(&row, 1).Error; err != nil {
		t.Fatalf("读取更新后的存储设置失败: %v", err)
	}
	access, err = secure.Decrypt("test-secret", row.QiniuAccessKey)
	if err != nil || access != "storage-access" || row.QiniuBucket != "bucket-two" {
		t.Fatalf("留空密钥时应保留原密钥: access=%q bucket=%q err=%v", access, row.QiniuBucket, err)
	}

	status, data = s.do(t, "PUT", "/api/admin/storage", adminToken, map[string]any{
		"provider":    "local",
		"qiniuBucket": "bucket-two",
		"qiniuDomain": "https://cdn2.example.com",
		"qiniuRegion": "z0",
		"qiniuPrefix": "folio",
	})
	if status != http.StatusOK || obj(data, "storage")["active"] != "local" {
		t.Fatalf("切回本地失败: %d %v", status, data)
	}
	if obj(data, "storage")["hasAccessKey"] != true {
		t.Fatalf("切回本地后应保留七牛密钥: %v", data)
	}

	status, data = s.do(t, "PUT", "/api/admin/storage", adminToken, map[string]any{
		"provider":    "qiniu",
		"qiniuBucket": "bucket-three",
		"qiniuDomain": "cdn3.example.com",
		"qiniuRegion": "z0",
		"qiniuPrefix": "folio",
	})
	if status != http.StatusOK || obj(data, "storage")["active"] != "qiniu" {
		t.Fatalf("用已保存密钥切回七牛失败: %d %v", status, data)
	}

	status, _ = s.do(t, "POST", "/api/admin/storage/test", adminToken, map[string]any{"provider": "local"})
	if status != http.StatusBadRequest {
		t.Fatalf("本地存储不应做云连接测试，实际 %d", status)
	}

	status, _ = s.do(t, "PUT", "/api/admin/storage", adminToken, map[string]any{"provider": "s3"})
	if status != http.StatusBadRequest {
		t.Fatalf("未知存储类型应拒绝，实际 %d", status)
	}

	readerToken, _ := s.register(t, "storagereader")
	status, _ = s.do(t, "GET", "/api/admin/storage", readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("普通用户不应读取存储设置，实际 %d", status)
	}
}

func TestStorageSettingsEnvFallback(t *testing.T) {
	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.StorageProvider = "qiniu"
		cfg.QiniuAccessKey = "env-access"
		cfg.QiniuSecretKey = "env-secret"
		cfg.QiniuBucket = "env-bucket"
		cfg.QiniuDomain = "cdn.env.example.com"
		cfg.QiniuPrefix = "env"
	})
	token, _ := s.register(t, "envstorage")
	status, data := s.do(t, "GET", "/api/admin/storage", token, nil)
	if status != http.StatusOK {
		t.Fatalf("读取环境变量存储配置失败: %d %v", status, data)
	}
	storage := obj(data, "storage")
	if storage["source"] != "env" || storage["active"] != "qiniu" || storage["hasAccessKey"] != true {
		t.Fatalf("环境变量配置未正确显示: %v", storage)
	}
	status, data = s.do(t, "PUT", "/api/admin/storage", token, map[string]any{
		"provider":    "local",
		"qiniuBucket": "env-bucket",
		"qiniuDomain": "https://cdn.env.example.com",
		"qiniuPrefix": "env",
	})
	if status != http.StatusOK || obj(data, "storage")["active"] != "local" {
		t.Fatalf("从环境变量切换本地失败: %d %v", status, data)
	}
	if obj(data, "storage")["source"] != "database" {
		t.Fatalf("保存后应以数据库配置为准: %v", data)
	}
}

func TestStorageSettingsRejectsIncomplete(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "incompletestorage")
	status, _ := s.do(t, "PUT", "/api/admin/storage", token, map[string]any{
		"provider":    "qiniu",
		"qiniuBucket": "bucket",
		"qiniuDomain": "cdn.example.com",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("缺少七牛密钥时应拒绝，实际 %d", status)
	}
}

func TestAIHelpRetrievalAndModes(t *testing.T) {
	var calls int
	var seenBody string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		body, _ := io.ReadAll(r.Body)
		seenBody = string(body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_help","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"按步骤操作即可。"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":10}}`)
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) { cfg.AIBaseURL = fake.URL })
	adminToken, _ := s.register(t, "helpstaff")
	readerToken, _ := s.register(t, "helpreader")

	status, answer := s.do(t, "POST", "/api/ai/ask", readerToken, map[string]any{
		"question": "怎么邀请注册？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusOK {
		t.Fatalf("帮助问答失败: %d %v", status, answer)
	}
	foundHelp := false
	for _, item := range list(answer, "sources") {
		source := item.(map[string]any)
		if source["kind"] == "help" && source["title"] == "邀请注册与管理员建号" {
			foundHelp = true
		}
	}
	if !foundHelp {
		t.Fatalf("应检索到邀请注册的站点帮助: %v", answer["sources"])
	}
	if !strings.Contains(seenBody, "邀请码") || !strings.Contains(seenBody, "站点帮助") {
		t.Fatalf("请求上下文应包含站点帮助: %s", seenBody)
	}

	calls = 0
	status, answer = s.do(t, "POST", "/api/ai/ask", readerToken, map[string]any{
		"question": "第一个问题：火星土壤的化学成分是什么？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5", "mode": "strict",
	})
	if status != http.StatusOK || calls != 0 {
		t.Fatalf("严格模式无资料时不应调用上游: status=%d calls=%d %v", status, calls, answer)
	}
	if len(list(answer, "sources")) != 0 {
		t.Fatalf("严格模式无资料时不应返回来源: %v", answer)
	}

	calls = 0
	status, answer = s.do(t, "POST", "/api/ai/ask", readerToken, map[string]any{
		"question": "第一个问题：火星土壤的化学成分是什么？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5", "mode": "expanded",
	})
	if status != http.StatusOK || calls != 1 {
		t.Fatalf("通用知识模式应调用上游: status=%d calls=%d %v", status, calls, answer)
	}
	if !strings.Contains(seenBody, "没有检索到站内资料") {
		t.Fatalf("通用知识模式上下文不正确: %s", seenBody)
	}

	status, answer = s.do(t, "POST", "/api/ai/ask", readerToken, map[string]any{
		"question": "七牛云存储怎么配置？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5", "mode": "expanded",
		"history": []map[string]string{
			{"role": "user", "content": "先问一句"},
			{"role": "assistant", "content": "先答一句"},
		},
	})
	if status != http.StatusOK {
		t.Fatalf("带历史问答失败: %d %v", status, answer)
	}
	for _, item := range list(answer, "sources") {
		source := item.(map[string]any)
		if source["kind"] == "help" && source["title"] == "存储设置" {
			t.Fatalf("普通读者不应检索到管理员存储帮助: %v", answer["sources"])
		}
	}
	if !strings.Contains(seenBody, "先问一句") || !strings.Contains(seenBody, "先答一句") {
		t.Fatalf("多轮历史没有传给模型: %s", seenBody)
	}

	status, answer = s.do(t, "POST", "/api/ai/ask", adminToken, map[string]any{
		"question": "七牛云存储怎么配置？", "apiKey": "user-secret-key", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusOK {
		t.Fatalf("管理员帮助问答失败: %d %v", status, answer)
	}
	foundStaffHelp := false
	for _, item := range list(answer, "sources") {
		source := item.(map[string]any)
		if source["kind"] == "help" && source["title"] == "存储设置" {
			foundStaffHelp = true
		}
	}
	if !foundStaffHelp {
		t.Fatalf("管理员应检索到存储设置帮助: %v", answer["sources"])
	}

	status, meta := s.do(t, "GET", "/api/ai/meta", readerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("读取 AI 知识库信息失败: %d %v", status, meta)
	}
	if number(obj(meta, "knowledge")["help"]) <= 0 || len(list(meta, "modes")) != 2 {
		t.Fatalf("AI 知识库信息不完整: %v", meta)
	}
}

func TestSiteFeaturesCatalog(t *testing.T) {
	s := newTestAPI(t)
	status, data := s.do(t, "GET", "/api/site/features", "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取站点功能目录失败: %d %v", status, data)
	}
	catalog := obj(data, "catalog")
	if catalog["version"] == "" || len(list(catalog, "categories")) < 8 {
		t.Fatalf("站点功能目录不完整: %v", catalog)
	}
	found := false
	for _, item := range list(catalog, "categories") {
		category := item.(map[string]any)
		for _, f := range list(category, "features") {
			feature := f.(map[string]any)
			if feature["id"] == "invite-register" {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("站点功能目录缺少邀请注册")
	}
}

func TestAIAskStream(t *testing.T) {
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "user-stream-key" {
			t.Errorf("流式请求没有使用用户 Key: %q", r.Header.Get("x-api-key"))
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "event: message_start\n")
		fmt.Fprint(w, `data: {"type":"message_start","message":{"id":"msg_stream","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":0}}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_start\n")
		fmt.Fprint(w, `data: {"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"流式"}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_delta\n")
		fmt.Fprint(w, `data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"回答"}}`+"\n\n")
		fmt.Fprint(w, "event: content_block_stop\n")
		fmt.Fprint(w, `data: {"type":"content_block_stop","index":0}`+"\n\n")
		fmt.Fprint(w, "event: message_delta\n")
		fmt.Fprint(w, `data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":2}}`+"\n\n")
		fmt.Fprint(w, "event: message_stop\n")
		fmt.Fprint(w, `data: {"type":"message_stop"}`+"\n\n")
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) { cfg.AIBaseURL = fake.URL })
	readerToken, _ := s.register(t, "streamreader")
	payload, _ := json.Marshal(map[string]any{
		"question": "怎么邀请注册？", "apiKey": "user-stream-key", "model": "claude-sonnet-5-5",
	})
	req := httptest.NewRequest("POST", "/api/ai/ask/stream", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+readerToken)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("流式问答失败: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"event: meta", "邀请注册与管理员建号", "event: delta", "流式", "回答", "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("流式响应缺少 %q: %s", want, body)
		}
	}
}

func TestAIAskStreamOpenAICompatible(t *testing.T) {
	var seenAuth string
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"兼容"}}]}`+"\n\n")
		fmt.Fprint(w, `data: {"choices":[{"delta":{"content":"流式"}}]}`+"\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) { cfg.DeepSeekBaseURL = fake.URL })
	readerToken, _ := s.register(t, "streamcompat")
	payload, _ := json.Marshal(map[string]any{
		"question": "怎么邀请注册？", "apiKey": "user-compat-key", "provider": "deepseek", "model": "deepseek-chat",
	})
	req := httptest.NewRequest("POST", "/api/ai/ask/stream", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+readerToken)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("兼容流式问答失败: %d %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if seenAuth != "Bearer user-compat-key" {
		t.Fatalf("兼容流式请求没有使用用户 Key: %q", seenAuth)
	}
	for _, want := range []string{"event: meta", "event: delta", "兼容", "流式", "event: done"} {
		if !strings.Contains(body, want) {
			t.Fatalf("兼容流式响应缺少 %q: %s", want, body)
		}
	}
}

func TestAccessSettings(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "accessowner")

	status, data := s.do(t, "GET", "/api/admin/access", token, nil)
	if status != http.StatusOK {
		t.Fatalf("读取注册设置失败: %d %v", status, data)
	}
	if data["allowRegister"] != true || data["qqLogin"] != false || data["qqConfigured"] != false {
		t.Fatalf("默认注册设置不正确: %v", data)
	}

	status, data = s.do(t, "PUT", "/api/admin/access", token, map[string]any{
		"allowRegister": false, "qqLogin": true,
	})
	if status != http.StatusBadRequest {
		t.Fatalf("未配置凭据时不应允许开启 QQ 登录: %d %v", status, data)
	}

	status, data = s.do(t, "PUT", "/api/admin/access", token, map[string]any{
		"allowRegister": false, "qqLogin": false,
	})
	if status != http.StatusOK || obj(data, "access")["allowRegister"] != false {
		t.Fatalf("保存注册开关失败: %d %v", status, data)
	}

	// 保存其他站点设置时，不应把未提交的注册开关重置。
	status, data = s.do(t, "PUT", "/api/admin/settings", token, map[string]any{
		"siteName": "测试站", "siteDesc": "描述", "siteOwner": "tester", "loginCaptcha": false,
	})
	if status != http.StatusOK || obj(data, "settings")["allowRegister"] != false {
		t.Fatalf("更新站点设置不应重置注册开关: %d %v", status, data)
	}

	status, _ = s.do(t, "POST", "/api/auth/register", "", map[string]string{
		"username": "closedreader", "nickname": "closedreader", "email": "closed@example.com", "password": "password123",
	})
	if status != http.StatusForbidden {
		t.Fatalf("关闭注册后新用户应被拒绝，实际 %d", status)
	}
}

func TestQQLoginSwitch(t *testing.T) {
	s := newTestAPI(t, func(cfg *config.Config) {
		cfg.QQAppID = "qq-app"
		cfg.QQAppKey = "qq-key"
		cfg.QQRedirect = "http://example.com/api/auth/qq/callback"
	})
	token, _ := s.register(t, "qqswitch")

	status, data := s.do(t, "GET", "/api/admin/access", token, nil)
	if status != http.StatusOK || data["qqConfigured"] != true || data["qqLogin"] != true {
		t.Fatalf("配置凭据后 QQ 登录应默认开启: %d %v", status, data)
	}
	status, data = s.do(t, "GET", "/api/auth/qq/status", "", nil)
	if status != http.StatusOK || data["enabled"] != true {
		t.Fatalf("QQ 登录状态应为开启: %d %v", status, data)
	}

	status, data = s.do(t, "PUT", "/api/admin/access", token, map[string]any{
		"allowRegister": true, "qqLogin": false,
	})
	if status != http.StatusOK {
		t.Fatalf("关闭 QQ 登录失败: %d %v", status, data)
	}
	status, data = s.do(t, "GET", "/api/auth/qq/status", "", nil)
	if status != http.StatusOK || data["enabled"] != false {
		t.Fatalf("QQ 登录状态应为关闭: %d %v", status, data)
	}
	status, _ = s.do(t, "GET", "/api/auth/qq/url", "", nil)
	if status != http.StatusServiceUnavailable {
		t.Fatalf("关闭后 QQ 授权入口应不可用，实际 %d", status)
	}

	status, data = s.do(t, "PUT", "/api/admin/access", token, map[string]any{
		"allowRegister": true, "qqLogin": true,
	})
	if status != http.StatusOK {
		t.Fatalf("重新开启 QQ 登录失败: %d %v", status, data)
	}
	status, data = s.do(t, "GET", "/api/auth/qq/status", "", nil)
	if status != http.StatusOK || data["enabled"] != true {
		t.Fatalf("QQ 登录状态应重新开启: %d %v", status, data)
	}
}

func TestTagCountsByKind(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "tagcountowner")
	for _, doc := range []map[string]any{
		{"kind": "article", "title": "带共用标签的文章", "content": "正文", "tags": []string{"共用"}, "status": "published"},
		{"kind": "doc", "title": "带共用标签的文档", "content": "正文", "tags": []string{"共用"}, "status": "published"},
		{"kind": "article", "title": "只有文章标签", "content": "正文", "tags": []string{"只文章"}, "status": "published"},
	} {
		status, created := s.do(t, "POST", "/api/docs", token, doc)
		if status != http.StatusCreated {
			t.Fatalf("创建标签测试内容失败: %d %v", status, created)
		}
	}
	counts := func(query string) map[string]int {
		status, data := s.do(t, "GET", "/api/tags"+query, "", nil)
		if status != http.StatusOK {
			t.Fatalf("读取标签失败: %d %v", status, data)
		}
		out := map[string]int{}
		for _, item := range list(data, "items") {
			tag := item.(map[string]any)
			out[tag["name"].(string)] = int(number(tag["count"]))
		}
		return out
	}
	articles := counts("?kind=article")
	if articles["共用"] != 1 || articles["只文章"] != 1 {
		t.Fatalf("文章标签计数不正确: %v", articles)
	}
	docs := counts("?kind=doc")
	if docs["共用"] != 1 || docs["只文章"] != 0 {
		t.Fatalf("文档标签计数不正确: %v", docs)
	}
	all := counts("")
	if all["共用"] != 2 {
		t.Fatalf("全站标签计数不正确: %v", all)
	}
}

func TestAIKeysPersistence(t *testing.T) {
	var seenKey string
	var calls int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		seenKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"msg_key","type":"message","role":"assistant","model":"claude-sonnet-5-5","content":[{"type":"text","text":"已使用保存的 Key。"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":10}}`)
	}))
	defer fake.Close()

	s := newTestAPI(t, func(cfg *config.Config) { cfg.AIBaseURL = fake.URL })
	token, _ := s.register(t, "keyowner")
	status, data := s.do(t, "GET", "/api/ai/keys", token, nil)
	if status != http.StatusOK {
		t.Fatalf("读取已保存 Key 失败: %d %v", status, data)
	}
	if saved := obj(data, "saved"); len(saved) != 0 {
		t.Fatalf("初始不应有保存的 Key: %v", data)
	}

	status, data = s.do(t, "PUT", "/api/ai/keys", token, map[string]string{
		"provider": "anthropic", "apiKey": "stored-user-key",
	})
	if status != http.StatusOK {
		t.Fatalf("保存 AI Key 失败: %d %v", status, data)
	}
	var row models.AIProviderKey
	if err := s.api.DB.Where("provider = ?", "anthropic").First(&row).Error; err != nil {
		t.Fatalf("读取 AI Key 记录失败: %v", err)
	}
	if !strings.HasPrefix(row.Key, "enc:v1:") || strings.Contains(row.Key, "stored-user-key") {
		t.Fatalf("AI Key 应加密保存: %s", row.Key)
	}
	status, data = s.do(t, "GET", "/api/ai/keys", token, nil)
	if status != http.StatusOK || obj(data, "saved")["anthropic"] != true {
		t.Fatalf("已保存 Key 状态不正确: %d %v", status, data)
	}

	status, answer := s.do(t, "POST", "/api/ai/ask", token, map[string]string{
		"question": "怎么邀请注册？", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusOK || calls != 1 || seenKey != "stored-user-key" {
		t.Fatalf("应使用已保存的 Key: status=%d calls=%d key=%q %v", status, calls, seenKey, answer)
	}

	status, data = s.do(t, "DELETE", "/api/ai/keys/anthropic", token, nil)
	if status != http.StatusOK || obj(data, "saved")["anthropic"] == true {
		t.Fatalf("删除已保存 Key 失败: %d %v", status, data)
	}
	status, _ = s.do(t, "POST", "/api/ai/ask", token, map[string]string{
		"question": "怎么邀请注册？", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusBadRequest || calls != 1 {
		t.Fatalf("删除后缺少 Key 应拒绝，实际 status=%d calls=%d", status, calls)
	}

	reader, _ := s.register(t, "keyreader")
	status, data = s.do(t, "GET", "/api/ai/keys", reader, nil)
	if status != http.StatusOK || len(obj(data, "saved")) != 0 {
		t.Fatalf("用户之间不应共享 Key: %d %v", status, data)
	}
	status, _ = s.do(t, "PUT", "/api/ai/keys", token, map[string]string{"provider": "unknown", "apiKey": "stored-user-key"})
	if status != http.StatusBadRequest {
		t.Fatalf("未知服务商应拒绝，实际 %d", status)
	}
}

func TestPrivateDocuments(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "privacyowner")
	readerToken, _ := s.register(t, "privacyreader")

	create := func(body map[string]any) uint {
		status, data := s.do(t, "POST", "/api/docs", ownerToken, body)
		if status != http.StatusCreated {
			t.Fatalf("创建私密测试文档失败: %d %v", status, data)
		}
		return uint(number(obj(data, "doc")["id"]))
	}

	privateRoot := create(map[string]any{
		"kind": "doc", "title": "量子土豆根", "content": "量子土豆根的私密内容。",
		"tags": []string{"私密测试"}, "status": "published", "private": true,
	})
	publicChild := create(map[string]any{
		"kind": "doc", "title": "私密根下的公开子文档", "content": "子文档内容。",
		"status": "published", "parentID": privateRoot, "private": false,
	})
	publicParent := create(map[string]any{
		"kind": "doc", "title": "公开父文档", "content": "公开父文档内容。",
		"status": "published", "private": false,
	})
	privateChild := create(map[string]any{
		"kind": "doc", "title": "公开父下的私密子文档", "content": "私密子文档内容。",
		"status": "published", "parentID": publicParent, "private": true,
	})

	listIDs := func(token string) map[uint]bool {
		status, data := s.do(t, "GET", "/api/docs?kind=doc&pageSize=100", token, nil)
		if status != http.StatusOK {
			t.Fatalf("读取文档列表失败: %d %v", status, data)
		}
		ids := map[uint]bool{}
		for _, item := range list(data, "items") {
			ids[uint(number(item.(map[string]any)["id"]))] = true
		}
		return ids
	}
	readerIDs := listIDs(readerToken)
	if readerIDs[privateRoot] || readerIDs[publicChild] || readerIDs[privateChild] {
		t.Fatalf("读者不应看到私密文档或私密父级下的子文档: %v", readerIDs)
	}
	if !readerIDs[publicParent] {
		t.Fatalf("公开父文档应可见: %v", readerIDs)
	}
	ownerIDs := listIDs(ownerToken)
	for _, id := range []uint{privateRoot, publicChild, publicParent, privateChild} {
		if !ownerIDs[id] {
			t.Fatalf("作者应看到自己的文档 %d: %v", id, ownerIDs)
		}
	}

	for _, id := range []uint{privateRoot, publicChild, privateChild} {
		status, _ := s.do(t, "GET", fmt.Sprintf("/api/docs/%d", id), readerToken, nil)
		if status != http.StatusForbidden {
			t.Fatalf("读者访问私密文档 %d 应返回 403 并提供申请入口，实际 %d", id, status)
		}
	}
	status, _ := s.do(t, "GET", fmt.Sprintf("/api/docs/%d", privateRoot), ownerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("作者应能访问自己的私密文档，实际 %d", status)
	}
	status, _ = s.do(t, "GET", fmt.Sprintf("/api/docs/%d", publicParent), "", nil)
	if status != http.StatusOK {
		t.Fatalf("公开父文档应允许匿名访问，实际 %d", status)
	}

	status, treeData := s.do(t, "GET", "/api/doc-tree", "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取目录树失败: %d %v", status, treeData)
	}
	treeIDs := map[uint]bool{}
	for _, item := range list(treeData, "items") {
		treeIDs[uint(number(item.(map[string]any)["id"]))] = true
	}
	if treeIDs[privateRoot] || treeIDs[publicChild] || treeIDs[privateChild] || !treeIDs[publicParent] {
		t.Fatalf("目录树可见性不正确: %v", treeIDs)
	}

	status, search := s.do(t, "GET", "/api/docs?kind=doc&q=%E9%87%8F%E5%AD%90%E5%9C%9F%E8%B1%86%E6%A0%B9", readerToken, nil)
	if status != http.StatusOK || number(search["total"]) != 0 {
		t.Fatalf("搜索不应命中私密文档: %d %v", status, search)
	}

	status, _, sitemap := s.raw(t, "GET", "/sitemap.xml", "")
	if status != http.StatusOK || strings.Contains(sitemap, "量子土豆根") || strings.Contains(sitemap, "私密根下的公开子文档") || strings.Contains(sitemap, "公开父下的私密子文档") {
		t.Fatalf("sitemap 不应包含私密文档: %s", sitemap)
	}

	for _, path := range []string{"/api/docs/%d/like", "/api/docs/%d/favorite"} {
		status, _ = s.do(t, "POST", fmt.Sprintf(path, privateRoot), readerToken, nil)
		if status != http.StatusNotFound {
			t.Fatalf("读者不应能操作私密文档 %s，实际 %d", path, status)
		}
	}
	status, _ = s.do(t, "POST", fmt.Sprintf("/api/docs/%d/comments", privateRoot), readerToken, map[string]string{"content": "不该出现"})
	if status != http.StatusNotFound {
		t.Fatalf("读者不应能评论私密文档，实际 %d", status)
	}

	status, data := s.do(t, "POST", "/api/ai/ask", readerToken, map[string]string{
		"question": "量子土豆根是什么？", "apiKey": "sk-test-invalid-key", "model": "claude-sonnet-5-5",
	})
	if status != http.StatusOK || len(list(data, "sources")) != 0 {
		t.Fatalf("AI 检索不应返回私密文档: %d %v", status, data)
	}
}

func TestPrivateAccessRequests(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "accessowner2")
	readerToken, readerID := s.register(t, "accessreader2")

	status, created := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "doc", "title": "私密申请测试根", "content": "私密根内容。", "status": "published", "private": true,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建私密根文档失败: %d %v", status, created)
	}
	rootID := uint(number(obj(created, "doc")["id"]))
	status, childCreated := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "doc", "title": "私密根下的测试子文档", "content": "子文档内容。", "status": "published", "parentID": rootID,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建私密子文档失败: %d %v", status, childCreated)
	}
	childID := uint(number(obj(childCreated, "doc")["id"]))

	status, _ = s.do(t, "GET", fmt.Sprintf("/api/docs/%d", rootID), readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("读者访问私密文档应返回 403，实际 %d", status)
	}
	status, info := s.do(t, "GET", fmt.Sprintf("/api/docs/%d/access", rootID), readerToken, nil)
	if status != http.StatusOK || info["private"] != true || info["canView"] != false || info["status"] != "none" {
		t.Fatalf("阅读申请信息不正确: %d %v", status, info)
	}
	if obj(info, "owner")["username"] != "accessowner2" {
		t.Fatalf("申请人信息不正确: %v", info)
	}

	status, req := s.do(t, "POST", fmt.Sprintf("/api/docs/%d/access-requests", childID), readerToken, map[string]string{
		"note": "想学习这部分内容",
	})
	if status != http.StatusOK || req["status"] != "pending" {
		t.Fatalf("提交阅读申请失败: %d %v", status, req)
	}
	var notice models.Notification
	if err := s.api.DB.Where("type = ?", models.NotifyAccessRequest).Order("id DESC").First(&notice).Error; err != nil {
		t.Fatalf("作者没有收到阅读申请通知: %v", err)
	}
	if notice.DocID == nil || *notice.DocID != rootID {
		t.Fatalf("阅读申请通知应指向私密范围根文档: %v", notice)
	}

	status, listData := s.do(t, "GET", "/api/me/access-requests", ownerToken, nil)
	if status != http.StatusOK || len(list(listData, "items")) != 1 {
		t.Fatalf("作者阅读申请列表不正确: %d %v", status, listData)
	}
	requestID := uint(number(list(listData, "items")[0].(map[string]any)["id"]))
	status, _ = s.do(t, "PUT", fmt.Sprintf("/api/access-requests/%d", requestID), readerToken, map[string]string{"action": "approve"})
	if status != http.StatusForbidden {
		t.Fatalf("非作者不应处理阅读申请，实际 %d", status)
	}
	status, result := s.do(t, "PUT", fmt.Sprintf("/api/access-requests/%d", requestID), ownerToken, map[string]string{"action": "approve"})
	if status != http.StatusOK || result["status"] != models.AccessApproved {
		t.Fatalf("通过阅读申请失败: %d %v", status, result)
	}
	for _, id := range []uint{rootID, childID} {
		status, _ = s.do(t, "GET", fmt.Sprintf("/api/docs/%d", id), readerToken, nil)
		if status != http.StatusOK {
			t.Fatalf("通过后应能访问私密范围 %d，实际 %d", id, status)
		}
	}
	status, info = s.do(t, "GET", fmt.Sprintf("/api/docs/%d/access", rootID), readerToken, nil)
	if status != http.StatusOK || info["canView"] != true {
		t.Fatalf("通过后 access 信息不正确: %d %v", status, info)
	}
	status, _ = s.do(t, "POST", fmt.Sprintf("/api/docs/%d/access-requests", rootID), readerToken, map[string]string{})
	if status != http.StatusBadRequest {
		t.Fatalf("已有权限后重复申请应拒绝，实际 %d", status)
	}

	var resultNotice models.Notification
	if err := s.api.DB.Where("type = ? AND user_id = ?", models.NotifyAccessResult, readerID).Order("id DESC").First(&resultNotice).Error; err != nil {
		var all []models.Notification
		s.api.DB.Order("id DESC").Limit(10).Find(&all)
		t.Fatalf("申请人没有收到处理结果通知: %v; notifications=%v", err, all)
	}
	if resultNotice.DocID == nil || *resultNotice.DocID != rootID {
		t.Fatalf("通过通知应带回私密范围链接: %v", resultNotice)
	}

	status, result = s.do(t, "PUT", fmt.Sprintf("/api/access-requests/%d", requestID), ownerToken, map[string]string{"action": "deny"})
	if status != http.StatusOK || result["status"] != models.AccessDenied {
		t.Fatalf("拒绝阅读申请失败: %d %v", status, result)
	}
	status, _ = s.do(t, "GET", fmt.Sprintf("/api/docs/%d", rootID), readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("拒绝后应撤销访问权限，实际 %d", status)
	}
	status, req = s.do(t, "POST", fmt.Sprintf("/api/docs/%d/access-requests", rootID), readerToken, map[string]string{"note": "再申请一次"})
	if status != http.StatusOK || req["status"] != models.AccessPending {
		t.Fatalf("被拒后应允许重新申请: %d %v", status, req)
	}
}

func TestAccessRequestWithdrawAndRevoke(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "accessowner3")
	readerToken, _ := s.register(t, "accessreader3")
	status, created := s.do(t, "POST", "/api/docs", ownerToken, map[string]any{
		"kind": "doc", "title": "撤回申请测试", "content": "私密内容。", "status": "published", "private": true,
	})
	if status != http.StatusCreated {
		t.Fatalf("创建私密文档失败: %d %v", status, created)
	}
	docID := uint(number(obj(created, "doc")["id"]))
	status, _ = s.do(t, "POST", fmt.Sprintf("/api/docs/%d/access-requests", docID), readerToken, map[string]string{"note": "第一版说明"})
	if status != http.StatusOK {
		t.Fatalf("提交申请失败: %d", status)
	}
	status, outgoing := s.do(t, "GET", "/api/me/access-requests/outgoing", readerToken, nil)
	if status != http.StatusOK || len(list(outgoing, "items")) != 1 {
		t.Fatalf("我的申请列表不正确: %d %v", status, outgoing)
	}
	requestID := uint(number(list(outgoing, "items")[0].(map[string]any)["id"]))
	status, result := s.do(t, "DELETE", fmt.Sprintf("/api/access-requests/%d", requestID), readerToken, nil)
	if status != http.StatusOK || result["status"] != models.AccessCancelled {
		t.Fatalf("撤回申请失败: %d %v", status, result)
	}
	status, outgoing = s.do(t, "GET", "/api/me/access-requests/outgoing", readerToken, nil)
	if status != http.StatusOK || list(outgoing, "items")[0].(map[string]any)["status"] != models.AccessCancelled {
		t.Fatalf("撤回状态不正确: %d %v", status, outgoing)
	}

	status, _ = s.do(t, "POST", fmt.Sprintf("/api/docs/%d/access-requests", docID), readerToken, map[string]string{"note": "第二版说明"})
	if status != http.StatusOK {
		t.Fatalf("重新申请失败: %d", status)
	}
	status, listData := s.do(t, "GET", "/api/me/access-requests", ownerToken, nil)
	if status != http.StatusOK || len(list(listData, "items")) != 1 {
		t.Fatalf("作者申请列表不正确: %d %v", status, listData)
	}
	requestID = uint(number(list(listData, "items")[0].(map[string]any)["id"]))
	status, _ = s.do(t, "PUT", fmt.Sprintf("/api/access-requests/%d", requestID), ownerToken, map[string]string{"action": "approve"})
	if status != http.StatusOK {
		t.Fatalf("通过申请失败: %d", status)
	}
	status, _ = s.do(t, "GET", fmt.Sprintf("/api/docs/%d", docID), readerToken, nil)
	if status != http.StatusOK {
		t.Fatalf("通过后应能访问，实际 %d", status)
	}
	status, result = s.do(t, "PUT", fmt.Sprintf("/api/access-requests/%d", requestID), ownerToken, map[string]string{"action": "deny"})
	if status != http.StatusOK || result["status"] != models.AccessDenied {
		t.Fatalf("撤销访问失败: %d %v", status, result)
	}
	status, _ = s.do(t, "GET", fmt.Sprintf("/api/docs/%d", docID), readerToken, nil)
	if status != http.StatusForbidden {
		t.Fatalf("撤销后应失去访问权限，实际 %d", status)
	}
	var notice models.Notification
	if err := s.api.DB.Where("type = ?", models.NotifyAccessResult).Order("id DESC").First(&notice).Error; err != nil {
		t.Fatalf("撤销通知不存在: %v", err)
	}
	if !strings.Contains(notice.Content, "撤销") {
		t.Fatalf("撤销通知内容不正确: %v", notice)
	}
}

func TestSearchRankingAndPrivacy(t *testing.T) {
	s := newTestAPI(t)
	ownerToken, _ := s.register(t, "searchowner")
	readerToken, _ := s.register(t, "searchreader")

	create := func(token string, body map[string]any) uint {
		status, data := s.do(t, "POST", "/api/docs", token, body)
		if status != http.StatusCreated {
			t.Fatalf("创建搜索测试内容失败: %d %v", status, data)
		}
		return uint(number(obj(data, "doc")["id"]))
	}
	articleID := create(ownerToken, map[string]any{
		"kind": "article", "title": "星尘检索指南", "abstract": "关于星尘检索的摘要。",
		"content": "正文介绍星尘检索的方法。", "tags": []string{"检索标签"}, "status": "published",
	})
	docID := create(ownerToken, map[string]any{
		"kind": "doc", "title": "普通文档", "abstract": "另一篇文档。",
		"content": "正文里也提到了星尘检索。", "tags": []string{"检索标签"}, "status": "published",
	})
	privateID := create(ownerToken, map[string]any{
		"kind": "doc", "title": "星尘检索私密文档", "content": "星尘检索私密内容。", "status": "published", "private": true,
	})

	status, data := s.do(t, "GET", "/api/search?q=%E6%98%9F%E5%B0%98%E6%A3%80%E7%B4%A2", "", nil)
	if status != http.StatusOK {
		t.Fatalf("搜索失败: %d %v", status, data)
	}
	if number(obj(data, "counts")["all"]) != 2 || number(data["total"]) != 2 {
		t.Fatalf("搜索计数不正确: %v", data)
	}
	items := list(data, "items")
	if len(items) != 2 || uint(number(items[0].(map[string]any)["id"])) != articleID {
		t.Fatalf("搜索排序不正确: %v", items)
	}
	if !strings.Contains(items[0].(map[string]any)["abstract"].(string), "星尘检索") {
		t.Fatalf("搜索摘要片段缺少关键词: %v", items[0])
	}
	for _, item := range items {
		if uint(number(item.(map[string]any)["id"])) == privateID {
			t.Fatalf("公开搜索不应返回私密文档: %v", item)
		}
	}

	status, data = s.do(t, "GET", "/api/search?q=%E6%98%9F%E5%B0%98%E6%A3%80%E7%B4%A2&kind=doc", readerToken, nil)
	if status != http.StatusOK || number(data["total"]) != 1 || uint(number(list(data, "items")[0].(map[string]any)["id"])) != docID {
		t.Fatalf("按类型搜索不正确: %d %v", status, data)
	}

	status, data = s.do(t, "GET", "/api/search?tag=%E6%A3%80%E7%B4%A2%E6%A0%87%E7%AD%BE", "", nil)
	if status != http.StatusOK || number(data["total"]) != 2 {
		t.Fatalf("标签搜索不正确: %d %v", status, data)
	}

	status, data = s.do(t, "GET", "/api/search?q=%E6%98%9F%E5%B0%98%E6%A3%80%E7%B4%A2", ownerToken, nil)
	if status != http.StatusOK || number(data["total"]) != 3 {
		t.Fatalf("作者搜索应包含自己的私密文档: %d %v", status, data)
	}

	status, data = s.do(t, "GET", "/api/search", "", nil)
	if status != http.StatusOK || len(list(data, "items")) != 0 {
		t.Fatalf("空搜索应返回空结果: %d %v", status, data)
	}
}

func TestArticleNeighbors(t *testing.T) {
	s := newTestAPI(t)
	token, _ := s.register(t, "neighborowner")
	ids := make([]uint, 0, 3)
	for _, title := range []string{"邻居文章一", "邻居文章二", "邻居文章三"} {
		status, data := s.do(t, "POST", "/api/docs", token, map[string]any{
			"kind": "article", "title": title, "content": "正文", "status": "published",
		})
		if status != http.StatusCreated {
			t.Fatalf("创建邻居文章失败: %d %v", status, data)
		}
		ids = append(ids, uint(number(obj(data, "doc")["id"])))
	}
	status, data := s.do(t, "GET", fmt.Sprintf("/api/docs/%d/neighbors", ids[1]), "", nil)
	if status != http.StatusOK {
		t.Fatalf("读取相邻文章失败: %d %v", status, data)
	}
	prev := obj(data, "prev")
	next := obj(data, "next")
	if uint(number(prev["id"])) != ids[0] || uint(number(next["id"])) != ids[2] {
		t.Fatalf("相邻文章顺序不正确: %v", data)
	}
}
