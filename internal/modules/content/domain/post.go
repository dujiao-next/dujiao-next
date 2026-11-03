package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Post 文章/公告表
type Post struct {
	ID            uint         `gorm:"primarykey" json:"id"`                    // 主键
	Slug          string       `gorm:"uniqueIndex;not null" json:"slug"`        // 唯一标识
	Type          string       `gorm:"not null;index" json:"type"`              // 类型（blog/notice）
	TitleJSON     jsonmap.JSON `gorm:"type:json;not null" json:"title"`         // 多语言标题
	SummaryJSON   jsonmap.JSON `gorm:"type:json" json:"summary"`                // 多语言摘要
	ContentJSON   jsonmap.JSON `gorm:"type:json" json:"content"`                // 多语言内容（富文本）
	Layout        string       `gorm:"not null;default:article" json:"layout"`  // 页面形态：article / blocks
	ContentBlocks jsonmap.JSON `gorm:"type:json" json:"content_blocks"`         // 区块化页面内容
	Thumbnail     string       `json:"thumbnail"`                               // 缩略图
	CategoryID    *uint        `gorm:"index" json:"category_id"`                // 文章分类ID
	IsPublished   bool         `gorm:"default:false;index" json:"is_published"` // 是否发布
	PublishedAt   *time.Time   `gorm:"index" json:"published_at"`               // 发布时间
	CreatedAt     time.Time    `gorm:"index" json:"created_at"`                 // 创建时间
	DeletedAt     *time.Time   `gorm:"index" json:"-"`                          // 软删除时间
}

// TableName 指定表名
func (Post) TableName() string {
	return "posts"
}

// PostProduct 文章与商品的多对多关联
type PostProduct struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	PostID    uint      `gorm:"not null;uniqueIndex:idx_post_product,priority:1" json:"post_id"`
	ProductID uint      `gorm:"not null;uniqueIndex:idx_post_product,priority:2;index" json:"product_id"`
	Sort      int       `gorm:"not null;default:0" json:"sort"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 指定表名
func (PostProduct) TableName() string {
	return "post_products"
}
