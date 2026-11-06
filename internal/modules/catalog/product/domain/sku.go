package productdomain

import (
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

const (
	// DefaultSKUCode 历史单规格商品迁移时使用的默认 SKU 编码
	DefaultSKUCode = "DEFAULT"
)

// ProductSKU 商品 SKU 表（v1：价格+库存维度）
type ProductSKU struct {
	ID                 uint         `gorm:"primarykey" json:"id"`                                                                       // 主键
	ProductID          uint         `gorm:"not null;index;uniqueIndex:idx_product_sku_code" json:"product_id"`                          // 商品ID
	SKUCode            string       `gorm:"column:sku_code;type:varchar(64);not null;uniqueIndex:idx_product_sku_code" json:"sku_code"` // SKU编码（同商品内唯一）
	SpecValuesJSON     jsonmap.JSON `gorm:"type:json" json:"spec_values"`                                                               // 规格值（如颜色/版本）
	PriceAmount        money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`                                  // SKU价格
	CostPriceAmount    money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"cost_price_amount"`                             // 成本价
	AgencyPriceAmount  money.Amount `gorm:"type:decimal(20,2);not null;default:0" json:"agency_price_amount"`                           // 代理价（供API对接与分销商成本）
	ManualStockTotal   int          `gorm:"not null;default:0" json:"manual_stock_total"`                                               // 手动剩余库存（-1 表示无限库存，>=0 表示当前可售数量）
	ManualStockLocked  int          `gorm:"not null;default:0" json:"manual_stock_locked"`                                              // 手动库存占用量（待支付）
	ManualStockSold    int          `gorm:"not null;default:0" json:"manual_stock_sold"`                                                // 手动库存已售量（支付成功后累加）
	AutoStockAvailable int64        `gorm:"-" json:"auto_stock_available"`                                                              // 自动发货库存可用量（仅结构，不写入数据库）
	AutoStockTotal     int64        `gorm:"-" json:"auto_stock_total"`                                                                  // 自动发货库存总量（仅结构，不写入数据库）
	AutoStockLocked    int64        `gorm:"-" json:"auto_stock_locked"`                                                                 // 自动发货库存占用量（仅结构，不写入数据库）
	AutoStockSold      int64        `gorm:"-" json:"auto_stock_sold"`                                                                   // 自动发货库存已售量（仅结构，不写入数据库）
	UpstreamStock      int          `gorm:"-" json:"upstream_stock"`                                                                    // 上游库存（-1=无限, 0=售罄, >0=有货；仅结构，不写入数据库）
	IsActive           bool         `gorm:"default:true;index" json:"is_active"`                                                        // 是否启用
	SortOrder          int          `gorm:"default:0;index" json:"sort_order"`                                                          // 排序权重
	CreatedAt          time.Time    `gorm:"index" json:"created_at"`                                                                    // 创建时间
	UpdatedAt          time.Time    `gorm:"index" json:"updated_at"`                                                                    // 更新时间
	DeletedAt          *time.Time   `gorm:"index" json:"-"`                                                                             // 软删除时间

	Product *Product `gorm:"foreignKey:ProductID" json:"product,omitempty"` // 关联商品
}

// TableName 指定表名
func (ProductSKU) TableName() string {
	return "product_skus"
}
