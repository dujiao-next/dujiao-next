package domain

import (
	"time"

	"github.com/dujiao-next/internal/shared/money"
)

// UserProductPrice 用户专属商品价，两种定价模式二选一：
//   - 一口价（PriceAmount > 0，DiscountRate = 0）：命中后直接作为结算单价；
//   - 百分比折扣（DiscountRate 1~99，PriceAmount = 0）：按商品/SKU 当前售价乘以折扣率。
//
// 下单时按「基础价 → 活动价 → 批发价 → 会员等级价」算出最终单价后，仅当专属价
// 严格低于该单价时才用它覆盖，并把活动价、批发价、会员等级价带来的优惠清零；
// 专属价更高时以原流程的价格结算。
//
// DiscountRate 用 0 表示"未启用"而非 100，是因为它是 Go 的零值：老客户端不传该字段
// 时必须退化成一口价，不能变成免费。
type UserProductPrice struct {
	ID           uint         `gorm:"primarykey" json:"id"`
	UserID       uint         `gorm:"uniqueIndex:idx_user_product_price;not null" json:"user_id"`                        // 关联用户
	ProductID    uint         `gorm:"uniqueIndex:idx_user_product_price;not null" json:"product_id"`                     // 关联商品
	SKUID        uint         `gorm:"column:sku_id;uniqueIndex:idx_user_product_price;not null;default:0" json:"sku_id"` // 0=商品级，>0=SKU级
	PriceAmount  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`                         // 专属一口价（折扣模式下为 0）
	DiscountRate money.Amount `gorm:"type:decimal(6,2);not null;default:0" json:"discount_rate"`                         // 折扣百分比（80=八折，0=未启用）
	CreatedAt    time.Time    `gorm:"index" json:"created_at"`
	UpdatedAt    time.Time    `gorm:"index" json:"updated_at"`
	DeletedAt    *time.Time   `gorm:"index" json:"-"`
}

func (UserProductPrice) TableName() string {
	return "user_product_prices"
}
