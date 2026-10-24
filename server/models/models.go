package models

import "time"

const (
	KindArticle = "article"
	KindDoc     = "doc"

	StatusDraft     = "draft"
	StatusPublished = "published"

	RoleSuperAdmin = "super_admin"
	RoleAdmin      = "admin"
	RoleUser       = "user"

	NotifyLike    = "like"
	NotifyComment = "comment"
	NotifyFollow  = "follow"

	MessageText  = "text"
	MessageImage = "image"
)

type Base struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type User struct {
	Base
	Username    string     `gorm:"size:32;uniqueIndex;not null" json:"username"`
	Nickname    string     `gorm:"size:32;not null" json:"nickname"`
	Email       string     `gorm:"size:128" json:"email"`
	Password    string     `gorm:"size:128;not null" json:"-"`
	Bio         string     `gorm:"size:255" json:"bio"`
	Role        string     `gorm:"size:16;not null;default:user" json:"role"`
	QQOpenID    string     `gorm:"size:64;index" json:"-"`
	LastLoginAt *time.Time `json:"lastLoginAt"`
}

// Document 同时承载博客文章（article）与知识库文档（doc）。
// 文档通过 ParentID 组成树，文章始终是平的。
type Document struct {
	Base
	Kind          string     `gorm:"size:16;index;not null" json:"kind"`
	Title         string     `gorm:"size:200;not null" json:"title"`
	Abstract      string     `gorm:"size:500" json:"abstract"`
	Content       string     `gorm:"type:longtext" json:"content,omitempty"`
	AuthorID      uint       `gorm:"index" json:"authorID"`
	Author        User       `gorm:"foreignKey:AuthorID" json:"author"`
	ParentID      *uint      `gorm:"index" json:"parentID"`
	CategoryID    *uint      `gorm:"index" json:"categoryID"`
	Category      *Category  `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
	Tags          []string   `gorm:"type:text;serializer:json" json:"tags"`
	Status        string     `gorm:"size:16;index;not null;default:draft" json:"status"`
	Pinned        bool       `json:"pinned"`
	OpenComment   bool       `gorm:"not null;default:true" json:"openComment"`
	Sort          int        `json:"sort"`
	ViewCount     int        `json:"viewCount"`
	LikeCount     int        `json:"likeCount"`
	CommentCount  int        `json:"commentCount"`
	FavoriteCount int        `json:"favoriteCount"`
	PublishedAt   *time.Time `json:"publishedAt"`
}

// Category 只用于文章；文档仍通过 ParentID 组成目录树。
type Category struct {
	Base
	Name string `gorm:"size:32;uniqueIndex;not null" json:"name"`
	Sort int    `json:"sort"`
}

type Like struct {
	Base
	UserID uint `gorm:"uniqueIndex:idx_like_user_doc" json:"userID"`
	DocID  uint `gorm:"uniqueIndex:idx_like_user_doc;index" json:"docID"`
}

type FavoriteFolder struct {
	Base
	UserID    uint   `gorm:"uniqueIndex:idx_folder_user_name" json:"userID"`
	Name      string `gorm:"size:32;uniqueIndex:idx_folder_user_name" json:"name"`
	IsDefault bool   `json:"isDefault"`
	Sort      int    `json:"sort"`
}

type Favorite struct {
	Base
	UserID   uint `gorm:"uniqueIndex:idx_fav_user_doc" json:"userID"`
	DocID    uint `gorm:"uniqueIndex:idx_fav_user_doc;index" json:"docID"`
	FolderID uint `gorm:"index" json:"folderID"`
}

// Comment 只有两级：ParentID 为空是根评论，否则指向根评论。
type Comment struct {
	Base
	DocID       uint   `gorm:"index;not null" json:"docID"`
	UserID      uint   `gorm:"index" json:"userID"`
	User        User   `gorm:"foreignKey:UserID" json:"user"`
	Content     string `gorm:"size:1000" json:"content"`
	ParentID    *uint  `gorm:"index" json:"parentID"`
	ReplyToName string `gorm:"size:32" json:"replyToName"`
	LikeCount   int    `json:"likeCount"`
}

type CommentLike struct {
	Base
	UserID    uint `gorm:"uniqueIndex:idx_clike_user_comment" json:"userID"`
	CommentID uint `gorm:"uniqueIndex:idx_clike_user_comment;index" json:"commentID"`
}

type Follow struct {
	Base
	FollowerID uint `gorm:"uniqueIndex:idx_follow" json:"followerID"`
	FolloweeID uint `gorm:"uniqueIndex:idx_follow;index" json:"followeeID"`
}

type Message struct {
	Base
	SenderID   uint       `gorm:"index" json:"senderID"`
	ReceiverID uint       `gorm:"index" json:"receiverID"`
	Type       string     `gorm:"size:16;default:text" json:"type"`
	Content    string     `gorm:"size:2000" json:"content"`
	ImageURL   string     `gorm:"size:512" json:"imageURL"`
	ReadAt     *time.Time `json:"readAt"`
}

type Notification struct {
	Base
	UserID   uint   `gorm:"index" json:"userID"` // 接收者
	ActorID  uint   `gorm:"index" json:"actorID"`
	Actor    User   `gorm:"foreignKey:ActorID" json:"actor"`
	Type     string `gorm:"size:16" json:"type"`
	DocID    *uint  `json:"docID"`
	DocKind  string `gorm:"size:16" json:"docKind"`
	DocTitle string `gorm:"size:200" json:"docTitle"`
	Content  string `gorm:"size:300" json:"content"`
	Read     bool   `gorm:"index" json:"read"`
}

// Image 记录上传到本站的图片。文件本身存放在 UploadDir 下，
// 用内容哈希命名以达到天然的秒传与去重。
type Image struct {
	Base
	UserID   uint   `gorm:"index" json:"userID"`
	User     User   `gorm:"foreignKey:UserID" json:"user"`
	Filename string `gorm:"size:255" json:"filename"`
	Path     string `gorm:"size:300" json:"-"`
	MIME     string `gorm:"size:64" json:"mime"`
	Size     int64  `json:"size"`
	Hash     string `gorm:"size:64;index" json:"-"`
}

// EmailCode 保存邮箱验证码。只存 HMAC 摘要，验证码用后即焚。
type EmailCode struct {
	Base
	Email      string     `gorm:"size:128;index;not null" json:"email"`
	Purpose    string     `gorm:"size:16;index;not null" json:"purpose"`
	CodeHash   string     `gorm:"size:64;not null" json:"-"`
	ExpiresAt  time.Time  `gorm:"index" json:"expiresAt"`
	Attempts   int        `json:"attempts"`
	ConsumedAt *time.Time `json:"consumedAt"`
	IP         string     `gorm:"size:64" json:"ip"`
}

// UserSetting 保存隐私、消息通知与主页样式偏好。
type UserSetting struct {
	Base
	UserID             uint   `gorm:"uniqueIndex;not null" json:"userID"`
	OpenFollow         bool   `json:"openFollow"`
	OpenFans           bool   `json:"openFans"`
	OpenPrivateChat    bool   `json:"openPrivateChat"`
	OpenCommentMessage bool   `json:"openCommentMessage"`
	OpenDiggMessage    bool   `json:"openDiggMessage"`
	HomeStyle          string `gorm:"size:16" json:"homeStyle"`
}

// SiteSetting 是单行站点设置，后台修改后立即生效。
type SiteSetting struct {
	Base
	SiteName      string `gorm:"size:64" json:"siteName"`
	SiteDesc      string `gorm:"size:255" json:"siteDesc"`
	SiteOwner     string `gorm:"size:64" json:"siteOwner"`
	AllowRegister bool   `json:"allowRegister"`
	LoginCaptcha  bool   `json:"loginCaptcha"`
}

// GlobalNotice 是管理员发给全站用户的公告，展示在通知页顶部。
type GlobalNotice struct {
	Base
	Title         string `gorm:"size:120;not null" json:"title"`
	Content       string `gorm:"size:1000" json:"content"`
	Link          string `gorm:"size:255" json:"link"`
	CreatedBy     uint   `json:"createdBy"`
	CreatedByName string `gorm:"size:32" json:"createdByName"`
}

// NoticeRead 记录某个用户读过某条全站公告。
type NoticeRead struct {
	Base
	NoticeID uint `gorm:"uniqueIndex:idx_notice_read" json:"noticeID"`
	UserID   uint `gorm:"uniqueIndex:idx_notice_read" json:"userID"`
}

// OperationLog 记录登录、内容与后台管理动作，供超级管理员审计。
type OperationLog struct {
	Base
	UserID     uint   `gorm:"index" json:"userID"`
	Username   string `gorm:"size:32;index" json:"username"`
	Action     string `gorm:"size:32;index;not null" json:"action"`
	TargetType string `gorm:"size:32" json:"targetType"`
	TargetID   uint   `json:"targetID"`
	Detail     string `gorm:"size:500" json:"detail"`
	IP         string `gorm:"size:64" json:"ip"`
	UserAgent  string `gorm:"size:255" json:"userAgent"`
}

// ViewHistory 记录登录用户最近打开过的内容，用于「浏览历史」。
type ViewHistory struct {
	Base
	UserID uint `gorm:"uniqueIndex:idx_history_user_doc" json:"userID"`
	DocID  uint `gorm:"uniqueIndex:idx_history_user_doc;index" json:"docID"`
}
