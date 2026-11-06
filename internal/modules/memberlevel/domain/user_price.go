package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// UserProductPrice 用户专属商品价（一口价）。
// 命中后直接作为该用户的结算单价，不再叠加活动价、批发价和会员等级价。
type UserProductPrice struct {
	ID          uint         `gorm:"primarykey" json:"id"`
	UserID      uint         `gorm:"uniqueIndex:idx_user_product_price;not null" json:"user_id"`                        // 关联用户
	ProductID   uint         `gorm:"uniqueIndex:idx_user_product_price;not null" json:"product_id"`                     // 关联商品
	SKUID       uint         `gorm:"column:sku_id;uniqueIndex:idx_user_product_price;not null;default:0" json:"sku_id"` // 0=商品级，>0=SKU级
	PriceAmount money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`                         // 专属一口价
	CreatedAt   time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt   *time.Time   `gorm:"index" json:"-"`
}

func (UserProductPrice) TableName() string {
	return "user_product_prices"
}
