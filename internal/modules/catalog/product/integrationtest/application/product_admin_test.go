package integrationtest

import (
	"errors"
	"strconv"
	"testing"

	paymentgormstore "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore"

	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	catalogproductbootstrap "github.com/dujiao-next/internal/bootstrap/catalogproduct"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	"github.com/dujiao-next/internal/constants"
	cartdomain "github.com/dujiao-next/internal/modules/cart/domain"
	cartgormstore "github.com/dujiao-next/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestProductServiceQuickUpdateAllowsMappedActivationWithoutCategory(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := productdomain.Product{
		CategoryID:      0,
		Slug:            "uncategorized-imported-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "uncategorized-imported-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
		IsActive:        false,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create uncategorized product failed: %v", err)
	}

	_, err := svc.Admin.QuickUpdate(strconv.FormatUint(uint64(product.ID), 10), map[string]interface{}{"is_active": true})
	if err != nil {
		t.Fatalf("expected mapped product activation to succeed, got %v", err)
	}

	var got productdomain.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if !got.IsActive {
		t.Fatalf("expected mapped product to become active")
	}
}

func TestProductServiceQuickUpdateRejectsManualActivationWithoutCategory(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := productdomain.Product{
		CategoryID:      0,
		Slug:            "uncategorized-manual-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "uncategorized-manual-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        false,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create uncategorized manual product failed: %v", err)
	}

	_, err := svc.Admin.QuickUpdate(strconv.FormatUint(uint64(product.ID), 10), map[string]interface{}{"is_active": true})
	if err != productcontract.ErrProductCategoryInvalid {
		t.Fatalf("expected productcontract.ErrProductCategoryInvalid, got %v", err)
	}
}

func TestProductServiceDeleteCascade(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	// 创建分类
	cat := categorydomain.Category{Slug: "test-cat", NameJSON: jsonmap.JSON{"zh-CN": "test"}}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	// 创建商品
	product := productdomain.Product{
		CategoryID:      cat.ID,
		Slug:            "test-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "test-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	productID := strconv.FormatUint(uint64(product.ID), 10)

	// 创建关联 SKU
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "DEFAULT",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}

	// 创建会员等级价格
	mlp := memberleveldomain.MemberLevelPrice{
		ProductID:     product.ID,
		SKUID:         sku.ID,
		MemberLevelID: 1,
		PriceAmount:   money.FromDecimal(decimal.NewFromInt(8)),
	}
	if err := db.Create(&mlp).Error; err != nil {
		t.Fatalf("create member level price: %v", err)
	}

	// 创建购物车项
	cart := cartdomain.Item{
		UserID:          1,
		ProductID:       product.ID,
		SKUID:           sku.ID,
		Quantity:        1,
		FulfillmentType: constants.FulfillmentTypeManual,
	}
	if err := db.Create(&cart).Error; err != nil {
		t.Fatalf("create cart item: %v", err)
	}

	// 创建商品映射
	pm := mappingdomain.Mapping{
		ConnectionID:      1,
		LocalProductID:    product.ID,
		UpstreamProductID: 100,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create product mapping: %v", err)
	}

	// 创建 SKU 映射
	sm := mappingdomain.SKUMapping{
		ProductMappingID: pm.ID,
		LocalSKUID:       sku.ID,
		UpstreamSKUID:    200,
	}
	if err := db.Create(&sm).Error; err != nil {
		t.Fatalf("create sku mapping: %v", err)
	}

	// 执行删除
	if err := svc.Admin.Delete(productID); err != nil {
		t.Fatalf("delete product: %v", err)
	}

	// 验证所有关联数据已被软删除
	var skuCount int64
	db.Model(&productdomain.ProductSKU{}).Where("product_id = ? AND deleted_at IS NULL", product.ID).Count(&skuCount)
	if skuCount != 0 {
		t.Errorf("expected 0 SKUs after delete, got %d", skuCount)
	}

	var mlpCount int64
	db.Model(&memberleveldomain.MemberLevelPrice{}).
		Where("product_id = ? AND deleted_at IS NULL", product.ID).
		Count(&mlpCount)
	if mlpCount != 0 {
		t.Errorf("expected 0 member level prices after delete, got %d", mlpCount)
	}

	var cartCount int64
	db.Model(&cartdomain.Item{}).Where("product_id = ? AND deleted_at IS NULL", product.ID).Count(&cartCount)
	if cartCount != 0 {
		t.Errorf("expected 0 cart items after delete, got %d", cartCount)
	}

	var pmCount int64
	db.Model(&mappingdomain.Mapping{}).Where("local_product_id = ? AND deleted_at IS NULL", product.ID).Count(&pmCount)
	if pmCount != 0 {
		t.Errorf("expected 0 product mappings after delete, got %d", pmCount)
	}

	var smCount int64
	db.Model(&mappingdomain.SKUMapping{}).Where("product_mapping_id = ? AND deleted_at IS NULL", pm.ID).Count(&smCount)
	if smCount != 0 {
		t.Errorf("expected 0 SKU mappings after delete, got %d", smCount)
	}

	// 验证商品本身已被软删除
	var productCount int64
	db.Model(&productdomain.Product{}).Where("id = ? AND deleted_at IS NULL", product.ID).Count(&productCount)
	if productCount != 0 {
		t.Errorf("expected product to be soft-deleted, but still found %d", productCount)
	}
	var deletedProduct productdomain.Product
	if err := db.Where("id = ? AND deleted_at IS NOT NULL", product.ID).First(&deletedProduct).Error; err != nil {
		t.Fatalf("expected soft-deleted product row to remain persisted: %v", err)
	}
}

