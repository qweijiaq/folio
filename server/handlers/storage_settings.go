package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"folio/server/config"
	"folio/server/models"
	"folio/server/secure"
	foliostorage "folio/server/storage"

	"github.com/gin-gonic/gin"
	qiniustorage "github.com/qiniu/go-sdk/v7/storage"
)

type storageConfigInput struct {
	Provider       string `json:"provider"`
	QiniuAccessKey string `json:"qiniuAccessKey"`
	QiniuSecretKey string `json:"qiniuSecretKey"`
	QiniuBucket    string `json:"qiniuBucket"`
	QiniuDomain    string `json:"qiniuDomain"`
	QiniuRegion    string `json:"qiniuRegion"`
	QiniuPrefix    string `json:"qiniuPrefix"`
}

func (a *API) storedStorageSetting() (models.StorageSetting, bool) {
	var row models.StorageSetting
	if err := a.DB.First(&row, 1).Error; err != nil || row.ID == 0 {
		return row, false
	}
	return row, true
}

// loadStorageSetting 在启动时把后台保存的存储配置应用到存储管理器。
func (a *API) loadStorageSetting() {
	row, ok := a.storedStorageSetting()
	if !ok {
		return
	}
	cfg := a.Cfg
	cfg.StorageProvider = row.Provider
	if cfg.StorageProvider == "" {
		cfg.StorageProvider = "local"
	}
	accessKey, secretKey, err := a.decryptStorageSetting(row)
	if err != nil {
		log.Printf("读取后台图片存储密钥失败，已保留环境变量配置: %v", err)
		return
	}
	cfg.QiniuAccessKey = accessKey
	cfg.QiniuSecretKey = secretKey
	cfg.QiniuBucket = row.QiniuBucket
	cfg.QiniuDomain = row.QiniuDomain
	cfg.QiniuRegion = row.QiniuRegion
	cfg.QiniuPrefix = row.QiniuPrefix
	a.Storage.Configure(cfg)
	if cfg.StorageProvider == "qiniu" && !cfg.QiniuEnabled() {
		log.Printf("后台图片存储配置为七牛但凭据不完整，已回退本地；请到管理后台补全")
	}
}

func (a *API) decryptStorageSetting(row models.StorageSetting) (string, string, error) {
	accessKey, err := secure.Decrypt(a.Cfg.Secret, row.QiniuAccessKey)
	if err != nil {
		return "", "", err
	}
	secretKey, err := secure.Decrypt(a.Cfg.Secret, row.QiniuSecretKey)
	if err != nil {
		return "", "", err
	}
	return accessKey, secretKey, nil
}

// effectiveStorageConfig 返回当前实际使用的配置：数据库设置优先，其次是环境变量。
func (a *API) effectiveStorageConfig() (config.Config, string) {
	cfg := a.Cfg
	row, ok := a.storedStorageSetting()
	if !ok {
		source := "none"
		if cfg.QiniuEnabled() {
			source = "env"
		}
		return cfg, source
	}
	accessKey, secretKey, err := a.decryptStorageSetting(row)
	if err != nil {
		log.Printf("读取图片存储密钥失败，请重新在后台填写: %v", err)
	}
	cfg.StorageProvider = row.Provider
	if cfg.StorageProvider == "" {
		cfg.StorageProvider = "local"
	}
	cfg.QiniuAccessKey = accessKey
	cfg.QiniuSecretKey = secretKey
	cfg.QiniuBucket = row.QiniuBucket
	cfg.QiniuDomain = row.QiniuDomain
	cfg.QiniuRegion = row.QiniuRegion
	cfg.QiniuPrefix = row.QiniuPrefix
	return cfg, "database"
}

func (a *API) storageStatus() gin.H {
	cfg, source := a.effectiveStorageConfig()
	provider := cfg.StorageProvider
	if provider != "qiniu" {
		provider = "local"
	}
	return gin.H{
		"provider":       provider,
		"active":         a.Storage.Provider(),
		"source":         source,
		"configured":     cfg.QiniuEnabled(),
		"hasAccessKey":   strings.TrimSpace(cfg.QiniuAccessKey) != "",
		"hasSecretKey":   strings.TrimSpace(cfg.QiniuSecretKey) != "",
		"qiniuBucket":    cfg.QiniuBucket,
		"qiniuDomain":    cfg.QiniuDomain,
		"qiniuRegion":    cfg.QiniuRegion,
		"qiniuPrefix":    cfg.QiniuPrefix,
		"maxUploadMB":    a.Cfg.MaxUploadMB,
		"localUploadDir": a.Cfg.UploadDir,
	}
}

func normalizeQiniuDomain(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("请填写七牛绑定域名")
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" {
		return "", errors.New("七牛域名格式不正确")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("七牛域名只需要域名，不要带路径")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("七牛域名不需要查询参数")
	}
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/"), nil
}

