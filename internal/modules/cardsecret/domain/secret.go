package domain

import (
	"time"
)

const (
	StatusAvailable = "available"
	StatusReserved  = "reserved"
	StatusUsed      = "used"
)

// Secret 卡密库存实体。
type Secret struct {
	ID         uint       `gorm:"primarykey" json:"id"`                                                         // 主键
	ProductID  uint       `gorm:"not null;index:idx_card_secret_reserve" json:"product_id"`                     // 商品ID
	SKUID      uint       `gorm:"column:sku_id;not null;default:0;index:idx_card_secret_reserve" json:"sku_id"` // SKU ID
	BatchID    *uint      `gorm:"index" json:"batch_id,omitempty"`                                              // 批次ID
	Secret     string     `gorm:"type:text;not null" json:"secret"`                                             // 卡密内容
	Status     string     `gorm:"not null;index:idx_card_secret_reserve" json:"status"`                         // 状态（available/used）
	IsLoop     bool       `gorm:"not null;default:false" json:"is_loop"`                                        // 循环卡密：可重复发货，不占用不消耗
	OrderID    *uint      `gorm:"index" json:"order_id,omitempty"`                                              // 关联订单ID
	ReservedAt *time.Time `gorm:"index" json:"reserved_at"`                                                     // 占用时间
	UsedAt     *time.Time `gorm:"index" json:"used_at"`                                                         // 使用时间
	CreatedAt  time.Time  `gorm:"index" json:"created_at"`                                                      // 创建时间
	UpdatedAt  time.Time  `gorm:"index" json:"updated_at"`                                                      // 更新时间
	DeletedAt  *time.Time `gorm:"index" json:"-"`                                                               // 软删除时间

	Batch *Batch `gorm:"foreignKey:BatchID" json:"batch,omitempty"` // 批次信息
}

// TableName 指定表名
func (Secret) TableName() string {
	return "card_secrets"
}
