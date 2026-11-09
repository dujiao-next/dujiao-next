package catalogproductbootstrap

import (
	"errors"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"

	cardsecretcontract "github.com/dujiao-next/internal/modules/cardsecret/contract"
	categorycontract "github.com/dujiao-next/internal/modules/catalog/category/contract"

	cartgormstore "github.com/dujiao-next/internal/modules/cart/infrastructure/gormstore"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	productapplication "github.com/dujiao-next/internal/modules/catalog/product/application"
	productadmin "github.com/dujiao-next/internal/modules/catalog/product/application/admin"
	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"

	"gorm.io/gorm"
)

type memberLevelPriceCleaner interface {
	DeleteByProductInTx(tx *gorm.DB, productID uint) error
}

// ProductStore 是装配层需要的 Product 持久化与事务能力。
type ProductStore interface {
	productcontract.Repository
	Transaction(fn func(tx *gorm.DB) error) error
	BindTx(tx *gorm.DB) productcontract.Repository
}

// SKUStore 是装配层需要的 SKU 持久化与事务绑定能力。
type SKUStore interface {
	productcontract.SKURepository
	BindTx(tx *gorm.DB) productcontract.SKURepository
}

// CardSecretStore 是商品事务所需的卡密库存端口与事务绑定能力。
type CardSecretStore interface {
	cardsecretcontract.Repository
	BindTx(tx *gorm.DB) cardsecretcontract.Repository
}

// CardSecretBatchStore 是商品级联删除所需的卡密批次端口与事务绑定能力。
type CardSecretBatchStore interface {
	cardsecretcontract.BatchRepository
	BindTx(tx *gorm.DB) cardsecretcontract.BatchRepository
}

// MappingStore 是商品级联删除所需的映射持久化与事务能力。
type MappingStore interface {
	mappingcontract.MappingRepository
	BindTx(tx *gorm.DB) mappingcontract.MappingRepository
}

type OrderReader interface {
	CountOrderItemsByProduct(productID uint) (int64, error)
}

// Dependencies 是 Product 三组应用用例的装配依赖。
type Dependencies struct {
	Products          ProductStore
	SKUs              SKUStore
	CardSecrets       CardSecretStore
	CardSecretBatches CardSecretBatchStore
	Categories        categorycontract.Repository
	MemberLevelPrices memberLevelPriceCleaner
	UserPrices        memberLevelPriceCleaner
	Carts             *cartgormstore.Store
	ProductMappings   MappingStore
	Orders            OrderReader
	PaymentChannels   paymentcontract.ChannelStore
}

// Services 是 Product 查询、管理和写入用例的显式集合。
type Services struct {
	Read  *productapplication.Service
	Admin *productadmin.AdminService
	Write *productwrite.WriteService
}

// productWriteUnitOfWork 将 Product Application 所需端口绑定到同一事务。
type productWriteUnitOfWork struct {
	products    ProductStore
	skus        SKUStore
	cardSecrets CardSecretStore
}

func newProductWriteUnitOfWork(
	products ProductStore,
	skus SKUStore,
	cardSecrets CardSecretStore,
) productwrite.UnitOfWork {
	return &productWriteUnitOfWork{
		products:    products,
		skus:        skus,
		cardSecrets: cardSecrets,
	}
}

func (unit *productWriteUnitOfWork) WithinTransaction(fn func(repositories productwrite.TransactionRepositories) error) error {
	if fn == nil {
		return nil
	}
	if unit == nil || unit.products == nil {
		return errors.New("product transaction repository is nil")
	}
	return unit.products.Transaction(func(tx *gorm.DB) error {
		var skus productwrite.SKURepository
		if unit.skus != nil {
			skus = unit.skus.BindTx(tx)
		}
		var cardSecrets productwrite.CardSecretStockRepository
		if unit.cardSecrets != nil {
			cardSecrets = unit.cardSecrets.BindTx(tx)
		}
		return fn(productwrite.TransactionRepositories{
			Products:    unit.products.BindTx(tx),
			SKUs:        skus,
			CardSecrets: cardSecrets,
		})
	})
}