func TestProductServiceDeleteRollsBackCascadeWhenProductDeleteFails(t *testing.T) {
	_, db := newProductServiceForTest(t)
	deleteFailure := errors.New("delete product failed")
	baseProductStore := productgormstore.NewProductStore(db)
	productRepo := &failingProductDeleteRepository{
		Repository:  baseProductStore,
		transaction: baseProductStore.Transaction,
		binder:      baseProductStore.BindTx,
		err:         deleteFailure,
	}
	service := catalogproductbootstrap.New(catalogproductbootstrap.Dependencies{
		Products:          productRepo,
		SKUs:              productgormstore.NewSKUStore(db),
		CardSecrets:       cardsecretgormstore.New(db),
		CardSecretBatches: cardsecretgormstore.NewBatch(db),
		Categories:        categorygormstore.NewCategoryStore(db),
		MemberLevelPrices: memberlevelgormstore.NewPriceStore(db),
		Carts:             cartgormstore.New(db),
		ProductMappings:   mappinggormstore.NewMappingStore(db),
		Orders:            ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		PaymentChannels:   paymentgormstore.NewChannelStore(db),
	})

	product := productdomain.Product{
		CategoryID:      1,
		Slug:            "rollback-product-delete",
		TitleJSON:       jsonmap.JSON{"zh-CN": "rollback-product-delete"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	cart := cartdomain.Item{
		UserID:          1,
		ProductID:       product.ID,
		SKUID:           sku.ID,
		Quantity:        1,
		FulfillmentType: constants.FulfillmentTypeManual,
	}
	if err := db.Create(&cart).Error; err != nil {
		t.Fatalf("create cart item failed: %v", err)
	}
	mapping := mappingdomain.Mapping{
		ConnectionID:      1,
		LocalProductID:    product.ID,
		UpstreamProductID: 99,
	}
	if err := db.Create(&mapping).Error; err != nil {
		t.Fatalf("create product mapping failed: %v", err)
	}

	err := service.Admin.Delete(strconv.FormatUint(uint64(product.ID), 10))
	if !errors.Is(err, deleteFailure) {
		t.Fatalf("expected injected delete failure, got %v", err)
	}

	assertProductRelationCount(t, db, &productdomain.Product{}, "id = ?", product.ID, 1)
	assertProductRelationCount(t, db, &productdomain.ProductSKU{}, "product_id = ?", product.ID, 1)
	assertProductRelationCount(t, db, &cartdomain.Item{}, "product_id = ? AND deleted_at IS NULL", product.ID, 1)
	assertProductRelationCount(t, db, &mappingdomain.Mapping{}, "local_product_id = ?", product.ID, 1)
}

type failingProductDeleteRepository struct {
	productcontract.Repository
	transaction func(func(*gorm.DB) error) error
	binder      func(*gorm.DB) productcontract.Repository
	err         error
}

func (repo *failingProductDeleteRepository) Delete(string) error {
	return repo.err
}

func (repo *failingProductDeleteRepository) Transaction(fn func(*gorm.DB) error) error {
	return repo.transaction(fn)
}

func (repo *failingProductDeleteRepository) BindTx(tx *gorm.DB) productcontract.Repository {
	return &failingProductDeleteRepository{
		Repository: repo.binder(tx),
		err:        repo.err,
	}
}

func assertProductRelationCount(t *testing.T, db *gorm.DB, model interface{}, query string, arg interface{}, expected int64) {
	t.Helper()
	var count int64
	if err := db.Model(model).Where(query, arg).Count(&count).Error; err != nil {
		t.Fatalf("count product relation failed: %v", err)
	}
	if count != expected {
		t.Fatalf("expected relation count %d after rollback, got %d", expected, count)
	}
}
