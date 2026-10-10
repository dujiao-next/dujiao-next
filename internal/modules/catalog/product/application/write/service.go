package productwrite

import (
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/shopspring/decimal"
)

// ProductRepository 是商品创建和更新所需的最小持久化端口。
type ProductRepository interface {
	GetByID(id string) (*productdomain.Product, error)
	Create(item *productdomain.Product) error
	Update(item *productdomain.Product) error
	CountBySlug(slug string, excludeID *string) (int64, error)
	QuickUpdate(id string, fields map[string]interface{}) error
}

// SKURepository 是商品写入用例所需的 SKU 持久化端口。
type SKURepository interface {
	ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error)
	Create(item *productdomain.ProductSKU) error
	Update(item *productdomain.ProductSKU) error
	Delete(id uint) error
	PurgeSoftDeletedByProductAndCode(productID uint, skuCode string) error
}

// CategoryRepository 是商品分类归属校验所需的最小端口。
type CategoryRepository interface {
	productdomain.CategoryAssignmentRepository
}

// PaymentChannelStoresitory 是商品允许支付渠道过滤所需的最小端口。
type PaymentChannelStoresitory interface {
	ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error)
}

// CardSecretStockRepository 是修改自动发货 SKU 前所需的库存保护端口。
type CardSecretStockRepository interface {
	CountByProduct(productID, skuID uint) (int64, int64, int64, error)
}

// TransactionRepositories 是一次商品写事务内绑定的仓储集合。
type TransactionRepositories struct {
	Products    ProductRepository
	SKUs        SKURepository
	CardSecrets CardSecretStockRepository
}

// UnitOfWork 隐藏具体数据库和事务对象，不向 Application 暴露 GORM。
type UnitOfWork interface {
	WithinTransaction(fn func(repositories TransactionRepositories) error) error
}

// Options 描述商品写入应用服务依赖。
type Options struct {
	Products        ProductRepository
	SKUs            SKURepository
	Categories      CategoryRepository
	PaymentChannels PaymentChannelStoresitory
	Transactions    UnitOfWork
}

// WriteService 编排商品创建、更新和 SKU 同步。
type WriteService struct {
	products        ProductRepository
	skus            SKURepository
	categories      CategoryRepository
	paymentChannels PaymentChannelStoresitory
	transactions    UnitOfWork
}

// NewWriteService 创建商品写入应用服务。
func NewWriteService(options Options) *WriteService {
	return &WriteService{
		products:        options.Products,
		skus:            options.SKUs,
		categories:      options.Categories,
		paymentChannels: options.PaymentChannels,
		transactions:    options.Transactions,
	}
}

// CreateProductInput 创建或完整更新商品的输入。
type CreateProductInput struct {
	CategoryID           uint
	Slug                 string
	SeoMetaJSON          map[string]interface{}
	TitleJSON            map[string]interface{}
	DescriptionJSON      map[string]interface{}
	ContentJSON          map[string]interface{}
	InstructionsJSON     map[string]interface{}
	ManualFormSchemaJSON map[string]interface{}
	PriceAmount          decimal.Decimal
	CostPriceAmount      decimal.Decimal
	AgencyPriceAmount    decimal.Decimal
	// WholesalePrices 为可选字段：nil 表示更新时保留，非 nil 表示整体覆盖。
	WholesalePrices     *[]productdomain.WholesalePriceInput
	Images              []string
	Tags                []string
	PurchaseType        string
	MinPurchaseQuantity *int
	MaxPurchaseQuantity *int
	StockDisplayMode    string
	FulfillmentType     string
	ManualStockTotal    *int
	SKUs                []ProductSKUInput
	PaymentChannelIDs   []uint
	IsAffiliateEnabled  *bool
	IsActive            *bool
	SortOrder           int
}

// ProductSKUInput 描述商品 SKU 的完整写入值。
type ProductSKUInput struct {
	ID                uint
	SKUCode           string
	SpecValuesJSON    map[string]interface{}
	PriceAmount       decimal.Decimal
	CostPriceAmount   decimal.Decimal
	AgencyPriceAmount decimal.Decimal
	ManualStockTotal  int
	IsActive          *bool
	SortOrder         int
}

func (s *WriteService) filterAvailablePaymentChannelIDs(ids []uint) ([]uint, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	uniqueIDs := make([]uint, 0, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}
	if len(uniqueIDs) == 0 {
		return nil, nil
	}
	if s.paymentChannels == nil {
		return uniqueIDs, nil
	}

	channels, err := s.paymentChannels.ListByIDs(uniqueIDs)
	if err != nil {
		return nil, err
	}
	activeIDs := make(map[uint]struct{}, len(channels))
	for _, channel := range channels {
		if channel.IsActive {
			activeIDs[channel.ID] = struct{}{}
		}
	}

	filtered := make([]uint, 0, len(uniqueIDs))
	for _, id := range uniqueIDs {
		if _, ok := activeIDs[id]; ok {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	return filtered, nil
}
