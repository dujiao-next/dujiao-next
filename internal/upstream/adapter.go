package upstream

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

// 上游商品状态相关哨兵错误（adapter 层归一化）
var (
	// ErrUpstreamProductDeleted 上游商品已被软删除（不再存在）
	ErrUpstreamProductDeleted = errors.New("upstream product deleted")
	// ErrUpstreamProductUnavailable 上游商品已下架（仍然存在但不可购）
	// 旧版上游对下架商品返回 product_unavailable，新版改为 200 + is_active=false；
	// 此错误仅作为旧版兼容兜底使用。
	ErrUpstreamProductUnavailable = errors.New("upstream product unavailable")
)

// PingResult 连接测试结果
type PingResult struct {
	SiteName        string                 `json:"site_name"`
	ProtocolVersion string                 `json:"protocol_version"`
	UserID          uint                   `json:"user_id"`
	Balance         string                 `json:"balance"`
	Currency        string                 `json:"currency"`
	MemberLevel     map[string]interface{} `json:"member_level,omitempty"`
}

// ListProductsOpts 商品列表参数
type ListProductsOpts struct {
	Page            int
	PageSize        int
	UpdatedAfter    *time.Time
	IncludeInactive bool // 是否包含已下架商品（用于下游识别上游下架/删除）
}

// ProductListResult 商品列表结果
type ProductListResult struct {
	Total int               `json:"total"`
	Items []UpstreamProduct `json:"items"`
	// IncludesInactive 上游是否真的返回了下架商品（响应回声字段）
	// 旧上游不识别 include_inactive 参数，此字段为 false，下游不可据此推断"missing=已删除"
	IncludesInactive bool `json:"includes_inactive"`
}

