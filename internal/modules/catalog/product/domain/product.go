package productdomain

import (
	"time"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
)

// Product 商品表
type Product struct {
	ID                   uint                `gorm:"primarykey" json:"id"`                                                // 主键
	CategoryID           uint                `gorm:"not null;index" json:"category_id"`                                   // 分类ID
	Slug                 string              `gorm:"uniqueIndex;not null" json:"slug"`                                    // 唯一标识
	SeoMetaJSON          jsonmap.JSON        `gorm:"type:json" json:"seo_meta"`                                           // SEO 元数据
	TitleJSON            jsonmap.JSON        `gorm:"type:json;not null" json:"title"`                                     // 多语言标题
	DescriptionJSON      jsonmap.JSON        `gorm:"type:json" json:"description"`                                        // 多语言描述
	ContentJSON          jsonmap.JSON        `gorm:"type:json" json:"content"`                                            // 多语言详情（Markdown）
	InstructionsJSON     jsonmap.JSON        `gorm:"type:json" json:"instructions"`                                       // 多语言交付后使用说明（仅订单详情可见）
	CompareJSON          jsonmap.JSON        `gorm:"type:json" json:"compare"`                                            // 商品对比配置（维度组、取值、和谁比）
	FaqJSON              jsonmap.JSON        `gorm:"type:json" json:"faq"`                                                // 常见问题（{items:[...]}）
	InBoxJSON            jsonmap.JSON        `gorm:"type:json" json:"in_box"`                                             // 包装内容（{image,items,note}）
	PriceAmount          money.Amount        `gorm:"type:decimal(20,2);not null;default:0" json:"price_amount"`           // 价格金额
	CostPriceAmount      money.Amount        `gorm:"type:decimal(20,2);not null;default:0" json:"cost_price_amount"`      // 成本价（取最低活跃SKU成本价）
	WholesalePrices      WholesalePriceTiers `gorm:"type:json" json:"wholesale_prices"`                                   // 批发价阶梯
	Images               jsonslice.Strings   `gorm:"type:json" json:"images"`                                             // 图片数组
	Tags                 jsonslice.Strings   `gorm:"type:json" json:"tags"`                                               // 标签数组
	PurchaseType         string              `gorm:"type:varchar(20);not null;default:'member'" json:"purchase_type"`     // 购买身份（guest/member）
	MinPurchaseQuantity  int                 `gorm:"not null;default:0" json:"min_purchase_quantity"`                     // 单次最小购买数量（0 表示不限制）
	MaxPurchaseQuantity  int                 `gorm:"not null;default:0" json:"max_purchase_quantity"`                     // 单次最大购买数量（0 表示不限制）
	StockDisplayMode     string              `gorm:"type:varchar(20);not null;default:'exact'" json:"stock_display_mode"` // 公开库存展示模式（exact/status/range/hidden）
	FulfillmentType      string              `gorm:"type:varchar(20);not null;default:'manual'" json:"fulfillment_type"`  // 交付类型（auto/manual）
	ManualFormSchemaJSON jsonmap.JSON        `gorm:"type:json" json:"manual_form_schema"`                                 // 人工交付表单 schema
	ManualStockTotal     int                 `gorm:"not null;default:0" json:"manual_stock_total"`                        // 手动剩余库存（-1 表示无限库存，>=0 表示当前可售数量）
	ManualStockLocked    int                 `gorm:"not null;default:0" json:"manual_stock_locked"`                       // 手动库存占用量（待支付）
	ManualStockSold      int                 `gorm:"not null;default:0" json:"manual_stock_sold"`                         // 手动库存已售量（支付成功后累加）
	PaymentChannelIDs    string              `gorm:"type:text" json:"payment_channel_ids"`                                // 允许的支付渠道ID（jsonmap.JSON数组字符串，空表示不限制）
	IsAffiliateEnabled   bool                `gorm:"not null;default:false;index" json:"is_affiliate_enabled"`            // 是否参与推广返利
	AutoStockAvailable   int64               `gorm:"-" json:"auto_stock_available"`                                       // 自动发货库存可用量（仅结构，不写入数据库）
	AutoStockTotal       int64               `gorm:"-" json:"auto_stock_total"`                                           // 自动发货库存总量（仅结构，不写入数据库）
	AutoStockLocked      int64               `gorm:"-" json:"auto_stock_locked"`                                          // 自动发货库存占用量（仅结构，不写入数据库）
	AutoStockSold        int64               `gorm:"-" json:"auto_stock_sold"`                                            // 自动发货库存已售量（仅结构，不写入数据库）
	IsMapped             bool                `gorm:"not null;default:false;index" json:"is_mapped"`                       // 是否为对接商品
	IsActive             bool                `gorm:"default:false;index" json:"is_active"`                                // 是否上架
	SortOrder            int                 `gorm:"default:0;index" json:"sort_order"`                                   // 排序权重
	CreatedAt            time.Time           `gorm:"index" json:"created_at"`                                             // 创建时间
	UpdatedAt            time.Time           `json:"updated_at"`                                                          // 更新时间
	DeletedAt            *time.Time          `gorm:"index" json:"-"`                                                      // 软删除时间

	// 关联
	Category categorydomain.Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"` // 分类信息
	SKUs     []ProductSKU            `gorm:"foreignKey:ProductID" json:"skus,omitempty"`      // SKU 列表
}

// TableName 指定表名
func (Product) TableName() string {
	return "products"
}

// CompareGroup 是商品对比维度组：一组对比项，同分类下的商品共用。
//
// 商品本身只存「值」（compare.values），维度定义放在这里，
// 前台据此自动生成「哪款 xx 适合你」的对比表，不需要每页手画版式。
type CompareGroup struct {
	ID         uint         `gorm:"primarykey" json:"id"`
	NameJSON   jsonmap.JSON `gorm:"type:json;not null" json:"name"`               // 组名（多语言）
	CategoryID *uint        `gorm:"index" json:"category_id"`                     // 绑定分类，nil 表示全局可用
	Dimensions jsonmap.JSON `gorm:"type:json" json:"dimensions"`                  // { items: [{key,label,icon,unit,type,group,visible,order}] }
	IsActive   bool         `gorm:"not null;default:true;index" json:"is_active"` // 是否启用
	SortOrder  int          `gorm:"not null;default:0;index" json:"sort_order"`   // 排序权重
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

// TableName 固定表名，避免 GORM 复数化差异。
func (CompareGroup) TableName() string { return "compare_groups" }

// CompareDimension 是对比维度（只用于 Go 侧读取，持久层仍是 JSON）。
type CompareDimension struct {
	Key     string       `json:"key"`
	Label   jsonmap.JSON `json:"label"`
	Icon    string       `json:"icon"`
	Unit    string       `json:"unit"`
	Type    string       `json:"type"`
	Group   string       `json:"group"`
	Visible bool         `json:"visible"`
	Order   int          `json:"order"`
}
