// Package storage 抽象图片对象的存放位置：本地磁盘或七牛云对象存储。
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"folio/server/config"

	"github.com/qiniu/go-sdk/v7/auth"
	"github.com/qiniu/go-sdk/v7/storage"
)

type Object struct {
	Provider string
	Key      string
	URL      string
}

type Storage interface {
	Provider() string
	Put(ctx context.Context, key string, data []byte, mime string) (Object, error)
	Delete(ctx context.Context, key string) error
	URL(key string) string
}

// Manager 始终保留本地存储，用于读取/删除旧图片；active 决定新图片写到哪里。
type Manager struct {
	active string
	local  *Local
	qiniu  *Qiniu
}

func New(cfg config.Config) *Manager {
	manager := &Manager{active: "local", local: NewLocal(cfg.UploadDir)}
	if cfg.QiniuEnabled() {
		manager.qiniu = NewQiniu(cfg)
	}
	if cfg.StorageProvider == "qiniu" && manager.qiniu != nil {
		manager.active = "qiniu"
	}
	return manager
}

func (m *Manager) Provider() string { return m.active }

func (m *Manager) QiniuConfigured() bool { return m.qiniu != nil }

func (m *Manager) For(provider string) Storage {
	if provider == "qiniu" && m.qiniu != nil {
		return m.qiniu
	}
	return m.local
}

func (m *Manager) Put(ctx context.Context, key string, data []byte, mime string) (Object, error) {
	return m.For(m.active).Put(ctx, key, data, mime)
}

// Local 把图片写到 FOLIO_UPLOAD_DIR，通过 /uploads 静态访问。
type Local struct {
	dir string
}

func NewLocal(dir string) *Local { return &Local{dir: dir} }

func (l *Local) Provider() string { return "local" }

func (l *Local) URL(key string) string {
	return "/uploads/" + strings.TrimLeft(filepath.ToSlash(key), "/")
}

func (l *Local) Put(_ context.Context, key string, data []byte, _ string) (Object, error) {
	rel, err := safeRelativeKey(key)
	if err != nil {
		return Object{}, err
	}
	full := filepath.Join(l.dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return Object{}, fmt.Errorf("创建图片目录失败: %w", err)
	}
	if _, err := os.Stat(full); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(full, data, 0o644); err != nil {
			return Object{}, fmt.Errorf("保存图片失败: %w", err)
		}
	} else if err != nil {
		return Object{}, fmt.Errorf("检查图片文件失败: %w", err)
	}
	return Object{Provider: "local", Key: rel, URL: l.URL(rel)}, nil
}

func (l *Local) Delete(_ context.Context, key string) error {
	rel, err := safeRelativeKey(key)
	if err != nil {
		return err
	}
	err = os.Remove(filepath.Join(l.dir, filepath.FromSlash(rel)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Qiniu 把图片上传到七牛云，返回自定义域名或私有空间签名 URL。
type Qiniu struct {
	accessKey string
	secretKey string
	bucket    string
	domain    string
	regionID  string
	prefix    string

	mu    sync.Mutex
	cfg   *storage.Config
	mac   *auth.Credentials
	ready bool
	err   error

	// 仅测试时覆盖上传入口，生产环境使用区域解析出的入口。
	upHost string
}

func NewQiniu(cfg config.Config) *Qiniu {
	domain := strings.TrimRight(strings.TrimSpace(cfg.QiniuDomain), "/")
	if domain != "" && !strings.HasPrefix(domain, "http://") && !strings.HasPrefix(domain, "https://") {
		domain = "https://" + domain
	}
	accessKey := strings.TrimSpace(cfg.QiniuAccessKey)
	secretKey := strings.TrimSpace(cfg.QiniuSecretKey)
	return &Qiniu{
		accessKey: accessKey,
		secretKey: secretKey,
		bucket:    strings.TrimSpace(cfg.QiniuBucket),
		domain:    domain,
		regionID:  strings.TrimSpace(cfg.QiniuRegion),
		prefix:    strings.Trim(strings.TrimSpace(cfg.QiniuPrefix), "/"),
		mac:       auth.New(accessKey, secretKey),
	}
}

func (q *Qiniu) Provider() string { return "qiniu" }

func (q *Qiniu) objectKey(key string) string {
	key = strings.TrimLeft(filepath.ToSlash(key), "/")
	if q.prefix == "" {
		return key
	}
	return q.prefix + "/" + key
}

func (q *Qiniu) URL(key string) string {
	key = strings.TrimLeft(filepath.ToSlash(key), "/")
	if key == "" {
		return ""
	}
	return strings.TrimRight(q.domain, "/") + "/" + key
}

func (q *Qiniu) ensure() error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.ready {
		return nil
	}
	if q.err != nil {
		return q.err
	}
	var region storage.Region
	if q.regionID != "" {
		resolved, ok := storage.GetRegionByID(storage.RegionID(q.regionID))
		if !ok {
			q.err = fmt.Errorf("未知的七牛区域 %q（可选 z0、cn-east-2、z1、z2、na0、as0）", q.regionID)
			return q.err
		}
		region = resolved
	} else {
		resolved, err := storage.GetRegion(q.accessKey, q.bucket)
		if err != nil {
			q.err = fmt.Errorf("自动获取七牛区域失败，请配置 FOLIO_QINIU_REGION: %w", err)
			return q.err
		}
		region = *resolved
	}
	q.cfg = &storage.Config{Region: &region, UseHTTPS: true, UpHost: q.upHost}
	q.ready = true
	return nil
}

func (q *Qiniu) Put(ctx context.Context, key string, data []byte, mime string) (Object, error) {
	if err := q.ensure(); err != nil {
		return Object{}, err
	}
	objectKey := q.objectKey(key)
	policy := storage.PutPolicy{
		Scope:   q.bucket + ":" + objectKey,
		Expires: uint64(time.Now().Add(time.Hour).Unix()),
	}
	upToken := policy.UploadToken(q.mac)
	uploader := storage.NewFormUploader(q.cfg)
	ret := storage.PutRet{}
	extra := &storage.PutExtra{MimeType: mime}
	if err := uploader.Put(ctx, &ret, upToken, objectKey, bytes.NewReader(data), int64(len(data)), extra); err != nil {
		return Object{}, fmt.Errorf("上传到七牛失败: %w", err)
	}
	return Object{Provider: "qiniu", Key: objectKey, URL: q.URL(objectKey)}, nil
}

func (q *Qiniu) Delete(ctx context.Context, key string) error {
	if err := q.ensure(); err != nil {
		return err
	}
	manager := storage.NewBucketManager(q.mac, q.cfg)
	err := manager.Delete(q.bucket, strings.TrimLeft(filepath.ToSlash(key), "/"))
	if err == nil {
		return nil
	}
	var qerr *storage.ErrorInfo
	if errors.As(err, &qerr) && qerr.Code == 612 {
		return nil
	}
	return err
}

func safeRelativeKey(key string) (string, error) {
	rel := filepath.ToSlash(strings.TrimLeft(strings.TrimSpace(key), "/"))
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(rel)))
	if clean == "." || clean == "" || strings.HasPrefix(clean, "../") || clean == ".." {
		return "", errors.New("非法的图片路径")
	}
	return clean, nil
}
