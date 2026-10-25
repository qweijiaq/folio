package db

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"folio/server/config"
	"folio/server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Open 根据配置选择 MySQL、进程内 SQLite 或文件 SQLite，并执行迁移。
func Open(cfg config.Config) (*gorm.DB, error) {
	switch cfg.DBDriver {
	case "mysql":
		return openMySQL(cfg)
	case "memory":
		return openMemory()
	default:
		return openSQLite(cfg.DBPath)
	}
}

func migrate(gdb *gorm.DB) error {
	if err := gdb.AutoMigrate(
		&models.User{},
		&models.UserSession{},
		&models.Category{},
		&models.Document{},
		&models.DocumentRevision{},
		&models.Series{},
		&models.Like{},
		&models.FavoriteFolder{},
		&models.Favorite{},
		&models.Comment{},
		&models.CommentLike{},
		&models.Follow{},
		&models.Message{},
		&models.Notification{},
		&models.Image{},
		&models.AIProviderKey{},
		&models.AIFeedback{},
		&models.DocAccessRequest{},
		&models.ViewHistory{},
		&models.DocDailyStat{},
		&models.InviteCode{},
		&models.EmailCode{},
		&models.UserSetting{},
		&models.GlobalNotice{},
		&models.NoticeRead{},
		&models.SiteSetting{},
		&models.StorageSetting{},
		&models.OperationLog{},
	); err != nil {
		return err
	}
	return ensureSuperAdmin(gdb)
}

// ensureSuperAdmin 兼容旧数据：如果库里还没有超级管理员，
// 就把最早的 admin（没有 admin 时取最早的用户）提升为超级管理员。
func ensureSuperAdmin(gdb *gorm.DB) error {
	var count int64
	if err := gdb.Model(&models.User{}).Where("role = ?", models.RoleSuperAdmin).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	var u models.User
	err := gdb.Where("role = ?", models.RoleAdmin).Order("id ASC").First(&u).Error
	if err != nil {
		if err := gdb.Order("id ASC").First(&u).Error; err != nil {
			return nil // 空库，等第一位注册用户
		}
	}
	return gdb.Model(&models.User{}).Where("id = ?", u.ID).UpdateColumn("role", models.RoleSuperAdmin).Error
}

// EnsureSiteSetting 为老库补一条站点设置，之后以数据库里的值为准。
func EnsureSiteSetting(gdb *gorm.DB, cfg config.Config) error {
	var count int64
	if err := gdb.Model(&models.SiteSetting{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return gdb.Create(&models.SiteSetting{
		SiteName:      cfg.SiteName,
		SiteDesc:      cfg.SiteDesc,
		SiteOwner:     cfg.SiteOwner,
		AllowRegister: true,
		LoginCaptcha:  cfg.LoginCaptcha,
	}).Error
}

// PublishScheduled 把到点的定时内容转为已发布。
func PublishScheduled(gdb *gorm.DB) error {
	return gdb.Model(&models.Document{}).
		Where("status = ? AND publish_at IS NOT NULL AND publish_at <= ?", models.StatusScheduled, time.Now()).
		Updates(map[string]any{
			"status":       models.StatusPublished,
			"published_at": gorm.Expr("publish_at"),
			"publish_at":   nil,
		}).Error
}

// CleanupSessions 删除已过期或撤销超过七天的登录会话。
func CleanupSessions(gdb *gorm.DB) error {
	now := time.Now()
	return gdb.Where("expires_at < ? OR (revoked_at IS NOT NULL AND revoked_at < ?)", now, now.Add(-7*24*time.Hour)).
		Delete(&models.UserSession{}).Error
}

// CleanupDailyStats 删除已不存在内容的每日统计，避免看板被历史测试数据污染。
func CleanupDailyStats(gdb *gorm.DB) error {
	return gdb.Where("doc_id NOT IN (?)", gdb.Model(&models.Document{}).Select("id")).
		Delete(&models.DocDailyStat{}).Error
}

// EnsureFavoriteFolders 为所有用户补齐默认收藏夹，并把旧收藏归入默认收藏夹。
func EnsureFavoriteFolders(gdb *gorm.DB) error {
	var users []models.User
	if err := gdb.Select("id").Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		var folder models.FavoriteFolder
		err := gdb.Where("user_id = ? AND is_default = ?", user.ID, true).First(&folder).Error
		if err != nil {
			folder = models.FavoriteFolder{UserID: user.ID, Name: "默认收藏夹", IsDefault: true}
			if createErr := gdb.Create(&folder).Error; createErr != nil {
				if findErr := gdb.Where("user_id = ? AND name = ?", user.ID, "默认收藏夹").First(&folder).Error; findErr != nil {
					return createErr
				}
				gdb.Model(&folder).UpdateColumn("is_default", true)
			}
		}
		if err := gdb.Model(&models.Favorite{}).
			Where("user_id = ? AND (folder_id IS NULL OR folder_id = 0)", user.ID).
			UpdateColumn("folder_id", folder.ID).Error; err != nil {
			return err
		}
	}
	return nil
}

func openSQLite(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 失败: %w", err)
	}
	if err := migrate(gdb); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return gdb, nil
}

func openMemory() (*gorm.DB, error) {
	dsn := fmt.Sprintf("file:folio-memory-%d?mode=memory&cache=shared&_pragma=busy_timeout(5000)", os.Getpid())
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("打开内存数据库失败: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	// 共享内存库在最后一个连接关闭时会消失，固定单连接保证进程存活期间数据不丢。
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	if err := migrate(gdb); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	log.Println("使用内存数据库（重启后数据清空）")
	return gdb, nil
}

func openMySQL(cfg config.Config) (*gorm.DB, error) {
	if cfg.MySQLDSN == "" {
		return nil, fmt.Errorf("FOLIO_MYSQL_DSN 不能为空")
	}
	gdb, err := gorm.Open(mysql.Open(cfg.MySQLDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(time.Hour)
	if err := migrate(gdb); err != nil {
		return nil, fmt.Errorf("数据库迁移失败: %w", err)
	}
	return gdb, nil
}
