package application

import (
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// TrackClickInput 推广点击记录输入。
type TrackClickInput struct {
	AffiliateCode string
	VisitorKey    string
	LandingPath   string
	Referrer      string
	ClientIP      string
	UserAgent     string
}

// WithdrawApplyInput 提现申请输入。
type WithdrawApplyInput struct {
	Amount  decimal.Decimal
	Channel string
	Account string
}

// Dashboard 推广用户中心数据。
type Dashboard struct {
	Opened              bool         `json:"opened"`
	AffiliateCode       string       `json:"affiliate_code"`
	Status              string       `json:"status"` // 推广账号状态（active/disabled），停用后推广不再产生佣金
	PromotionPath       string       `json:"promotion_path"`
	ClickCount          int64        `json:"click_count"`
	ValidOrderCount     int64        `json:"valid_order_count"`
	ConversionRate      float64      `json:"conversion_rate"`
	PendingCommission   money.Amount `json:"pending_commission"`
	AvailableCommission money.Amount `json:"available_commission"`
	WithdrawnCommission money.Amount `json:"withdrawn_commission"`
}

// SKURate 是单个规格对推广员生效的返利比例。
type SKURate struct {
	SKUID       uint         `json:"sku_id"`
	RatePercent money.Amount `json:"rate_percent"`
}

// ProductRates 是推广员查看某个商品时生效的返利比例（已按 SKU → 商品 → 全站解析）。
// 0 表示该商品/规格不参与返利。
type ProductRates struct {
	RatePercent money.Amount `json:"rate_percent"`
	SKUs        []SKURate    `json:"skus"`
}

// Stats 推广统计数据。
type Stats struct {
	ClickCount          int64
	ValidOrderCount     int64
	ConversionRate      float64
	PendingCommission   money.Amount
	AvailableCommission money.Amount
	WithdrawnCommission money.Amount
}

// AdminUserItem 后台推广用户列表项。
type AdminUserItem struct {
	Profile affiliatedomain.Profile `json:"profile"`
	Stats   Stats                   `json:"stats"`
}

// AdminProfileListFilter 后台推广用户列表过滤。
type AdminProfileListFilter struct {
	Page     int
	PageSize int
	UserID   uint
	Status   string
	Code     string
	Keyword  string
}

// AdminCommissionListFilter 后台佣金列表过滤。
type AdminCommissionListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	OrderNo            string
	Status             string
	Keyword            string
}

// AdminWithdrawListFilter 后台提现列表过滤。
type AdminWithdrawListFilter struct {
	Page               int
	PageSize           int
	AffiliateProfileID uint
	Status             string
	Keyword            string
}