func (a *API) resolveStorageInput(in storageConfigInput) (config.Config, error) {
	cfg := a.Cfg
	existingAccess, existingSecret := cfg.QiniuAccessKey, cfg.QiniuSecretKey
	var decryptErr error
	if row, ok := a.storedStorageSetting(); ok {
		cfg.StorageProvider = row.Provider
		cfg.QiniuBucket = row.QiniuBucket
		cfg.QiniuDomain = row.QiniuDomain
		cfg.QiniuRegion = row.QiniuRegion
		cfg.QiniuPrefix = row.QiniuPrefix
		existingAccess, existingSecret = "", ""
		accessKey, secretKey, err := a.decryptStorageSetting(row)
		if err != nil {
			decryptErr = err
		} else {
			existingAccess, existingSecret = accessKey, secretKey
		}
	}

	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if provider == "" {
		provider = "local"
	}
	if provider != "local" && provider != "qiniu" {
		return cfg, errors.New("图片存储只能是 local 或 qiniu")
	}
	if decryptErr != nil && provider == "qiniu" {
		return cfg, errors.New("已保存的七牛密钥无法解密，请重新填写 AccessKey 和 SecretKey")
	}
	accessKey := strings.TrimSpace(in.QiniuAccessKey)
	if accessKey == "" {
		accessKey = existingAccess
	}
	secretKey := strings.TrimSpace(in.QiniuSecretKey)
	if secretKey == "" {
		secretKey = existingSecret
	}
	bucket := strings.TrimSpace(in.QiniuBucket)
	domain, err := normalizeQiniuDomain(in.QiniuDomain)
	if err != nil && provider == "qiniu" {
		return cfg, err
	}
	if err != nil {
		domain = strings.TrimSpace(in.QiniuDomain)
	}
	region := strings.ToLower(strings.TrimSpace(in.QiniuRegion))
	prefix := strings.Trim(strings.TrimSpace(in.QiniuPrefix), "/")
	if prefix == "" {
		prefix = "folio"
	}
	if region != "" {
		if _, ok := qiniustorage.GetRegionByID(qiniustorage.RegionID(region)); !ok {
			return cfg, errors.New("未知的七牛区域，可选 z0、cn-east-2、z1、z2、na0、as0")
		}
	}
	if provider == "qiniu" {
		if accessKey == "" || secretKey == "" {
			return cfg, errors.New("七牛 AccessKey 和 SecretKey 不能为空；已保存的密钥可以留空保持不变")
		}
		if bucket == "" {
			return cfg, errors.New("请填写七牛空间名")
		}
	}

	cfg.StorageProvider = provider
	cfg.QiniuAccessKey = accessKey
	cfg.QiniuSecretKey = secretKey
	cfg.QiniuBucket = bucket
	cfg.QiniuDomain = domain
	cfg.QiniuRegion = region
	cfg.QiniuPrefix = prefix
	return cfg, nil
}

func (a *API) AdminStorage(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"storage": a.storageStatus()})
}

func (a *API) AdminStorageUpdate(c *gin.Context) {
	var in storageConfigInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	cfg, err := a.resolveStorageInput(in)
	if err != nil {
		bad(c, err.Error())
		return
	}
	sealedAccess, err := secure.Encrypt(a.Cfg.Secret, cfg.QiniuAccessKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, "加密七牛密钥失败")
		return
	}
	sealedSecret, err := secure.Encrypt(a.Cfg.Secret, cfg.QiniuSecretKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, "加密七牛密钥失败")
		return
	}

	var row models.StorageSetting
	if err := a.DB.First(&row, 1).Error; err != nil {
		row = models.StorageSetting{Base: models.Base{ID: 1}}
	}
	row.Provider = cfg.StorageProvider
	row.QiniuAccessKey = sealedAccess
	row.QiniuSecretKey = sealedSecret
	row.QiniuBucket = cfg.QiniuBucket
	row.QiniuDomain = cfg.QiniuDomain
	row.QiniuRegion = cfg.QiniuRegion
	row.QiniuPrefix = cfg.QiniuPrefix
	if err := a.DB.Save(&row).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存图片存储设置失败")
		return
	}
	a.Storage.Configure(cfg)
	a.logAction(c, "更新存储设置", "storage", row.ID, "图片存储切换为 "+cfg.StorageProvider)
	c.JSON(http.StatusOK, gin.H{"storage": a.storageStatus()})
}

func (a *API) AdminStorageTest(c *gin.Context) {
	var in storageConfigInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	cfg, err := a.resolveStorageInput(in)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if cfg.StorageProvider != "qiniu" {
		bad(c, "本地存储不需要连接测试")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	defer cancel()
	qiniu := foliostorage.NewQiniu(cfg)
	if err := qiniu.Probe(ctx); err != nil {
		log.Printf("七牛连接测试失败: %v", err)
		fail(c, http.StatusBadRequest, fmt.Sprintf("连接失败：%v", err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "连接成功，测试文件已删除"})
}
