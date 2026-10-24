package handlers

import (
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (a *API) ensureDefaultFolder(userID uint) (*models.FavoriteFolder, error) {
	var folder models.FavoriteFolder
	err := a.DB.Where("user_id = ? AND is_default = ?", userID, true).First(&folder).Error
	if err == nil {
		return &folder, nil
	}
	folder = models.FavoriteFolder{UserID: userID, Name: "默认收藏夹", IsDefault: true}
	if createErr := a.DB.Create(&folder).Error; createErr != nil {
		if findErr := a.DB.Where("user_id = ? AND name = ?", userID, "默认收藏夹").First(&folder).Error; findErr != nil {
			return nil, createErr
		}
		a.DB.Model(&folder).UpdateColumn("is_default", true)
	}
	a.DB.Model(&models.Favorite{}).
		Where("user_id = ? AND (folder_id IS NULL OR folder_id = 0)", userID).
		UpdateColumn("folder_id", folder.ID)
	return &folder, nil
}

func (a *API) FavoriteFolderList(c *gin.Context) {
	me := currentUser(c)
	if _, err := a.ensureDefaultFolder(me.ID); err != nil {
		fail(c, http.StatusInternalServerError, "读取收藏夹失败")
		return
	}
	var folders []models.FavoriteFolder
	a.DB.Where("user_id = ?", me.ID).Order("is_default DESC, sort ASC, id ASC").Find(&folders)
	items := make([]gin.H, 0, len(folders))
	for _, folder := range folders {
		var count int64
		a.DB.Model(&models.Favorite{}).Where("user_id = ? AND folder_id = ?", me.ID, folder.ID).Count(&count)
		items = append(items, gin.H{
			"id":        folder.ID,
			"name":      folder.Name,
			"isDefault": folder.IsDefault,
			"sort":      folder.Sort,
			"count":     count,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) FavoriteFolderCreate(c *gin.Context) {
	me := currentUser(c)
	var in struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Name = trim(in.Name)
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 20 {
		bad(c, "收藏夹名称需在 1-20 个字符之间")
		return
	}
	if _, err := a.ensureDefaultFolder(me.ID); err != nil {
		fail(c, http.StatusInternalServerError, "创建收藏夹失败")
		return
	}
	var exists int64
	a.DB.Model(&models.FavoriteFolder{}).Where("user_id = ? AND lower(name) = lower(?)", me.ID, in.Name).Count(&exists)
	if exists > 0 {
		bad(c, "收藏夹已存在")
		return
	}
	folder := models.FavoriteFolder{UserID: me.ID, Name: in.Name}
	if err := a.DB.Create(&folder).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建收藏夹失败")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"folder": folder})
}

func (a *API) FavoriteFolderUpdate(c *gin.Context) {
	me := currentUser(c)
	id, ok := paramID(c)
	if !ok {
		return
	}
	var folder models.FavoriteFolder
	if err := a.DB.Where("id = ? AND user_id = ?", id, me.ID).First(&folder).Error; err != nil {
		fail(c, http.StatusNotFound, "收藏夹不存在")
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Name = trim(in.Name)
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 20 {
		bad(c, "收藏夹名称需在 1-20 个字符之间")
		return
	}
	var exists int64
	a.DB.Model(&models.FavoriteFolder{}).
		Where("user_id = ? AND lower(name) = lower(?) AND id <> ?", me.ID, in.Name, folder.ID).
		Count(&exists)
	if exists > 0 {
		bad(c, "收藏夹已存在")
		return
	}
	a.DB.Model(&folder).UpdateColumn("name", in.Name)
	folder.Name = in.Name
	c.JSON(http.StatusOK, gin.H{"folder": folder})
}

func (a *API) FavoriteFolderDelete(c *gin.Context) {
	me := currentUser(c)
	id, ok := paramID(c)
	if !ok {
		return
	}
	var folder models.FavoriteFolder
	if err := a.DB.Where("id = ? AND user_id = ?", id, me.ID).First(&folder).Error; err != nil {
		fail(c, http.StatusNotFound, "收藏夹不存在")
		return
	}
	if folder.IsDefault {
		bad(c, "默认收藏夹不能删除")
		return
	}
	defaultFolder, err := a.ensureDefaultFolder(me.ID)
	if err != nil {
		fail(c, http.StatusInternalServerError, "删除收藏夹失败")
		return
	}
	var moved int64
	err = a.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Favorite{}).
			Where("user_id = ? AND folder_id = ?", me.ID, folder.ID).
			UpdateColumn("folder_id", defaultFolder.ID)
		moved = res.RowsAffected
		return tx.Delete(&folder).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "删除收藏夹失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "moved": moved, "defaultFolderID": defaultFolder.ID})
}
