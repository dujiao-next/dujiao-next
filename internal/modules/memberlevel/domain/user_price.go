package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// UserProductPrice 用户专属商品价（一口价）。
// 下单时按「基础价 → 活动价 → 批发价 → 会员等级价」算出最终单价后，仅当专属价
// 严格低于该单价时才用它覆盖，并把活动价、批发价、会员等级价带来的优惠清零；
// 专属价更高时以原流程的价格结算。
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
