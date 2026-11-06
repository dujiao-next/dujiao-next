package upstreamhttp

import (
	"errors"
	"net/http"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	downstreamcallbackdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

const (
	upstreamUserIDKey       = "upstream_user_id"
	upstreamCredentialIDKey = "upstream_credential_id"
)

var (
	ErrProductNotFound       = errors.New("product not found")
	ErrOrderNotFound         = errors.New("order not found")
	ErrOrderCancelNotAllowed = errors.New("order cancel not allowed")
	ErrWalletInsufficient    = errors.New("wallet insufficient balance")
	ErrStockInsufficient     = errors.New("stock insufficient")
	ErrProductUnavailable    = errors.New("product unavailable")
	ErrSKUUnavailable        = errors.New("sku unavailable")
	ErrInvalidOrderItem      = errors.New("invalid order item")
	ErrManualFormInvalid     = errors.New("manual form invalid")
)

type CreateOrderItem struct {
	ProductID       uint
	SKUID           uint
	Quantity        int
	FulfillmentType string
}

type CreateOrderInput struct {
	UserID          uint
	Items           []CreateOrderItem
	ClientIP        string
	ManualFormData  map[string]jsonmap.JSON
	SkipRiskControl bool
	IsApiOrder      bool
}

type CreatePaymentInput struct {
	OrderID    uint
	UseBalance bool
	ClientIP   string
}

type CreatePaymentResult struct {
	OrderPaid bool
}

type CategoryRepository interface {
	List() ([]categorydomain.Category, error)
}

type ProductService interface {
	ListForUpstreamSync(updatedAfter *time.Time, includeInactive bool, page, pageSize int) ([]productdomain.Product, int64, error)
	ApplyAutoStockCounts(products []productdomain.Product) error
	GetAdminByID(id string) (*productdomain.Product, error)
}

type UserRepository interface {
	GetByID(id uint) (*userdomain.User, error)
}

type ProductRepository interface {
	GetByID(id string) (*productdomain.Product, error)
}

type SKURepository interface {
	GetByID(id uint) (*productdomain.ProductSKU, error)
}

type ProductMappingRepository interface {
	ListByLocalProductIDs(productIDs []uint) ([]mappingdomain.Mapping, error)
}

type SKUMappingRepository interface {
	GetByLocalSKUID(skuID uint) (*mappingdomain.SKUMapping, error)
}

type MemberLevelService interface {
	ResolveMemberPrice(levelID, productID, skuID uint, basePrice decimal.Decimal) (decimal.Decimal, decimal.Decimal)
	GetByID(id uint) (*memberleveldomain.MemberLevel, error)
}

type Settings interface {
	GetByKey(key string) (jsonmap.JSON, error)
	GetSiteCurrency(defaultValue string) (string, error)
}

type Wallet interface {
	GetAccount(userID uint) (*walletdomain.Account, error)
}

type Orders interface {
	CreateOrder(input CreateOrderInput) (*orderdomain.Order, error)
	GetOrderByUser(orderID, userID uint) (*orderdomain.Order, error)
	CancelOrder(orderID, userID uint) (*orderdomain.Order, error)
	BuildLocalRefundRecordsForOrder(order *orderdomain.Order) ([]jsonmap.JSON, error)
}

type Payments interface {
	CreatePayment(input CreatePaymentInput) (*CreatePaymentResult, error)
}

type ProcurementOrders interface {
	GetByLocalOrderNo(orderNo string) (*procurementdomain.Order, error)
	HandleUpstreamCallback(orderID uint, status string, fulfillment *procurementcontract.Fulfillment) error
}

type DownstreamOrderReferences interface {
	Create(ref *downstreamcallbackdomain.OrderRef) error
	GetByCredentialAndDownstreamNo(credentialID uint, downstreamOrderNo string) (*downstreamcallbackdomain.OrderRef, error)
}

type SiteConnections interface {
	GetByApiKey(apiKey string) (*siteconnectiondomain.Connection, error)
}

type SecretDecrypter interface {
	DecryptSecret(encrypted string) (string, error)
}

type Dependencies struct {
	Categories        CategoryRepository
	Products          ProductService
	Users             UserRepository
	ProductRepository ProductRepository
	SKUs              SKURepository
	ProductMappings   ProductMappingRepository
	SKUMappings       SKUMappingRepository
	MemberLevels      MemberLevelService
	Settings          Settings
	Wallet            Wallet
	Orders            Orders
	Payments          Payments
	Procurements      ProcurementOrders
	DownstreamRefs    DownstreamOrderReferences
	Connections       SiteConnections
	ConnectionSecrets SecretDecrypter
}

type Handler struct {
	Dependencies
}

func New(dependencies Dependencies) *Handler {
	if dependencies.Categories == nil || dependencies.Products == nil || dependencies.Users == nil ||
		dependencies.ProductRepository == nil || dependencies.SKUs == nil || dependencies.ProductMappings == nil ||
		dependencies.SKUMappings == nil || dependencies.MemberLevels == nil || dependencies.Settings == nil ||
		dependencies.Wallet == nil || dependencies.Orders == nil || dependencies.Payments == nil ||
		dependencies.Procurements == nil || dependencies.DownstreamRefs == nil || dependencies.Connections == nil ||
		dependencies.ConnectionSecrets == nil {
		panic("upstream handler: required dependency is nil")
	}
	return &Handler{Dependencies: dependencies}
}

func getUpstreamUserID(c *gin.Context) uint {
	value, _ := c.Get(upstreamUserIDKey)
	id, _ := value.(uint)
	return id
}

func getUpstreamCredentialID(c *gin.Context) uint {
	value, _ := c.Get(upstreamCredentialIDKey)
	id, _ := value.(uint)
	return id
}

func successResponse(c *gin.Context, data interface{}) {
	if data == nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	c.JSON(http.StatusOK, data)
}

func errorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"ok": false, "error_code": code, "error_message": message})
}
