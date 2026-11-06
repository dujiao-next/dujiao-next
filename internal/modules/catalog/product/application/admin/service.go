package productadmin

import productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

// ProductRepository 是商品管理用例所需的最小商品端口。
type ProductRepository interface {
	GetByID(id string) (*productdomain.Product, error)
	GetAdminByID(id string) (*productdomain.Product, error)
	QuickUpdate(id string, fields map[string]interface{}) error
}

// CategoryRepository 是商品上架分类校验所需的最小分类端口。
type CategoryRepository interface {
	productdomain.CategoryAssignmentRepository
}

// CardSecretStockRepository 提供删除商品前的库存占用检查。
type CardSecretStockRepository interface {
	CountAvailable(productID, skuID uint) (int64, error)
	CountReserved(productID, skuID uint) (int64, error)
}

// OrderHistoryRepository 提供删除商品前的成交记录检查。
type OrderHistoryRepository interface {
	CountOrderItemsByProduct(productID uint) (int64, error)
}

type ProductDeleteRepository interface {
	Delete(id string) error
}

type CardSecretDeleteRepository interface {
	DeleteByProduct(productID uint) error
}

type CardSecretBatchDeleteRepository interface {
	DeleteByProduct(productID uint) error
}

type SKUDeleteRepository interface {
	DeleteByProduct(productID uint) error
}

type MemberLevelPriceDeleteRepository interface {
	DeleteByProduct(productID uint) error
}

// UserPriceDeleteRepository 清理用户专属价，与会员等级价同款。
type UserPriceDeleteRepository interface {
	DeleteByProduct(productID uint) error
}

type CartDeleteRepository interface {
	DeleteByProduct(productID uint) error
}

type ProductMappingDeleteRepository interface {
	DeleteByLocalProduct(productID uint) error
}

// DeleteRepositories 是一次商品级联删除事务中绑定的全部窄端口。
type DeleteRepositories struct {
	Products          ProductDeleteRepository
	CardSecrets       CardSecretDeleteRepository
	CardSecretBatches CardSecretBatchDeleteRepository
	SKUs              SKUDeleteRepository
	MemberLevelPrices MemberLevelPriceDeleteRepository
	UserPrices        UserPriceDeleteRepository
	Carts             CartDeleteRepository
	ProductMappings   ProductMappingDeleteRepository
}

// UnitOfWork 隔离应用层与具体数据库事务实现。
type UnitOfWork interface {
	WithinTransaction(fn func(DeleteRepositories) error) error
}

type Options struct {
	Products     ProductRepository
	Categories   CategoryRepository
	CardSecrets  CardSecretStockRepository
	Orders       OrderHistoryRepository
	Transactions UnitOfWork
}

// AdminService 承载商品删除、快速更新和批发价管理用例。
type AdminService struct {
	products     ProductRepository
	categories   CategoryRepository
	cardSecrets  CardSecretStockRepository
	orders       OrderHistoryRepository
	transactions UnitOfWork
}

func NewAdminService(options Options) *AdminService {
	return &AdminService{
		products:     options.Products,
		categories:   options.Categories,
		cardSecrets:  options.CardSecrets,
		orders:       options.Orders,
		transactions: options.Transactions,
	}
}