// productAdminUnitOfWork 将商品级联删除涉及的端口绑定到同一事务。
type productAdminUnitOfWork struct {
	products          ProductStore
	productSKUs       SKUStore
	cardSecrets       CardSecretStore
	cardSecretBatches CardSecretBatchStore
	memberLevelPrices memberLevelPriceCleaner
	userPrices        memberLevelPriceCleaner
	carts             *cartgormstore.Store
	productMappings   MappingStore
}

func newProductAdminUnitOfWork(
	products ProductStore,
	productSKUs SKUStore,
	cardSecrets CardSecretStore,
	cardSecretBatches CardSecretBatchStore,
	memberLevelPrices memberLevelPriceCleaner,
	userPrices memberLevelPriceCleaner,
	carts *cartgormstore.Store,
	productMappings MappingStore,
) productadmin.UnitOfWork {
	return &productAdminUnitOfWork{
		products:          products,
		productSKUs:       productSKUs,
		cardSecrets:       cardSecrets,
		cardSecretBatches: cardSecretBatches,
		memberLevelPrices: memberLevelPrices,
		userPrices:        userPrices,
		carts:             carts,
		productMappings:   productMappings,
	}
}

func (unit *productAdminUnitOfWork) WithinTransaction(fn func(productadmin.DeleteRepositories) error) error {
	if fn == nil {
		return nil
	}
	if unit == nil || unit.products == nil {
		return errors.New("product transaction repository is nil")
	}
	return unit.products.Transaction(func(tx *gorm.DB) error {
		return fn(productadmin.DeleteRepositories{
			Products:          unit.products.BindTx(tx),
			CardSecrets:       unit.cardSecrets.BindTx(tx),
			CardSecretBatches: unit.cardSecretBatches.BindTx(tx),
			SKUs:              unit.productSKUs.BindTx(tx),
			MemberLevelPrices: priceDeleteAdapter{tx: tx, cleaner: unit.memberLevelPrices},
			UserPrices:        priceDeleteAdapter{tx: tx, cleaner: unit.userPrices},
			Carts:             unit.carts.WithTx(tx),
			ProductMappings:   bindMappingDeleteTx(unit.productMappings, tx),
		})
	})
}

func bindMappingDeleteTx(repo MappingStore, tx *gorm.DB) productadmin.ProductMappingDeleteRepository {
	if repo == nil {
		return nil
	}
	return repo.BindTx(tx)
}

// priceDeleteAdapter 把「事务内删除某商品的价格行」适配成商品级联删除所需的窄端口。
// 会员等级价与用户专属价的清理签名完全一致，共用这一个适配器。
type priceDeleteAdapter struct {
	tx      *gorm.DB
	cleaner memberLevelPriceCleaner
}

func (adapter priceDeleteAdapter) DeleteByProduct(productID uint) error {
	return adapter.cleaner.DeleteByProductInTx(adapter.tx, productID)
}

// New 显式装配 Product 查询、管理和写入用例。
func New(dependencies Dependencies) Services {
	read := productapplication.NewService(productapplication.Options{
		Products:   dependencies.Products,
		Categories: dependencies.Categories,
		Stock:      dependencies.CardSecrets,
	})
	admin := productadmin.NewAdminService(productadmin.Options{
		Products:     dependencies.Products,
		Categories:   dependencies.Categories,
		CardSecrets:  dependencies.CardSecrets,
		Orders:       dependencies.Orders,
		Transactions: newProductAdminUnitOfWork(dependencies.Products, dependencies.SKUs, dependencies.CardSecrets, dependencies.CardSecretBatches, dependencies.MemberLevelPrices, dependencies.UserPrices, dependencies.Carts, dependencies.ProductMappings),
	})
	write := productwrite.NewWriteService(productwrite.Options{
		Products:        dependencies.Products,
		SKUs:            dependencies.SKUs,
		Categories:      dependencies.Categories,
		PaymentChannels: dependencies.PaymentChannels,
		Transactions:    newProductWriteUnitOfWork(dependencies.Products, dependencies.SKUs, dependencies.CardSecrets),
	})
	return Services{Read: read, Admin: admin, Write: write}
}
