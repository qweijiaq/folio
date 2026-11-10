package handlers

import (
	"net/http"
	"strings"

	"folio/server/models"

	"github.com/gin-gonic/gin"
)

type projectInput struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	URL      string   `json:"url"`
	RepoURL  string   `json:"repoURL"`
	CoverURL string   `json:"coverURL"`
	Tags     []string `json:"tags"`
	Visible  *bool    `json:"visible"`
	Sort     int      `json:"sort"`
}

func cleanProjectURL(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", true
	}
	if runeLen(value) > 500 {
		return "", false
	}
	if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
		return "", false
	}
	return value, true
}

func normalizeProjectInput(in *projectInput) error {
	in.Title = trim(in.Title)
	in.Summary = trim(in.Summary)
	if runeLen(in.Title) < 1 || runeLen(in.Title) > 80 {
		return errProjectInput("项目名称需在 1-80 个字符之间")
	}
	if runeLen(in.Summary) > 300 {
		return errProjectInput("项目简介最多 300 个字符")
	}
	var ok bool
	if in.URL, ok = cleanProjectURL(in.URL); !ok {
		return errProjectInput("项目主页必须是 http:// 或 https:// 开头的链接")
	}
	if in.RepoURL, ok = cleanProjectURL(in.RepoURL); !ok {
		return errProjectInput("仓库地址必须是 http:// 或 https:// 开头的链接")
	}
	if in.CoverURL, ok = cleanProjectURL(in.CoverURL); !ok {
		return errProjectInput("封面地址必须是 http:// 或 https:// 开头的链接")
	}
	in.Tags = normalizeTags(in.Tags)
	return nil
}

type errProjectInput string

func (e errProjectInput) Error() string { return string(e) }

func (a *API) ProjectList(c *gin.Context) {
	var items []models.Project
	if err := a.DB.Where("visible = ?", true).Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "读取项目失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) AdminProjectList(c *gin.Context) {
	var items []models.Project
	if err := a.DB.Order("sort ASC, id ASC").Find(&items).Error; err != nil {
		fail(c, http.StatusInternalServerError, "读取项目失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *API) ProjectCreate(c *gin.Context) {
	var in projectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	if err := normalizeProjectInput(&in); err != nil {
		bad(c, err.Error())
		return
	}
	visible := true
	if in.Visible != nil {
		visible = *in.Visible
	}
	project := models.Project{
		Title:    in.Title,
		Summary:  in.Summary,
		URL:      in.URL,
		RepoURL:  in.RepoURL,
		CoverURL: in.CoverURL,
		Tags:     in.Tags,
		Visible:  visible,
		Sort:     in.Sort,
	}
	if err := a.DB.Create(&project).Error; err != nil {
		fail(c, http.StatusInternalServerError, "创建项目失败")
		return
	}
	a.logAction(c, "创建项目", "project", project.ID, project.Title)
	c.JSON(http.StatusCreated, gin.H{"project": project})
}

func (a *API) ProjectUpdate(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var project models.Project
	if err := a.DB.First(&project, id).Error; err != nil {
		fail(c, http.StatusNotFound, "项目不存在")
		return
	}
	var in projectInput
	if err := c.ShouldBindJSON(&in); err != nil {
		bad(c, "请求格式错误")
		return
	}
	if err := normalizeProjectInput(&in); err != nil {
		bad(c, err.Error())
		return
	}
	project.Title = in.Title
	project.Summary = in.Summary
	project.URL = in.URL
	project.RepoURL = in.RepoURL
	project.CoverURL = in.CoverURL
	project.Tags = in.Tags
	project.Sort = in.Sort
	if in.Visible != nil {
		project.Visible = *in.Visible
	}
	if err := a.DB.Save(&project).Error; err != nil {
		fail(c, http.StatusInternalServerError, "保存项目失败")
		return
	}
	a.logAction(c, "更新项目", "project", project.ID, project.Title)
	c.JSON(http.StatusOK, gin.H{"project": project})
}

func (a *API) ProjectDelete(c *gin.Context) {
	id, ok := paramID(c)
	if !ok {
		return
	}
	var project models.Project
	if err := a.DB.First(&project, id).Error; err != nil {
		fail(c, http.StatusNotFound, "项目不存在")
		return
	}
	if err := a.DB.Delete(&project).Error; err != nil {
		fail(c, http.StatusInternalServerError, "删除项目失败")
		return
	}
	a.logAction(c, "删除项目", "project", project.ID, project.Title)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
