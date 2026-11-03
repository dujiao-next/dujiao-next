package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Banner 首页轮播图
type Banner struct {
	ID           uint         `gorm:"primarykey" json:"id"`                                      // 主键
	Name         string       `gorm:"type:varchar(120);not null;index" json:"name"`              // 后台名称
	Position     string       `gorm:"type:varchar(60);not null;index" json:"position"`           // 投放位置
	TitleJSON    jsonmap.JSON `gorm:"type:json" json:"title"`                                    // 多语言标题
	SubtitleJSON jsonmap.JSON `gorm:"type:json" json:"subtitle"`                                 // 多语言副标题
	CTATextJSON  jsonmap.JSON `gorm:"type:json" json:"cta_text"`                                 // 主按钮文字（多语言，可留空）
	BuyTextJSON  jsonmap.JSON `gorm:"type:json" json:"buy_text"`                                 // 购买按钮文字（多语言，可留空）
	BuyLink      string       `gorm:"type:varchar(1000)" json:"buy_link"`                        // 购买按钮跳转地址（可留空）
	Tone         string       `gorm:"type:varchar(10)" json:"tone"`                              // 底色：dark / light / 空=自动
	Image        string       `gorm:"type:varchar(500);not null" json:"image"`                   // 主图
	MobileImage  string       `gorm:"type:varchar(500)" json:"mobile_image"`                     // 移动端图片
	LinkType     string       `gorm:"type:varchar(20);not null;default:'none'" json:"link_type"` // 跳转类型
	LinkValue    string       `gorm:"type:varchar(1000)" json:"link_value"`                      // 跳转值
	OpenInNewTab bool         `gorm:"default:false" json:"open_in_new_tab"`                      // 是否新窗口打开
	IsActive     bool         `gorm:"default:true;index" json:"is_active"`                       // 是否启用
	StartAt      *time.Time   `gorm:"index" json:"start_at"`                                     // 生效时间
	EndAt        *time.Time   `gorm:"index" json:"end_at"`                                       // 失效时间
	SortOrder    int          `gorm:"default:0;index" json:"sort_order"`                         // 排序
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`                                   // 创建时间
	UpdatedAt    time.Time    `json:"updated_at"`                                                // 更新时间
	DeletedAt    *time.Time   `gorm:"index" json:"-"`                                            // 软删除
}

// TableName 指定表名
func (Banner) TableName() string {
	return "banners"
}
