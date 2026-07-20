package models

import (
	"time"

	"gorm.io/gorm"
)

// MemberLevel 会员等级定义
type MemberLevel struct {
	ID                uint   `gorm:"primarykey" json:"id"`
	NameJSON          JSON   `gorm:"type:json;not null" json:"name"`                                  // 多语言名称
	Slug              string `gorm:"uniqueIndex;not null" json:"slug"`                                // 唯一标识（default/silver/gold/diamond）
	Icon              string `gorm:"default:''" json:"icon"`                                          // 等级图标（emoji 或图片 URL）
	DiscountRate      Money  `gorm:"type:decimal(6,2);not null;default:100" json:"discount_rate"`     // 全局折扣率（100=原价, 90=9折, 80=8折）
	RechargeThreshold Money  `gorm:"type:decimal(20,2);not null;default:0" json:"recharge_threshold"` // 充值累计升级阈值（0=不按此条件）
	SpendThreshold    Money  `gorm:"type:decimal(20,2);not null;default:0" json:"spend_threshold"`    // 消费累计升级阈值（0=不按此条件）
	IsDefault         bool   `gorm:"not null;default:false" json:"is_default"`                        // 是否默认等级（仅一个）
	SortOrder         int    `gorm:"not null;default:0" json:"sort_order"`                            // 排序权重（越大等级越高）
	IsActive          bool   `gorm:"not null;default:true" json:"is_active"`                          // 是否启用
	// MemberCount 属于该等级的会员数量（不持久化，查询时注入）
	MemberCount int64 `gorm:"-" json:"member_count"`

	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (MemberLevel) TableName() string {
	return "member_levels"
}
