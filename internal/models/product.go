package models

import (
	"time"

	"gorm.io/gorm"
)

// Product 商品表
type Product struct {
	ID                   uint                `gorm:"primarykey" json:"id"`                                                // 主键
	CategoryID           uint                `gorm:"not null;index" json:"category_id"`                                   // 分类ID
	Slug                 string              `gorm:"uniqueIndex;not null" json:"slug"`                                    // 唯一标识
	SeoMetaJSON          JSON                `gorm:"type:json" json:"seo_meta"`                                           // SEO 元数据
	TitleJSON            JSON                `gorm:"type:json;not null" json:"title"`                                     // 多语言标题
	DescriptionJSON      JSON                `gorm:"type:json" json:"description"`                                        // 多语言描述
	ContentJSON          JSON                `gorm:"type:json" json:"content"`                                            // 多语言详情（Markdown）
	InstructionsJSON     JSON                `gorm:"type:json" json:"instructions"`                                       // 多语言交付后使用说明（仅订单详情可见）
	PriceAmount          Money               `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`           // 价格金额
	CostPriceAmount      Money               `gorm:"type:decimal(20,2);not null;default:0" json:"cost_price_amount"`      // 成本价（取最低活跃SKU成本价）
	WholesalePrices      WholesalePriceTiers `gorm:"type:json" json:"wholesale_prices"`                                   // 批发价阶梯
	Images               StringArray         `gorm:"type:json" json:"images"`                                             // 图片数组
	Tags                 StringArray         `gorm:"type:json" json:"tags"`                                               // 标签数组
	PurchaseType         string              `gorm:"type:varchar(20);not null;default:'member'" json:"purchase_type"`     // 购买身份（guest/member）
	MinPurchaseQuantity  int                 `gorm:"not null;default:0" json:"min_purchase_quantity"`                     // 单次最小购买数量（0 表示不限制）
	MaxPurchaseQuantity  int                 `gorm:"not null;default:0" json:"max_purchase_quantity"`                     // 单次最大购买数量（0 表示不限制）
	StockDisplayMode     string              `gorm:"type:varchar(20);not null;default:'exact'" json:"stock_display_mode"` // 公开库存展示模式（exact/status/range/hidden）
	FulfillmentType      string              `gorm:"type:varchar(20);not null;default:'manual'" json:"fulfillment_type"`  // 交付类型（auto/manual）
	ManualFormSchemaJSON JSON                `gorm:"type:json" json:"manual_form_schema"`                                 // 人工交付表单 schema
	ManualStockTotal     int                 `gorm:"not null;default:0" json:"manual_stock_total"`                        // 手动剩余库存（-1 表示无限库存，>=0 表示当前可售数量）
	ManualStockLocked    int                 `gorm:"not null;default:0" json:"manual_stock_locked"`                       // 手动库存占用量（待支付）
	ManualStockSold      int                 `gorm:"not null;default:0" json:"manual_stock_sold"`                         // 手动库存已售量（支付成功后累加）
	PaymentChannelIDs    string              `gorm:"type:text" json:"payment_channel_ids"`                                // 允许的支付渠道ID（JSON数组字符串，空表示不限制）
	IsAffiliateEnabled   bool                `gorm:"not null;default:false;index" json:"is_affiliate_enabled"`            // 是否参与推广返利
	AutoStockAvailable   int64               `gorm:"-" json:"auto_stock_available"`                                       // 自动发货库存可用量（仅结构，不写入数据库）
	AutoStockTotal       int64               `gorm:"-" json:"auto_stock_total"`                                           // 自动发货库存总量（仅结构，不写入数据库）
	AutoStockLocked      int64               `gorm:"-" json:"auto_stock_locked"`                                          // 自动发货库存占用量（仅结构，不写入数据库）
	AutoStockSold        int64               `gorm:"-" json:"auto_stock_sold"`                                            // 自动发货库存已售量（仅结构，不写入数据库）
	IsMapped             bool                `gorm:"not null;default:false;index" json:"is_mapped"`                       // 是否为对接商品
	SupplierName         string              `gorm:"-" json:"supplier_name"`                                               // 上游供应商名称（DTO 承载，不落库）
	IsActive             bool                `gorm:"default:false;index" json:"is_active"`                                // 是否上架
	SortOrder            int                 `gorm:"default:0;index" json:"sort_order"`                                   // 排序权重
	CreatedAt            time.Time           `gorm:"index" json:"created_at"`                                             // 创建时间
	UpdatedAt            time.Time           `json:"updated_at"`                                                          // 更新时间
	DeletedAt            gorm.DeletedAt      `gorm:"index" json:"-"`                                                      // 软删除时间

	// 关联
	Category Category     `gorm:"foreignKey:CategoryID" json:"category,omitempty"` // 分类信息
	SKUs     []ProductSKU `gorm:"foreignKey:ProductID" json:"skus,omitempty"`      // SKU 列表
}

// TableName 指定表名
func (Product) TableName() string {
	return "products"
}
