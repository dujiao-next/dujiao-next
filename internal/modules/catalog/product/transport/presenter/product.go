package presenter

import (
	"time"

	categorypresenter "github.com/dujiao-next/internal/modules/catalog/category/transport/presenter"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// Product 是公开商品响应。
type Product struct {
	ID                   uint              `json:"id"`
	CategoryID           uint              `json:"category_id"`
	Slug                 string            `json:"slug"`
	SeoMeta              jsonmap.JSON      `json:"seo_meta"`
	Title                jsonmap.JSON      `json:"title"`
	Description          jsonmap.JSON      `json:"description"`
	Content              jsonmap.JSON      `json:"content"`
	Compare              jsonmap.JSON      `json:"compare,omitempty"`
	Faq                  jsonmap.JSON      `json:"faq,omitempty"`
	InBox                jsonmap.JSON      `json:"in_box,omitempty"`
	PriceAmount          money.Amount      `json:"price_amount"`
	WholesalePrices      []WholesalePrice  `json:"wholesale_prices,omitempty"`
	Images               jsonslice.Strings `json:"images"`
	Tags                 jsonslice.Strings `json:"tags"`
	PurchaseType         string            `json:"purchase_type"`
	MinPurchaseQuantity  int               `json:"min_purchase_quantity"`
	MaxPurchaseQuantity  int               `json:"max_purchase_quantity"`
	StockDisplayMode     string            `json:"stock_display_mode"`
	StockDisplay         string            `json:"stock_display"`
	StockRangeMin        *int              `json:"stock_range_min,omitempty"`
	StockRangeMax        *int              `json:"stock_range_max,omitempty"`
	StockQuantityHidden  bool              `json:"stock_quantity_hidden"`
	FulfillmentType      string            `json:"fulfillment_type"`
	ManualFormSchema     jsonmap.JSON      `json:"manual_form_schema"`
	ManualStockAvailable int               `json:"manual_stock_available"`
	AutoStockAvailable   int64             `json:"auto_stock_available"`
	StockStatus          string            `json:"stock_status"`
	IsSoldOut            bool              `json:"is_sold_out"`

	PaymentChannelIDs []uint `json:"payment_channel_ids,omitempty"`

	Category categorypresenter.Category `json:"category,omitempty"`
	SKUs     []SKU                      `json:"skus,omitempty"`

	PromotionID          *uint              `json:"promotion_id,omitempty"`
	PromotionName        string             `json:"promotion_name,omitempty"`
	PromotionType        string             `json:"promotion_type,omitempty"`
	PromotionPriceAmount *money.Amount      `json:"promotion_price_amount,omitempty"`
	PromotionRules       []PromotionRule    `json:"promotion_rules,omitempty"`
	MemberPrices         []MemberLevelPrice `json:"member_prices,omitempty"`

	RelatedPosts []RelatedPost `json:"related_posts,omitempty"`
}

// RelatedPost 是商品详情中的关联文章轻量响应。
type RelatedPost struct {
	ID          uint         `json:"id"`
	Slug        string       `json:"slug"`
	Type        string       `json:"type"`
	Title       jsonmap.JSON `json:"title"`
	Summary     jsonmap.JSON `json:"summary,omitempty"`
	Thumbnail   string       `json:"thumbnail,omitempty"`
	PublishedAt *time.Time   `json:"published_at"`
}

// WholesalePrice 是公开批发价响应，金额固定输出为两位小数字符串。
type WholesalePrice struct {
	SKUID       uint   `json:"sku_id,omitempty"`
	SKUCode     string `json:"sku_code,omitempty"`
	MinQuantity int    `json:"min_quantity"`
	UnitPrice   string `json:"unit_price"`
}

// WholesalePrices 统一公开 API 的批发价金额格式。
func WholesalePrices(tiers productdomain.WholesalePriceTiers) []WholesalePrice {
	if len(tiers) == 0 {
		return nil
	}
	result := make([]WholesalePrice, 0, len(tiers))
	for _, tier := range tiers {
		if tier.MinQuantity <= 0 || tier.UnitPrice.Decimal.LessThanOrEqual(decimal.Zero) {
			continue
		}
		result = append(result, WholesalePrice{
			SKUID:       tier.SKUID,
			SKUCode:     tier.SKUCode,
			MinQuantity: tier.MinQuantity,
			UnitPrice:   tier.UnitPrice.String(),
		})
	}
	return result
}

// SKU 是公开商品 SKU 响应。
type SKU struct {
	ID                  uint         `json:"id"`
	SKUCode             string       `json:"sku_code"`
	SpecValues          jsonmap.JSON `json:"spec_values"`
	PriceAmount         money.Amount `json:"price_amount"`
	ManualStockTotal    int          `json:"manual_stock_total"`
	ManualStockSold     int          `json:"manual_stock_sold"`
	AutoStockAvailable  int64        `json:"auto_stock_available"`
	UpstreamStock       int          `json:"upstream_stock"`
	StockStatus         string       `json:"stock_status"`
	StockDisplayMode    string       `json:"stock_display_mode"`
	StockDisplay        string       `json:"stock_display"`
	StockRangeMin       *int         `json:"stock_range_min,omitempty"`
	StockRangeMax       *int         `json:"stock_range_max,omitempty"`
	StockQuantityHidden bool         `json:"stock_quantity_hidden"`
	IsSoldOut           bool         `json:"is_sold_out"`
	IsActive            bool         `json:"is_active"`

	PromotionPriceAmount *money.Amount `json:"promotion_price_amount,omitempty"`
	MemberPriceAmount    *money.Amount `json:"member_price_amount,omitempty"`
}

// PromotionRule 是公开促销规则响应。
type PromotionRule struct {
	ID        uint         `json:"id"`
	Name      string       `json:"name"`
	Type      string       `json:"type"`
	Value     money.Amount `json:"value"`
	MinAmount money.Amount `json:"min_amount"`
}

// MemberLevelPrice 是公开会员等级价格响应。
type MemberLevelPrice struct {
	MemberLevelID uint         `json:"member_level_id"`
	SKUID         uint         `json:"sku_id"`
	PriceAmount   money.Amount `json:"price_amount"`
}
