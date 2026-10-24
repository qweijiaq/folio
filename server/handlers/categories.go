package handlers

import (
	"net/http"

	"folio/server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type categoryInput struct {
	Name string `json:"name"`
	Sort int    `json:"sort"`
}

func (a *API) CategoryList(c *gin.Context) {
	var categories []models.Category
	a.DB.Order("sort ASC, id ASC").Find(&categories)
	items := make([]gin.H, 0, len(categories))
	for _, category := range categories {
		var count int64
		a.DB.Model(&models.Document{}).
			Where("category_id = ? AND status = ?", category.ID, models.StatusPublished).
			Count(&count)
		items = append(items, gin.H{
			"id":    category.ID,
			"name":  category.Name,
			"sort":  category.Sort,
			"count": count,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) CategoryCreate(c *gin.Context) {
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Name = trim(in.Name)
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 16 {
		bad(c, "分类名称需在 1-16 个字符之间")
		return
	}
	var exists int64
	a.DB.Model(&models.Category{}).Where("lower(name) = lower(?)", in.Name).Count(&exists)
	if exists > 0 {
		bad(c, "分类已存在")
		return
	}
	category := models.Category{Name: in.Name, Sort: in.Sort}
	if err := a.DB.Create(&category).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建分类失败")
		return
	}
	c.JSON(http.StatusCreated, gin.H{"category": category})
}

func (a *API) CategoryUpdate(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var category models.Category
	if err := a.DB.First(&category, id).Error; err != nil {
		fail(c, http.StatusNotFound, "分类不存在")
		return
	}
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	in.Name = trim(in.Name)
	if runeLen(in.Name) < 1 || runeLen(in.Name) > 16 {
		bad(c, "分类名称需在 1-16 个字符之间")
		return
	}
	var exists int64
	a.DB.Model(&models.Category{}).Where("lower(name) = lower(?) AND id <> ?", in.Name, id).Count(&exists)
	if exists > 0 {
		bad(c, "分类已存在")
		return
	}
	if err := a.DB.Model(&category).Updates(map[string]any{"name": in.Name, "sort": in.Sort}).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存分类失败")
		return
	}
	category.Name = in.Name
	category.Sort = in.Sort
	c.JSON(http.StatusOK, gin.H{"category": category})
}

func (a *API) CategoryDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var category models.Category
	if err := a.DB.First(&category, id).Error; err != nil {
		fail(c, http.StatusNotFound, "分类不存在")
		return
	}
	var moved int64
	err := a.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.Document{}).Where("category_id = ?", id).UpdateColumn("category_id", nil)
		moved = res.RowsAffected
		return tx.Delete(&category).Error
	})
	if err != nil {
		fail(c, http.StatusInternalServerError, "删除分类失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "moved": moved})
}
