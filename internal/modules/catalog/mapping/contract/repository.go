package contract

import (
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
)

// ListFilter 是商品映射列表筛选条件。
type ListFilter struct {
	ConnectionID   uint
	UpstreamStatus string
	ProductStatus  string
	Search         string
	Page           int
	PageSize       int
}

// MappingRepository 是商品映射持久化端口。
type MappingRepository interface {
	GetByID(id uint) (*mappingdomain.Mapping, error)
	GetByLocalProductID(productID uint) (*mappingdomain.Mapping, error)
	GetByConnectionAndUpstreamID(connectionID, upstreamProductID uint) (*mappingdomain.Mapping, error)
	Create(mapping *mappingdomain.Mapping) error
	Update(mapping *mappingdomain.Mapping) error
	Delete(id uint) error
	DeleteByLocalProduct(productID uint) error
	List(filter ListFilter) ([]mappingdomain.Mapping, int64, error)
	ListByLocalProductIDs(productIDs []uint) ([]mappingdomain.Mapping, error)
	ListActiveByConnection(connectionID uint) ([]mappingdomain.Mapping, error)
	ListAllActive() ([]mappingdomain.Mapping, error)
	ListUpstreamIDsByConnection(connectionID uint) ([]uint, error)
}

// SKUMappingRepository 是 SKU 映射持久化端口。
type SKUMappingRepository interface {
	GetByLocalSKUID(skuID uint) (*mappingdomain.SKUMapping, error)
	ListByProductMapping(productMappingID uint) ([]mappingdomain.SKUMapping, error)
	ListByProductMappingIDs(productMappingIDs []uint) ([]mappingdomain.SKUMapping, error)
	Create(mapping *mappingdomain.SKUMapping) error
	Update(mapping *mappingdomain.SKUMapping) error
	DeleteByProductMapping(productMappingID uint) error
}

// ProductRepository 是映射上下文所需的最小本地商品端口。
type ProductRepository interface {
	GetByID(id string) (*productdomain.Product, error)
	Update(item *productdomain.Product) error
	QuickUpdate(id string, fields map[string]interface{}) error
	CountBySlug(slug string, excludeID *string) (int64, error)
}

// SKURepository 是映射上下文所需的最小本地 SKU 端口。
type SKURepository interface {
	GetByID(id uint) (*productdomain.ProductSKU, error)
	ListByProduct(productID uint, onlyActive bool) ([]productdomain.ProductSKU, error)
	Create(item *productdomain.ProductSKU) error
	Update(item *productdomain.ProductSKU) error
}

// CategoryRepository 是映射上下文所需的分类端口。
type CategoryRepository interface {
	productdomain.CategoryAssignmentRepository
	GetBySlug(slug string) (*categorydomain.Category, error)
	GetBySlugUnscoped(slug string) (*categorydomain.Category, error)
	Restore(category *categorydomain.Category) error
}

// ImportTxProductRepository 是导入事务内的本地商品写入端口。
type ImportTxProductRepository interface {
	Create(item *productdomain.Product) error
	QuickUpdate(id string, fields map[string]interface{}) error
}

// ImportTxSKURepository 是导入事务内的本地 SKU 写入端口。
type ImportTxSKURepository interface {
	Create(item *productdomain.ProductSKU) error
}

// ImportTxMappingRepository 是导入事务内的映射写入端口。
type ImportTxMappingRepository interface {
	Create(mapping *mappingdomain.Mapping) error
}

// ImportTxSKUMappingRepository 是导入事务内的 SKU 映射写入端口。
type ImportTxSKUMappingRepository interface {
	Create(mapping *mappingdomain.SKUMapping) error
}

// ImportRepositories 是一次上游商品导入事务中绑定的全部窄端口。
type ImportRepositories struct {
	Products    ImportTxProductRepository
	SKUs        ImportTxSKURepository
	Mappings    ImportTxMappingRepository
	SKUMappings ImportTxSKUMappingRepository
}

// UnitOfWork 是映射导入的事务边界。
type UnitOfWork interface {
	WithinTransaction(fn func(ImportRepositories) error) error
}
