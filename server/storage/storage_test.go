package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"folio/server/config"
)

func TestLocalStorageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewLocal(dir)
	object, err := store.Put(context.Background(), "2026/10/abcdef.png", []byte("hello"), "image/png")
	if err != nil {
		t.Fatalf("保存本地图片失败: %v", err)
	}
	if object.Provider != "local" || object.URL != "/uploads/2026/10/abcdef.png" {
		t.Fatalf("本地对象信息不正确: %#v", object)
	}
	path := filepath.Join(dir, "2026/10/abcdef.png")
	if data, err := os.ReadFile(path); err != nil || string(data) != "hello" {
		t.Fatalf("本地文件未正确写入: %v %q", err, data)
	}
	if err := store.Delete(context.Background(), object.Key); err != nil {
		t.Fatalf("删除本地图片失败: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("删除后文件仍存在: %v", err)
	}
	if err := store.Delete(context.Background(), object.Key); err != nil {
		t.Fatalf("重复删除应幂等: %v", err)
	}
}

func TestLocalStorageRejectsTraversal(t *testing.T) {
	store := NewLocal(t.TempDir())
	if _, err := store.Put(context.Background(), "../escape.png", []byte("x"), "image/png"); err == nil {
		t.Fatal("应拒绝包含 .. 的路径")
	}
	if err := store.Delete(context.Background(), "../../etc/passwd"); err == nil {
		t.Fatal("删除时也不应接受越界路径")
	}
}

func TestManagerSelection(t *testing.T) {
	dir := t.TempDir()
	localOnly := New(config.Config{UploadDir: dir, StorageProvider: "local"})
	if localOnly.Provider() != "local" || localOnly.For("qiniu").Provider() != "local" {
		t.Fatalf("未配置七牛时应回退本地: %s", localOnly.Provider())
	}

	qiniu := New(config.Config{
		UploadDir:       dir,
		StorageProvider: "qiniu",
		QiniuAccessKey:  "ak",
		QiniuSecretKey:  "sk",
		QiniuBucket:     "bucket",
		QiniuDomain:     "cdn.example.com",
		QiniuPrefix:     "folio",
	})
	if qiniu.Provider() != "qiniu" || !qiniu.QiniuConfigured() {
		t.Fatalf("配置完整时应选中七牛: %s", qiniu.Provider())
	}
	url := qiniu.For("qiniu").URL("folio/2026/10/a.png")
	if url != "https://cdn.example.com/folio/2026/10/a.png" {
		t.Fatalf("七牛公开 URL 不正确: %s", url)
	}
	if !strings.HasPrefix(qiniu.For("local").URL("folio/2026/10/a.png"), "/uploads/") {
		t.Fatal("本地存储应始终可用")
	}
}

func TestQiniuUploadWithFakeEndpoint(t *testing.T) {
	var uploads int
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Reqid", "fake-reqid")
		uploads++
		_, _ = w.Write([]byte(`{"key":"folio/2026/10/a.png","hash":"fake"}`))
	}))
	defer fake.Close()

	q := NewQiniu(config.Config{
		QiniuAccessKey: "ak",
		QiniuSecretKey: "sk",
		QiniuBucket:    "bucket",
		QiniuDomain:    "cdn.example.com",
		QiniuRegion:    "z0",
		QiniuPrefix:    "folio",
	})
	q.upHost = fake.URL

	object, err := q.Put(context.Background(), "2026/10/a.png", []byte("hello"), "image/png")
	if err != nil {
		t.Fatalf("七牛上传失败: %v", err)
	}
	if uploads != 1 || object.Key != "folio/2026/10/a.png" || object.URL != "https://cdn.example.com/folio/2026/10/a.png" {
		t.Fatalf("七牛上传结果不正确: uploads=%d object=%#v", uploads, object)
	}
}

func TestManagerRuntimeConfigure(t *testing.T) {
	manager := New(config.Config{UploadDir: t.TempDir(), StorageProvider: "local"})
	if manager.Provider() != "local" {
		t.Fatalf("默认应为本地存储: %s", manager.Provider())
	}
	manager.Configure(config.Config{
		UploadDir:       t.TempDir(),
		StorageProvider: "qiniu",
		QiniuAccessKey:  "ak",
		QiniuSecretKey:  "sk",
		QiniuBucket:     "bucket",
		QiniuDomain:     "https://cdn.example.com",
	})
	if manager.Provider() != "qiniu" || !manager.QiniuConfigured() {
		t.Fatalf("运行时切换七牛失败: %s", manager.Provider())
	}
	manager.Configure(config.Config{StorageProvider: "local"})
	if manager.Provider() != "local" {
		t.Fatalf("运行时切回本地失败: %s", manager.Provider())
	}
}
