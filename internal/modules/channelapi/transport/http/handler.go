package channelhttp

import (
	"context"
	"errors"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

var (
	ErrRiskIPBlacklisted             = errors.New("risk ip blacklisted")
	ErrRiskClientIPUnavailable       = errors.New("risk client ip unavailable")
	ErrRiskTooManyPendingOrders      = errors.New("too many pending orders")
	ErrRiskProductQuantityLimit      = errors.New("risk product quantity limit")
	ErrRiskPendingProductLimit       = errors.New("risk pending product quantity limit")
	ErrRiskOrderRateLimited          = errors.New("order rate limited")
	ErrProductSKURequired            = errors.New("product sku required")
	ErrProductSKUInvalid             = errors.New("product sku invalid")
	ErrInvalidOrderItem              = errors.New("invalid order item")
	ErrInvalidOrderAmount            = errors.New("invalid order amount")
	ErrProductPurchaseNotAllowed     = errors.New("product purchase not allowed")
	ErrProductMaxPurchaseExceeded    = errors.New("product max purchase exceeded")
	ErrProductMinPurchaseNotMet      = errors.New("product min purchase not met")
	ErrProductNotAvailable           = errors.New("product not available")
	ErrManualStockInsufficient       = errors.New("manual stock insufficient")
	ErrCardSecretInsufficient        = errors.New("card secret insufficient")
	ErrOrderCurrencyMismatch         = errors.New("order currency mismatch")
	ErrProductPriceInvalid           = errors.New("product price invalid")
	ErrManualFormSchemaInvalid       = errors.New("manual form schema invalid")
	ErrManualFormRequiredMissing     = errors.New("manual form required missing")
	ErrManualFormFieldInvalid        = errors.New("manual form field invalid")
	ErrManualFormTypeInvalid         = errors.New("manual form type invalid")
	ErrManualFormOptionInvalid       = errors.New("manual form option invalid")
	ErrPaymentInvalid                = errors.New("payment invalid")
	ErrOrderNotFound                 = errors.New("order not found")
	ErrOrderStatusInvalid            = errors.New("order status invalid")
	ErrPaymentChannelNotFound        = errors.New("payment channel not found")
	ErrPaymentChannelInactive        = errors.New("payment channel inactive")
	ErrPaymentProviderUnsupported    = errors.New("payment provider unsupported")
	ErrPaymentChannelConfigInvalid   = errors.New("payment channel config invalid")
	ErrPaymentGatewayRequestFailed   = errors.New("payment gateway request failed")
	ErrPaymentGatewayResponseInvalid = errors.New("payment gateway response invalid")
	ErrPaymentCurrencyMismatch       = errors.New("payment currency mismatch")
	ErrWalletOnlyPaymentRequired     = errors.New("wallet only payment required")
)

type TelegramIdentityInput struct {
	ChannelUserID string
	Username      string
	FirstName     string
	LastName      string
	AvatarURL     string
	InviteCode    string
}

type BindTelegramIdentityInput struct {
	Identity TelegramIdentityInput
	Email    string
	Code     string
}

type CreateOrderItem struct {
	ProductID       uint
	SKUID           uint
	Quantity        int
	FulfillmentType string
}

type CreateOrderInput struct {
	UserID              uint
	Items               []CreateOrderItem
	CouponCode          string
	AffiliateCode       string
	AffiliateVisitorKey string
	ClientIP            string
	ManualFormData      map[string]jsonmap.JSON
	SkipIPRiskControl   bool
}

type OrderPreview struct {
	Currency                string
	OriginalAmount          money.Amount
	DiscountAmount          money.Amount
	PromotionDiscountAmount money.Amount
	WholesaleDiscountAmount money.Amount
	TotalAmount             money.Amount
	Items                   []OrderPreviewItem
}

type OrderPreviewItem struct {
	ProductID, SKUID   uint
	TitleJSON          jsonmap.JSON
	SKUSnapshotJSON    jsonmap.JSON
	OriginalUnitPrice  money.Amount
	UnitPrice          money.Amount
	Quantity           int
	OriginalTotalPrice money.Amount
	TotalPrice         money.Amount
	CouponDiscount     money.Amount
	PromotionDiscount  money.Amount
	WholesaleDiscount  money.Amount
	FulfillmentType    string
}

type OrderListFilter struct {
	Page, PageSize int
	UserID         uint
	Status         string
}

type PaymentChannelListFilter struct {
	Page, PageSize int
	ActiveOnly     bool
}

type CreatePaymentInput struct {
	OrderID, ChannelID uint
	UseBalance         bool
	ClientIP           string
	Context            context.Context
}

type CreatePaymentResult struct {
	Payment          *paymentdomain.Payment
	Channel          *paymentdomain.PaymentChannel
	OrderPaid        bool
	WalletPaidAmount money.Amount
	OnlinePayAmount  money.Amount
}

type CategoryService interface {
	ListActive() ([]categorydomain.Category, error)
}

type CategoryRepository interface {
	CountActiveProducts(categoryID string) (int64, error)
}

type ProductService interface {
	ListPublicExact(categoryID string, page, pageSize int) ([]productdomain.Product, int64, error)
	ListPublic(categoryID, search string, page, pageSize int) ([]productdomain.Product, int64, error)
	ApplyAutoStockCounts(products []productdomain.Product) error
}

type ProductRepository interface {
	GetByID(id string) (*productdomain.Product, error)
}

type ProductMappingRepository interface {
	ListByLocalProductIDs(productIDs []uint) ([]mappingdomain.Mapping, error)
}

type SKUMappingRepository interface {
	ListByProductMappingIDs(mappingIDs []uint) ([]mappingdomain.SKUMapping, error)
}

type IdentityService interface {
	ResolveTelegramChannelIdentity(input TelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, error)
	ProvisionTelegramChannelIdentity(input TelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, bool, error)
	ProvisionTelegramChannelUserID(input TelegramIdentityInput) (uint, error)
	BindTelegramChannelByEmailCode(input BindTelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, uint, error)
}

type MemberLevelService interface {
	ResolveMemberPrice(levelID, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, decimal.Decimal)
}

type Settings interface {
	GetSiteCurrency(defaultValue string) (string, error)
	GetWalletOnlyPayment() bool
}

type Orders interface {
	PreviewOrder(input CreateOrderInput) (*OrderPreview, error)
	CreateOrder(input CreateOrderInput) (*orderdomain.Order, error)
	GetOrderByUser(orderID, userID uint) (*orderdomain.Order, error)
	GetOrderByUserOrderNo(orderNo string, userID uint) (*orderdomain.Order, error)
	CancelOrder(orderID, userID uint) (*orderdomain.Order, error)
	ListOrdersByUser(filter OrderListFilter) ([]orderdomain.Order, int64, error)
}

type Payments interface {
	GetWalletRechargeChannels() ([]paymentdomain.PaymentChannel, error)
	ListChannels(filter PaymentChannelListFilter) ([]paymentdomain.PaymentChannel, int64, error)
	GetAllowedChannelsForProducts(productIDs []uint) ([]paymentdomain.PaymentChannel, error)
	CreatePayment(input CreatePaymentInput) (*CreatePaymentResult, error)
}

type PaymentStoresitory interface {
	GetLatestPendingByOrder(orderID uint, now time.Time) (*paymentdomain.Payment, error)
	GetByID(id uint) (*paymentdomain.Payment, error)
}

type Dependencies struct {
	CategoryService    CategoryService
	CategoryRepo       CategoryRepository
	ProductService     ProductService
	ProductRepo        ProductRepository
	ProductMappingRepo ProductMappingRepository
	SKUMappingRepo     SKUMappingRepository
	UserAuthService    IdentityService
	MemberLevelService MemberLevelService
	SettingService     Settings
	OrderService       Orders
	PaymentService     Payments
	PaymentStore       PaymentStoresitory
}

type Handler struct {
	Dependencies
}

func New(dependencies Dependencies) *Handler {
	if dependencies.CategoryService == nil || dependencies.CategoryRepo == nil || dependencies.ProductService == nil ||
		dependencies.ProductRepo == nil || dependencies.ProductMappingRepo == nil || dependencies.SKUMappingRepo == nil ||
		dependencies.UserAuthService == nil || dependencies.MemberLevelService == nil || dependencies.SettingService == nil ||
		dependencies.OrderService == nil || dependencies.PaymentService == nil || dependencies.PaymentStore == nil {
		panic("channel handler: required dependency is nil")
	}
	return &Handler{Dependencies: dependencies}
}

type retryAfterError struct {
	cause   error
	seconds int64
}

func (e retryAfterError) Error() string { return e.cause.Error() }
func (e retryAfterError) Unwrap() error { return e.cause }

func WithRetryAfter(err error, seconds int64) error {
	if err == nil || seconds <= 0 {
		return err
	}
	return retryAfterError{cause: err, seconds: seconds}
}

func retryAfter(err error) int64 {
	var wrapped retryAfterError
	if errors.As(err, &wrapped) {
		return wrapped.seconds
	}
	return 0
}
