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

	"folio/server/cache"
	"folio/server/config"
	"folio/server/db"
	"folio/server/handlers"

	"github.com/gin-gonic/gin"
)

type testAPI struct {
	handler   http.Handler
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
	return &testAPI{handler: r, uploadDir: uploadDir}
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

	status, _ = s.do(t, "DELETE", "/api/admin/users/"+itoa(bobID), aliceToken, nil)
	if status != http.StatusOK {
		t.Fatalf("管理员删除用户失败: %d", status)
	}
	status, _ = s.do(t, "GET", "/api/users/bob", "", nil)
	if status != http.StatusNotFound {
		t.Fatalf("删除用户后应返回 404，实际 %d", status)
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

	status, _ := s.upload(t, readerToken, "file", "reader.png", pngData)
	if status != http.StatusForbidden {
		t.Fatalf("读者不应能上传图片，实际 %d", status)
	}

	status, first := s.upload(t, ownerToken, "file", "tiny.png", pngData)
	if status != http.StatusCreated {
		t.Fatalf("上传图片失败: %d %v", status, first)
	}
	image, _ := first["image"].(map[string]any)
	imageURL, _ := first["url"].(string)
	imageID := uint(number(image["id"]))
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
	if status != http.StatusOK || len(list["items"].([]any)) != 1 {
		t.Fatalf("图片列表异常: %d %v", status, list)
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
