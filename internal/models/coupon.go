package models

import (
	"time"

	"gorm.io/gorm"
)

// Coupon 优惠券
type Coupon struct {
	ID                     uint           `gorm:"primarykey" json:"id"`                                      // 主键
	Code                   string         `gorm:"uniqueIndex;not null" json:"code"`                          // 优惠码
	Type                   string         `gorm:"not null" json:"type"`                                      // 类型（fixed/percent）
	Value                  Money          `gorm:"type:decimal(20,2);not null" json:"value"`                  // 数值（固定金额或百分比）
	MinAmount              Money          `gorm:"type:decimal(20,2);not null;default:0" json:"min_amount"`   // 使用门槛
	MaxDiscount            Money          `gorm:"type:decimal(20,2);not null;default:0" json:"max_discount"` // 最大优惠金额
	UsageLimit             int            `gorm:"not null;default:0" json:"usage_limit"`                     // 总使用上限（0 表示不限制）
	UsedCount              int            `gorm:"not null;default:0" json:"used_count"`                      // 已使用次数
	PerUserLimit           int            `gorm:"not null;default:0" json:"per_user_limit"`                  // 每人使用上限（0 表示不限制）
	DisabledWholesalePrice bool           `gorm:"not null;default:false" json:"disabled_wholesale_price"`    // 是否禁止批发价商品使用
	PerItemDiscount        bool           `gorm:"not null;default:false" json:"per_item_discount"`           // 固定金额券是否按商品数量抵扣
	PaymentRoles           StringArray    `gorm:"type:json" json:"payment_roles"`                            // 付款角色限制（留空不限制）
	MemberLevels           UintArray      `gorm:"type:json" json:"member_levels"`                            // 会员等级限制（留空不限制）
	ScopeType              string         `gorm:"not null" json:"scope_type"`                                // 适用范围（product）
	ScopeRefIDs            string         `gorm:"type:text" json:"scope_ref_ids"`                            // 适用商品ID集合（JSON数组）
	StartsAt               *time.Time     `gorm:"index" json:"starts_at"`                                    // 生效时间
	EndsAt                 *time.Time     `gorm:"index" json:"ends_at"`                                      // 失效时间
	IsActive               bool           `gorm:"not null;default:true" json:"is_active"`                    // 是否启用
	CreatedAt              time.Time      `gorm:"index" json:"created_at"`                                   // 创建时间
	UpdatedAt              time.Time      `gorm:"index" json:"updated_at"`                                   // 更新时间
	DeletedAt              gorm.DeletedAt `gorm:"index" json:"-"`                                            // 软删除时间
}

// TableName 指定表名
func (Coupon) TableName() string {
	return "coupons"
}