// UpstreamProduct 上游商品信息
type UpstreamProduct struct {
	ID               uint                       `json:"id"`
	SeoMeta          models.JSON                `json:"seo_meta"`
	Title            models.JSON                `json:"title"`
	Description      models.JSON                `json:"description"`
	Content          models.JSON                `json:"content"`
	Images           []string                   `json:"images"`
	Tags             []string                   `json:"tags"`
	PriceAmount      string                     `json:"price_amount"`
	OriginalPrice    string                     `json:"original_price,omitempty"`
	MemberPrice      string                     `json:"member_price,omitempty"`
	WholesalePrices  models.WholesalePriceTiers `json:"wholesale_prices,omitempty"`
	Currency         string                     `json:"currency"`
	FulfillmentType  string                     `json:"fulfillment_type"`
	ManualFormSchema models.JSON                `json:"manual_form_schema"`
	IsActive         bool                       `json:"is_active"`
	CategoryID       uint                       `json:"category_id"`
	SKUs             []UpstreamSKU              `json:"skus"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

// UpstreamCategory 上游分类信息
type UpstreamCategory struct {
	ID        uint        `json:"id"`
	ParentID  uint        `json:"parent_id"`
	Slug      string      `json:"slug"`
	Name      models.JSON `json:"name"`
	Icon      string      `json:"icon"`
	SortOrder int         `json:"sort_order"`
}

// CategoryListResult 分类列表结果
type CategoryListResult struct {
	Supported  bool               `json:"supported"`
	Categories []UpstreamCategory `json:"categories"`
}

// UpstreamSKU 上游 SKU 信息
type UpstreamSKU struct {
	ID            uint        `json:"id"`
	SKUCode       string      `json:"sku_code"`
	SpecValues    models.JSON `json:"spec_values"`
	PriceAmount   string      `json:"price_amount"`
	OriginalPrice string      `json:"original_price,omitempty"`
	MemberPrice   string      `json:"member_price,omitempty"`
	StockStatus   string      `json:"stock_status"`
	StockQuantity int         `json:"stock_quantity"` // 实际可用库存（-1=无限）
	IsActive      bool        `json:"is_active"`
}

// CreateUpstreamOrderReq 创建上游采购单请求
type CreateUpstreamOrderReq struct {
	SKUID             uint        `json:"sku_id"`
	Quantity          int         `json:"quantity"`
	ManualFormData    models.JSON `json:"manual_form_data,omitempty"`
	DownstreamOrderNo string      `json:"downstream_order_no"`
	TraceID           string      `json:"trace_id"`
	CallbackURL       string      `json:"callback_url"`

	// Local-only fields are consumed by protocols that submit local product data.
	// They are excluded from the Dujiao-Next wire payload for compatibility.
	LocalProductID uint   `json:"-"`
	LocalSKUID     uint   `json:"-"`
	LocalSKUCode   string `json:"-"`
	Amount         string `json:"-"`
	Currency       string `json:"-"`
}

// CreateUpstreamOrderResp 创建上游采购单响应
type CreateUpstreamOrderResp struct {
	OK           bool   `json:"ok"`
	OrderID      uint   `json:"order_id,omitempty"`
	OrderNo      string `json:"order_no,omitempty"`
	Status       string `json:"status,omitempty"`
	Amount       string `json:"amount,omitempty"`
	Currency     string `json:"currency,omitempty"`
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// UpstreamFulfillment 上游交付信息
type UpstreamFulfillment struct {
	Type         string      `json:"type"`
	Status       string      `json:"status"`
	Payload      string      `json:"payload"`
	DeliveryData models.JSON `json:"delivery_data"`
	DeliveredAt  *time.Time  `json:"delivered_at,omitempty"`
}

// UpstreamOrderDetail 上游订单详情
type UpstreamOrderDetail struct {
	OrderID        uint                 `json:"order_id"`
	OrderNo        string               `json:"order_no"`
	Status         string               `json:"status"`
	Amount         string               `json:"amount"`
	RefundedAmount string               `json:"refunded_amount,omitempty"`
	Currency       string               `json:"currency"`
	Fulfillment    *UpstreamFulfillment `json:"fulfillment,omitempty"`
	// RefundRecords 上游退款记录（协议兼容字段）
	RefundRecords []models.JSON `json:"refund_records,omitempty"`
}

// Adapter 上游站点适配器接口
type Adapter interface {
	// Ping 连接测试
	Ping(ctx context.Context) (*PingResult, error)

	// ListCategories 拉取上游分类列表
	ListCategories(ctx context.Context) (*CategoryListResult, error)

	// ListProducts 拉取上游商品列表
	ListProducts(ctx context.Context, opts ListProductsOpts) (*ProductListResult, error)

	// GetProduct 获取单个商品详情
	GetProduct(ctx context.Context, productID uint) (*UpstreamProduct, error)

	// CreateOrder 发起采购单
	CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error)

	// GetOrder 查询上游订单状态
	GetOrder(ctx context.Context, orderID uint) (*UpstreamOrderDetail, error)

	// CancelOrder 取消采购单
	CancelOrder(ctx context.Context, orderID uint) error

	// DownloadImage 下载图片到本地
	DownloadImage(ctx context.Context, imageURL string) (localPath string, err error)
}

// OrderSubmitter is the minimal capability required by the procurement flow.
// Protocols that only accept pushed orders do not need to implement catalog or query APIs.
type OrderSubmitter interface {
	CreateOrder(ctx context.Context, req CreateUpstreamOrderReq) (*CreateUpstreamOrderResp, error)
}

// ConnectionTester provides the connection check used by the admin API.
type ConnectionTester interface {
	Ping(ctx context.Context) (*PingResult, error)
}

// NewAdapter 根据协议类型创建适配器
func NewAdapter(conn *models.SiteConnection, uploadsDir string) (Adapter, error) {
	switch conn.Protocol {
	case constants.ConnectionProtocolDujiaoNext:
		return NewDujiaoNextAdapter(conn, uploadsDir), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", conn.Protocol)
	}
}

// NewOrderSubmitter creates only the order-submission capability for a protocol.
func NewOrderSubmitter(conn *models.SiteConnection, uploadsDir string) (OrderSubmitter, error) {
	switch conn.Protocol {
	case constants.ConnectionProtocolDujiaoNext:
		return NewDujiaoNextAdapter(conn, uploadsDir), nil
	case constants.ConnectionProtocolGenericWebhook:
		return NewGenericWebhookAdapter(conn), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", conn.Protocol)
	}
}

// NewConnectionTester creates the protocol-specific connection test capability.
func NewConnectionTester(conn *models.SiteConnection, uploadsDir string) (ConnectionTester, error) {
	switch conn.Protocol {
	case constants.ConnectionProtocolDujiaoNext:
		return NewDujiaoNextAdapter(conn, uploadsDir), nil
	case constants.ConnectionProtocolGenericWebhook:
		return NewGenericWebhookAdapter(conn), nil
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", conn.Protocol)
	}
}
