package service

import (
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newSyncSingleSKURepo(t *testing.T) repository.ProductSKURepository {
	t.Helper()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.ProductSKU{}); err != nil {
		t.Fatalf("auto migrate product sku failed: %v", err)
	}
	return repository.NewProductSKURepository(db)
}

func TestSyncSingleProductSKU_MultipleRowsKeepsSingleActive(t *testing.T) {
	repo := newSyncSingleSKURepo(t)
	productID := uint(2001)

	inactiveDefault := models.ProductSKU{
		ProductID:        productID,
		SKUCode:          models.DefaultSKUCode,
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		ManualStockTotal: 9,
		IsActive:         false,
		SortOrder:        0,
	}
	firstActive := models.ProductSKU{
		ProductID:        productID,
		SKUCode:          "A",
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		ManualStockTotal: 2,
		IsActive:         true,
		SortOrder:        2,
	}
	secondActive := models.ProductSKU{
		ProductID:        productID,
		SKUCode:          "B",
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		ManualStockTotal: 4,
		IsActive:         true,
		SortOrder:        1,
	}
	if err := repo.Create(&inactiveDefault); err != nil {
		t.Fatalf("create inactive default sku failed: %v", err)
	}
	inactiveDefault.IsActive = false
	if err := repo.Update(&inactiveDefault); err != nil {
		t.Fatalf("update inactive default sku failed: %v", err)
	}
	if err := repo.Create(&firstActive); err != nil {
		t.Fatalf("create first active sku failed: %v", err)
	}
	if err := repo.Create(&secondActive); err != nil {
		t.Fatalf("create second active sku failed: %v", err)
	}

	targetPrice := decimal.RequireFromString("88.88")
	if err := syncSingleProductSKU(repo, productID, targetPrice, decimal.Zero, 5, true); err != nil {
		t.Fatalf("sync single sku failed: %v", err)
	}

	skus, err := repo.ListByProduct(productID, false)
	if err != nil {
		t.Fatalf("list sku failed: %v", err)
	}

	activeCount := 0
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		activeCount++
		if sku.ID != firstActive.ID {
			t.Fatalf("expected higher sort_order active sku id=%d, got id=%d", firstActive.ID, sku.ID)
		}
		if !sku.PriceAmount.Equal(targetPrice) {
			t.Fatalf("expected price %s, got %s", targetPrice.StringFixed(2), sku.PriceAmount.String())
		}
		if sku.ManualStockTotal != 5 {
			t.Fatalf("expected manual stock total 5, got %d", sku.ManualStockTotal)
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active sku, got %d", activeCount)
	}
}

func TestSyncSingleProductSKU_NoActivePrefersDefaultCode(t *testing.T) {
	repo := newSyncSingleSKURepo(t)
	productID := uint(2002)

	inactiveA := models.ProductSKU{
		ProductID:        productID,
		SKUCode:          "A",
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		ManualStockTotal: 3,
		IsActive:         false,
		SortOrder:        1,
	}
	inactiveDefault := models.ProductSKU{
		ProductID:        productID,
		SKUCode:          models.DefaultSKUCode,
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		ManualStockTotal: 8,
		IsActive:         false,
		SortOrder:        0,
	}
	if err := repo.Create(&inactiveA); err != nil {
		t.Fatalf("create inactive sku A failed: %v", err)
	}
	inactiveA.IsActive = false
	if err := repo.Update(&inactiveA); err != nil {
		t.Fatalf("update inactive sku A failed: %v", err)
	}
	if err := repo.Create(&inactiveDefault); err != nil {
		t.Fatalf("create inactive default sku failed: %v", err)
	}
	inactiveDefault.IsActive = false
	if err := repo.Update(&inactiveDefault); err != nil {
		t.Fatalf("update inactive default sku failed: %v", err)
	}

	targetPrice := decimal.RequireFromString("19.90")
	if err := syncSingleProductSKU(repo, productID, targetPrice, decimal.Zero, 6, true); err != nil {
		t.Fatalf("sync single sku failed: %v", err)
	}

	skus, err := repo.ListByProduct(productID, false)
	if err != nil {
		t.Fatalf("list sku failed: %v", err)
	}

	activeCount := 0
	for _, sku := range skus {
		if !sku.IsActive {
			continue
		}
		activeCount++
		if sku.ID != inactiveDefault.ID {
			t.Fatalf("expected default sku id=%d to be active, got id=%d", inactiveDefault.ID, sku.ID)
		}
		if !sku.PriceAmount.Equal(targetPrice) {
			t.Fatalf("expected price %s, got %s", targetPrice.StringFixed(2), sku.PriceAmount.String())
		}
		if sku.ManualStockTotal != 6 {
			t.Fatalf("expected manual stock total 6, got %d", sku.ManualStockTotal)
		}
	}
	if activeCount != 1 {
		t.Fatalf("expected exactly one active sku, got %d", activeCount)
	}
}

func TestApplyAutoStockCounts_LegacyStockPrefersDefaultSKU(t *testing.T) {
	svc, db := newAutoStockProductService(t)
	productID := uint(3001)
	defaultSKUID := uint(101)
	otherSKUID := uint(102)

	insertCardSecrets(t, db, productID, 0, models.CardSecretStatusAvailable, 2)
	insertCardSecrets(t, db, productID, 0, models.CardSecretStatusReserved, 1)
	insertCardSecrets(t, db, productID, 0, models.CardSecretStatusUsed, 1)
	insertCardSecrets(t, db, productID, defaultSKUID, models.CardSecretStatusAvailable, 3)
	insertCardSecrets(t, db, productID, otherSKUID, models.CardSecretStatusAvailable, 4)
	counts, err := svc.cardSecretRepo.CountStockByProductIDs([]uint{productID})
	if err != nil {
		t.Fatalf("count stock by product ids failed: %v", err)
	}
	if len(counts) != 5 {
		t.Fatalf("expected 5 grouped stock rows, got %d", len(counts))
	}
	bySKUAndStatus := make(map[uint]map[string]int64)
	for _, row := range counts {
		if bySKUAndStatus[row.SKUID] == nil {
			bySKUAndStatus[row.SKUID] = make(map[string]int64)
		}
		bySKUAndStatus[row.SKUID][row.Status] = row.Total
	}
	if bySKUAndStatus[0][models.CardSecretStatusAvailable] != 2 ||
		bySKUAndStatus[0][models.CardSecretStatusReserved] != 1 ||
		bySKUAndStatus[0][models.CardSecretStatusUsed] != 1 {
		t.Fatalf("unexpected legacy sku(0) rows: %+v", bySKUAndStatus[0])
	}
	if bySKUAndStatus[defaultSKUID][models.CardSecretStatusAvailable] != 3 {
		t.Fatalf("unexpected default sku rows: %+v", bySKUAndStatus[defaultSKUID])
	}
	if bySKUAndStatus[otherSKUID][models.CardSecretStatusAvailable] != 4 {
		t.Fatalf("unexpected other sku rows: %+v", bySKUAndStatus[otherSKUID])
	}

	products := []models.Product{
		{
			ID:              productID,
			FulfillmentType: constants.FulfillmentTypeAuto,
			SKUs: []models.ProductSKU{
				{
					ID:       otherSKUID,
					SKUCode:  "B",
					IsActive: true,
				},
				{
					ID:       defaultSKUID,
					SKUCode:  models.DefaultSKUCode,
					IsActive: true,
				},
			},
		},
	}

	if err := svc.ApplyAutoStockCounts(products); err != nil {
		t.Fatalf("apply auto stock counts failed: %v", err)
	}

	got := products[0]
	if got.AutoStockAvailable != 9 {
		t.Fatalf("expected product auto available=9, got %d", got.AutoStockAvailable)
	}
	if got.AutoStockLocked != 1 {
		t.Fatalf("expected product auto locked=1, got %d", got.AutoStockLocked)
	}
	if got.AutoStockSold != 1 {
		t.Fatalf("expected product auto sold=1, got %d", got.AutoStockSold)
	}
	if got.AutoStockTotal != 10 {
		t.Fatalf("expected product auto total=10, got %d", got.AutoStockTotal)
	}

	if got.SKUs[0].AutoStockAvailable != 4 {
		t.Fatalf("expected other sku auto available=4, got %d", got.SKUs[0].AutoStockAvailable)
	}
	if got.SKUs[0].AutoStockLocked != 0 || got.SKUs[0].AutoStockSold != 0 {
		t.Fatalf("expected other sku locked/sold to remain 0, got locked=%d sold=%d", got.SKUs[0].AutoStockLocked, got.SKUs[0].AutoStockSold)
	}

	if got.SKUs[1].AutoStockAvailable != 5 {
		t.Fatalf("expected default sku auto available=5, got %d", got.SKUs[1].AutoStockAvailable)
	}
	if got.SKUs[1].AutoStockLocked != 1 {
		t.Fatalf("expected default sku auto locked=1, got %d", got.SKUs[1].AutoStockLocked)
	}
	if got.SKUs[1].AutoStockSold != 1 {
		t.Fatalf("expected default sku auto sold=1, got %d", got.SKUs[1].AutoStockSold)
	}
}

func newAutoStockProductService(t *testing.T) (*ProductService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:product_auto_stock_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.CardSecret{}); err != nil {
		t.Fatalf("auto migrate card secret failed: %v", err)
	}
	secretRepo := repository.NewCardSecretRepository(db)
	return NewProductService(nil, nil, secretRepo, nil, nil, nil, nil, nil, nil, nil), db
}

func insertCardSecrets(t *testing.T, db *gorm.DB, productID, skuID uint, status string, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		row := models.CardSecret{
			ProductID: productID,
			SKUID:     skuID,
			Secret:    fmt.Sprintf("secret-%d-%d-%s-%d", productID, skuID, status, i),
			Status:    status,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create card secret failed: %v", err)
		}
	}
}

func TestProductServiceListPublicIncludesChildProductsForParentCategory(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	parent := models.Category{
		Slug:     "games",
		NameJSON: models.JSON{"zh-CN": "games"},
	}
	child := models.Category{
		ParentID: 1,
		Slug:     "steam",
		NameJSON: models.JSON{"zh-CN": "steam"},
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent category failed: %v", err)
	}
	child.ParentID = parent.ID
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child category failed: %v", err)
	}

	parentProduct := models.Product{
		CategoryID:  parent.ID,
		Slug:        "parent-product",
		TitleJSON:   models.JSON{"zh-CN": "parent-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	childProduct := models.Product{
		CategoryID:  child.ID,
		Slug:        "child-product",
		TitleJSON:   models.JSON{"zh-CN": "child-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&parentProduct).Error; err != nil {
		t.Fatalf("create parent product failed: %v", err)
	}
	if err := db.Create(&childProduct).Error; err != nil {
		t.Fatalf("create child product failed: %v", err)
	}

	products, total, err := svc.ListPublic(strconv.FormatUint(uint64(parent.ID), 10), "", 1, 20)
	if err != nil {
		t.Fatalf("list public products failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total=2, got %d", total)
	}
	if len(products) != 2 {
		t.Fatalf("expected 2 products, got %d", len(products))
	}
}

func TestProductServiceCreateRejectsParentCategoryWithChildren(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	parent := models.Category{
		Slug:     "games",
		NameJSON: models.JSON{"zh-CN": "games"},
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent category failed: %v", err)
	}
	child := models.Category{
		ParentID: parent.ID,
		Slug:     "steam",
		NameJSON: models.JSON{"zh-CN": "steam"},
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child category failed: %v", err)
	}

	_, err := svc.Create(CreateProductInput{
		CategoryID:      parent.ID,
		Slug:            "invalid-parent-product",
		TitleJSON:       map[string]interface{}{"zh-CN": "invalid-parent-product"},
		PriceAmount:     decimal.NewFromInt(10),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		ManualStockTotal: func() *int {
			value := 1
			return &value
		}(),
	})
	if err != ErrProductCategoryInvalid {
		t.Fatalf("expected ErrProductCategoryInvalid, got %v", err)
	}
}

func TestProductServiceQuickUpdateRejectsActivationWithoutCategory(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := models.Product{
		CategoryID:      0,
		Slug:            "uncategorized-imported-product",
		TitleJSON:       models.JSON{"zh-CN": "uncategorized-imported-product"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
		IsActive:        false,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create uncategorized product failed: %v", err)
	}

	_, err := svc.QuickUpdate(strconv.FormatUint(uint64(product.ID), 10), map[string]interface{}{"is_active": true})
	if err != ErrProductCategoryInvalid {
		t.Fatalf("expected ErrProductCategoryInvalid, got %v", err)
	}

	var got models.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if got.IsActive {
		t.Fatalf("expected product to remain inactive")
	}
}

func TestProductServiceListPublicSortOrderDescending(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "sort-test",
		NameJSON: models.JSON{"zh-CN": "sort-test"},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	high := models.Product{
		CategoryID:  category.ID,
		Slug:        "high-sort-product",
		TitleJSON:   models.JSON{"zh-CN": "high"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
		SortOrder:   100,
	}
	low := models.Product{
		CategoryID:  category.ID,
		Slug:        "low-sort-product",
		TitleJSON:   models.JSON{"zh-CN": "low"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := db.Create(&high).Error; err != nil {
		t.Fatalf("create high sort product failed: %v", err)
	}
	if err := db.Create(&low).Error; err != nil {
		t.Fatalf("create low sort product failed: %v", err)
	}

	rows, total, err := svc.ListPublic("", "", 1, 20)
	if err != nil {
		t.Fatalf("list public products failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total=2, got %d", total)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 products, got %d", len(rows))
	}
	if rows[0].Slug != "high-sort-product" || rows[1].Slug != "low-sort-product" {
		t.Fatalf("expected high sort_order first, got %s then %s", rows[0].Slug, rows[1].Slug)
	}
}

func TestProductServiceListPublicSortsSKUsDescending(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "sku-sort-test",
		NameJSON: models.JSON{"zh-CN": "sku-sort-test"},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:  category.ID,
		Slug:        "sku-order-product",
		TitleJSON:   models.JSON{"zh-CN": "sku-order-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
		SortOrder:   0,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	high := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "HIGH",
		SpecValuesJSON: models.JSON{},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      100,
	}
	low := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "LOW",
		SpecValuesJSON: models.JSON{},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      1,
	}
	if err := db.Create(&high).Error; err != nil {
		t.Fatalf("create high sort sku failed: %v", err)
	}
	if err := db.Create(&low).Error; err != nil {
		t.Fatalf("create low sort sku failed: %v", err)
	}

	rows, total, err := svc.ListPublic("", "", 1, 20)
	if err != nil {
		t.Fatalf("list public products failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected exactly 1 product, total=%d len=%d", total, len(rows))
	}
	if len(rows[0].SKUs) != 2 {
		t.Fatalf("expected 2 skus, got %d", len(rows[0].SKUs))
	}
	if rows[0].SKUs[0].SKUCode != "HIGH" || rows[0].SKUs[1].SKUCode != "LOW" {
		t.Fatalf("expected high sort_order sku first, got %s then %s", rows[0].SKUs[0].SKUCode, rows[0].SKUs[1].SKUCode)
	}
}

func TestProductServiceGetAdminByIDIncludesInactiveSKUs(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := models.Product{
		CategoryID:  1,
		Slug:        "admin-all-skus-product",
		TitleJSON:   models.JSON{"zh-CN": "admin-all-skus-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	activeSKU := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "ACTIVE",
		SpecValuesJSON: models.JSON{},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      10,
	}
	inactiveSKU := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "INACTIVE",
		SpecValuesJSON: models.JSON{},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		IsActive:       false,
		SortOrder:      1,
	}
	if err := db.Create(&activeSKU).Error; err != nil {
		t.Fatalf("create active sku failed: %v", err)
	}
	if err := db.Create(&inactiveSKU).Error; err != nil {
		t.Fatalf("create inactive sku failed: %v", err)
	}
	inactiveSKU.IsActive = false
	if err := db.Save(&inactiveSKU).Error; err != nil {
		t.Fatalf("persist inactive sku failed: %v", err)
	}

	got, err := svc.GetAdminByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil {
		t.Fatalf("get admin product failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected product, got nil")
	}
	if len(got.SKUs) != 2 {
		t.Fatalf("expected 2 skus for admin detail, got %d", len(got.SKUs))
	}
	if got.SKUs[0].SKUCode != "ACTIVE" || !got.SKUs[0].IsActive {
		t.Fatalf("expected first sku to be active ACTIVE, got %+v", got.SKUs[0])
	}
	if got.SKUs[1].SKUCode != "INACTIVE" || got.SKUs[1].IsActive {
		t.Fatalf("expected second sku to be inactive INACTIVE, got %+v", got.SKUs[1])
	}
}

func TestProductServiceUpdateKeepsMappedProductFulfillmentUpstream(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "mapped-category",
		NameJSON: models.JSON{"zh-CN": "mapped-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:       category.ID,
		Slug:             "mapped-product",
		TitleJSON:        models.JSON{"zh-CN": "mapped-product"},
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeUpstream,
		ManualStockTotal: 0,
		IsMapped:         true,
		IsActive:         true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create mapped product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        models.DefaultSKUCode,
		SpecValuesJSON: models.JSON{},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create mapped product sku failed: %v", err)
	}

	updated, err := svc.Update(strconv.FormatUint(uint64(product.ID), 10), CreateProductInput{
		CategoryID:      category.ID,
		Slug:            "mapped-product-updated",
		TitleJSON:       map[string]interface{}{"zh-CN": "mapped-product-updated"},
		PriceAmount:     decimal.NewFromInt(20),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		ManualStockTotal: func() *int {
			value := 0
			return &value
		}(),
		IsActive: func() *bool {
			value := true
			return &value
		}(),
	})
	if err != nil {
		t.Fatalf("update mapped product failed: %v", err)
	}
	if updated.FulfillmentType != constants.FulfillmentTypeUpstream {
		t.Fatalf("expected mapped product fulfillment type to remain upstream, got %s", updated.FulfillmentType)
	}

	reloaded, err := svc.GetAdminByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil {
		t.Fatalf("reload mapped product failed: %v", err)
	}
	if reloaded.FulfillmentType != constants.FulfillmentTypeUpstream {
		t.Fatalf("expected persisted fulfillment type upstream, got %s", reloaded.FulfillmentType)
	}
}

func TestProductServiceCreateFiltersUnavailablePaymentChannels(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "payment-channel-category",
		NameJSON: models.JSON{"zh-CN": "payment-channel-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	activeChannel := createProductTestPaymentChannel(t, db, "Active", true, false)
	inactiveChannel := createProductTestPaymentChannel(t, db, "Inactive", false, false)
	deletedChannel := createProductTestPaymentChannel(t, db, "Deleted", true, true)

	product, err := svc.Create(CreateProductInput{
		CategoryID:        category.ID,
		Slug:              "payment-channel-create",
		TitleJSON:         map[string]interface{}{"zh-CN": "payment-channel-create"},
		PriceAmount:       decimal.NewFromInt(10),
		PurchaseType:      constants.ProductPurchaseMember,
		FulfillmentType:   constants.FulfillmentTypeAuto,
		PaymentChannelIDs: []uint{deletedChannel.ID, inactiveChannel.ID, activeChannel.ID},
		IsActive: func() *bool {
			value := true
			return &value
		}(),
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	got := DecodeChannelIDs(product.PaymentChannelIDs)
	if len(got) != 1 || got[0] != activeChannel.ID {
		t.Fatalf("expected only active payment channel %d, got %v", activeChannel.ID, got)
	}
}

func TestProductServiceUpdateFiltersUnavailablePaymentChannels(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "payment-channel-update-category",
		NameJSON: models.JSON{"zh-CN": "payment-channel-update-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	deletedChannel := createProductTestPaymentChannel(t, db, "Deleted", true, true)
	product := models.Product{
		CategoryID:        category.ID,
		Slug:              "payment-channel-update",
		TitleJSON:         models.JSON{"zh-CN": "payment-channel-update"},
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:      constants.ProductPurchaseMember,
		FulfillmentType:   constants.FulfillmentTypeAuto,
		PaymentChannelIDs: EncodeChannelIDs([]uint{deletedChannel.ID}),
		IsActive:          true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	updated, err := svc.Update(strconv.FormatUint(uint64(product.ID), 10), CreateProductInput{
		CategoryID:        category.ID,
		Slug:              product.Slug,
		TitleJSON:         map[string]interface{}{"zh-CN": "payment-channel-update"},
		PriceAmount:       decimal.NewFromInt(10),
		PurchaseType:      constants.ProductPurchaseMember,
		FulfillmentType:   constants.FulfillmentTypeAuto,
		PaymentChannelIDs: []uint{deletedChannel.ID},
		IsActive: func() *bool {
			value := true
			return &value
		}(),
	})
	if err != nil {
		t.Fatalf("update product failed: %v", err)
	}
	if got := DecodeChannelIDs(updated.PaymentChannelIDs); len(got) != 0 {
		t.Fatalf("expected stale-only payment channels to be cleared, got %v", got)
	}
}

func TestProductServiceUpdateRejectsDisablingAutoSKUWithCardSecretStock(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "auto-card-secret-category",
		NameJSON: models.JSON{"zh-CN": "auto-card-secret-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            "auto-card-secret-product",
		TitleJSON:       models.JSON{"zh-CN": "auto-card-secret-product"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	stockSKU := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "SKU-STOCK",
		SpecValuesJSON: models.JSON{"zh-CN": "有库存"},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      2,
	}
	spareSKU := models.ProductSKU{
		ProductID:      product.ID,
		SKUCode:        "SKU-SPARE",
		SpecValuesJSON: models.JSON{"zh-CN": "无库存"},
		PriceAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:       true,
		SortOrder:      1,
	}
	if err := db.Create(&stockSKU).Error; err != nil {
		t.Fatalf("create stock sku failed: %v", err)
	}
	if err := db.Create(&spareSKU).Error; err != nil {
		t.Fatalf("create spare sku failed: %v", err)
	}

	insertCardSecrets(t, db, product.ID, stockSKU.ID, models.CardSecretStatusAvailable, 1)

	_, err := svc.Update(strconv.FormatUint(uint64(product.ID), 10), CreateProductInput{
		CategoryID:      category.ID,
		Slug:            product.Slug,
		TitleJSON:       map[string]interface{}{"zh-CN": "auto-card-secret-product"},
		PriceAmount:     decimal.NewFromInt(10),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		SKUs: []ProductSKUInput{
			{
				ID:             stockSKU.ID,
				SKUCode:        stockSKU.SKUCode,
				SpecValuesJSON: map[string]interface{}{"zh-CN": "有库存"},
				PriceAmount:    decimal.NewFromInt(10),
				IsActive: func() *bool {
					value := false
					return &value
				}(),
				SortOrder: 2,
			},
			{
				ID:             spareSKU.ID,
				SKUCode:        spareSKU.SKUCode,
				SpecValuesJSON: map[string]interface{}{"zh-CN": "无库存"},
				PriceAmount:    decimal.NewFromInt(10),
				IsActive: func() *bool {
					value := true
					return &value
				}(),
				SortOrder: 1,
			},
		},
		IsActive: func() *bool {
			value := true
			return &value
		}(),
	})
	if err != ErrProductSKUHasCardSecretStock {
		t.Fatalf("update product error want %v got %v", ErrProductSKUHasCardSecretStock, err)
	}
}

func newProductServiceForTest(t *testing.T) (*ProductService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:product_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductSKU{}, &models.CardSecret{}, &models.CardSecretBatch{}, &models.MemberLevelPrice{}, &models.CartItem{}, &models.ProductMapping{}, &models.SKUMapping{}, &models.Order{}, &models.OrderItem{}, &models.PaymentChannel{}); err != nil {
		t.Fatalf("auto migrate product service tables failed: %v", err)
	}

	return NewProductService(
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
		repository.NewCardSecretRepository(db),
		repository.NewCardSecretBatchRepository(db),
		repository.NewCategoryRepository(db),
		repository.NewMemberLevelPriceRepository(db),
		repository.NewCartRepository(db),
		repository.NewProductMappingRepository(db),
		repository.NewOrderRepository(db),
		repository.NewPaymentChannelRepository(db),
	), db
}

func createProductTestPaymentChannel(t *testing.T, db *gorm.DB, name string, active bool, deleted bool) models.PaymentChannel {
	t.Helper()

	channel := models.PaymentChannel{
		Name:            name,
		ProviderType:    "official",
		ChannelType:     "wechat",
		InteractionMode: "qr",
		IsActive:        active,
	}
	if err := db.Create(&channel).Error; err != nil {
		t.Fatalf("create payment channel failed: %v", err)
	}
	if !active {
		if err := db.Model(&channel).Update("is_active", false).Error; err != nil {
			t.Fatalf("disable payment channel failed: %v", err)
		}
		channel.IsActive = false
	}
	if deleted {
		if err := db.Delete(&channel).Error; err != nil {
			t.Fatalf("delete payment channel failed: %v", err)
		}
	}
	return channel
}

func TestProductServiceDeleteCascade(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	// 创建分类
	cat := models.Category{Slug: "test-cat", NameJSON: models.JSON{"zh-CN": "test"}}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	// 创建商品
	product := models.Product{
		CategoryID:      cat.ID,
		Slug:            "test-product",
		TitleJSON:       models.JSON{"zh-CN": "test-product"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}
	productID := strconv.FormatUint(uint64(product.ID), 10)

	// 创建关联 SKU
	sku := models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "DEFAULT",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku: %v", err)
	}

	// 创建会员等级价格
	mlp := models.MemberLevelPrice{
		ProductID:     product.ID,
		SKUID:         sku.ID,
		MemberLevelID: 1,
		PriceAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(8)),
	}
	if err := db.Create(&mlp).Error; err != nil {
		t.Fatalf("create member level price: %v", err)
	}

	// 创建购物车项
	cart := models.CartItem{
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
	pm := models.ProductMapping{
		ConnectionID:      1,
		LocalProductID:    product.ID,
		UpstreamProductID: 100,
	}
	if err := db.Create(&pm).Error; err != nil {
		t.Fatalf("create product mapping: %v", err)
	}

	// 创建 SKU 映射
	sm := models.SKUMapping{
		ProductMappingID: pm.ID,
		LocalSKUID:       sku.ID,
		UpstreamSKUID:    200,
	}
	if err := db.Create(&sm).Error; err != nil {
		t.Fatalf("create sku mapping: %v", err)
	}

	// 执行删除
	if err := svc.Delete(productID); err != nil {
		t.Fatalf("delete product: %v", err)
	}

	// 验证所有关联数据已被软删除
	var skuCount int64
	db.Model(&models.ProductSKU{}).Where("product_id = ?", product.ID).Count(&skuCount)
	if skuCount != 0 {
		t.Errorf("expected 0 SKUs after delete, got %d", skuCount)
	}

	var mlpCount int64
	db.Model(&models.MemberLevelPrice{}).Where("product_id = ?", product.ID).Count(&mlpCount)
	if mlpCount != 0 {
		t.Errorf("expected 0 member level prices after delete, got %d", mlpCount)
	}

	var cartCount int64
	db.Model(&models.CartItem{}).Where("product_id = ?", product.ID).Count(&cartCount)
	if cartCount != 0 {
		t.Errorf("expected 0 cart items after delete, got %d", cartCount)
	}

	var pmCount int64
	db.Model(&models.ProductMapping{}).Where("local_product_id = ?", product.ID).Count(&pmCount)
	if pmCount != 0 {
		t.Errorf("expected 0 product mappings after delete, got %d", pmCount)
	}

	var smCount int64
	db.Model(&models.SKUMapping{}).Where("product_mapping_id = ?", pm.ID).Count(&smCount)
	if smCount != 0 {
		t.Errorf("expected 0 SKU mappings after delete, got %d", smCount)
	}

	// 验证商品本身已被软删除
	var productCount int64
	db.Model(&models.Product{}).Where("id = ?", product.ID).Count(&productCount)
	if productCount != 0 {
		t.Errorf("expected product to be soft-deleted, but still found %d", productCount)
	}
}

func TestProductServiceCreateRejectsInvalidPurchaseLimits(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	cat := models.Category{Slug: "test-purchase-limit", NameJSON: models.JSON{"zh-CN": "test"}}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	intPtr := func(v int) *int { return &v }

	_, err := svc.Create(CreateProductInput{
		CategoryID:          cat.ID,
		Slug:                "invalid-limit-product",
		TitleJSON:           map[string]interface{}{"zh-CN": "invalid-limit-product"},
		PriceAmount:         decimal.NewFromInt(10),
		PurchaseType:        constants.ProductPurchaseMember,
		FulfillmentType:     constants.FulfillmentTypeManual,
		ManualStockTotal:    intPtr(1),
		MinPurchaseQuantity: intPtr(10),
		MaxPurchaseQuantity: intPtr(5),
	})
	if !errors.Is(err, ErrProductPurchaseLimitInvalid) {
		t.Fatalf("expected ErrProductPurchaseLimitInvalid, got %v", err)
	}
}

func TestProductServiceUpdateRejectsInvalidPurchaseLimits(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	cat := models.Category{Slug: "test-purchase-limit-update", NameJSON: models.JSON{"zh-CN": "test"}}
	if err := db.Create(&cat).Error; err != nil {
		t.Fatalf("create category: %v", err)
	}

	intPtr := func(v int) *int { return &v }

	created, err := svc.Create(CreateProductInput{
		CategoryID:          cat.ID,
		Slug:                "valid-limit-product",
		TitleJSON:           map[string]interface{}{"zh-CN": "valid"},
		PriceAmount:         decimal.NewFromInt(10),
		PurchaseType:        constants.ProductPurchaseMember,
		FulfillmentType:     constants.FulfillmentTypeManual,
		ManualStockTotal:    intPtr(1),
		MinPurchaseQuantity: intPtr(2),
		MaxPurchaseQuantity: intPtr(5),
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	_, err = svc.Update(strconv.FormatUint(uint64(created.ID), 10), CreateProductInput{
		CategoryID:          cat.ID,
		Slug:                "valid-limit-product",
		TitleJSON:           map[string]interface{}{"zh-CN": "valid"},
		PriceAmount:         decimal.NewFromInt(10),
		PurchaseType:        constants.ProductPurchaseMember,
		FulfillmentType:     constants.FulfillmentTypeManual,
		ManualStockTotal:    intPtr(1),
		MaxPurchaseQuantity: intPtr(1), // 已存在 min=2，新设 max=1 应触发校验
	})
	if !errors.Is(err, ErrProductPurchaseLimitInvalid) {
		t.Fatalf("expected ErrProductPurchaseLimitInvalid on update, got %v", err)
	}
}

// TestProductServiceUpdateWholesalePricesOptionalSemantics 验证批发价的可选更新语义：
// Update 省略 wholesale_prices（nil）时保留原配置；传入空切片时显式清空。
func TestProductServiceUpdateWholesalePricesOptionalSemantics(t *testing.T) {
	svc, db := newProductServiceForTest(t)
	boolPtr := func(v bool) *bool { return &v }

	category := models.Category{
		Slug:     "wholesale-update-category",
		NameJSON: models.JSON{"zh-CN": "wholesale-update-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	created, err := svc.Create(CreateProductInput{
		CategoryID:      category.ID,
		Slug:            "wholesale-update",
		TitleJSON:       map[string]interface{}{"zh-CN": "wholesale-update"},
		PriceAmount:     decimal.NewFromInt(100),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		WholesalePrices: &[]WholesalePriceInput{
			{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
		},
		IsActive: boolPtr(true),
	})
	if err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	if len(created.WholesalePrices) != 1 {
		t.Fatalf("expected 1 wholesale tier on create, got %+v", created.WholesalePrices)
	}

	idStr := strconv.FormatUint(uint64(created.ID), 10)
	baseUpdate := func() CreateProductInput {
		return CreateProductInput{
			CategoryID:      category.ID,
			Slug:            created.Slug,
			TitleJSON:       map[string]interface{}{"zh-CN": "wholesale-update"},
			PriceAmount:     decimal.NewFromInt(100),
			PurchaseType:    constants.ProductPurchaseMember,
			FulfillmentType: constants.FulfillmentTypeAuto,
			IsActive:        boolPtr(true),
		}
	}

	// 省略字段（nil）：应保留原批发价。
	keep := baseUpdate()
	keep.WholesalePrices = nil
	updated, err := svc.Update(idStr, keep)
	if err != nil {
		t.Fatalf("update without wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 1 || updated.WholesalePrices[0].UnitPrice.String() != "80.00" {
		t.Fatalf("expected wholesale prices kept when omitted, got %+v", updated.WholesalePrices)
	}

	// 传入空切片：显式清空。
	clear := baseUpdate()
	clear.WholesalePrices = &[]WholesalePriceInput{}
	cleared, err := svc.Update(idStr, clear)
	if err != nil {
		t.Fatalf("update with empty wholesale prices failed: %v", err)
	}
	if len(cleared.WholesalePrices) != 0 {
		t.Fatalf("expected wholesale prices cleared, got %+v", cleared.WholesalePrices)
	}
}

func TestProductServiceUpdateWholesalePricesOnlyTouchesWholesaleField(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	category := models.Category{
		Slug:     "wholesale-narrow-category",
		NameJSON: models.JSON{"zh-CN": "wholesale-narrow-category"},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:       category.ID,
		Slug:             "wholesale-narrow-product",
		TitleJSON:        models.JSON{"zh-CN": "原商品名"},
		PriceAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		CostPriceAmount:  models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 8,
		IsActive:         true,
		SortOrder:        9,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	updated, err := svc.UpdateWholesalePrices(strconv.FormatUint(uint64(product.ID), 10), []WholesalePriceInput{
		{MinQuantity: 10, UnitPrice: decimal.RequireFromString("70.00")},
		{MinQuantity: 5, UnitPrice: decimal.RequireFromString("80.00")},
	})
	if err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 2 {
		t.Fatalf("expected 2 wholesale tiers, got %+v", updated.WholesalePrices)
	}
	if updated.WholesalePrices[0].MinQuantity != 5 || updated.WholesalePrices[0].UnitPrice.String() != "80.00" {
		t.Fatalf("expected first tier sorted as min=5 price=80.00, got %+v", updated.WholesalePrices[0])
	}
	if updated.WholesalePrices[1].MinQuantity != 10 || updated.WholesalePrices[1].UnitPrice.String() != "70.00" {
		t.Fatalf("expected second tier sorted as min=10 price=70.00, got %+v", updated.WholesalePrices[1])
	}

	var got models.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if got.Slug != product.Slug || got.CategoryID != product.CategoryID || got.ManualStockTotal != product.ManualStockTotal || got.SortOrder != product.SortOrder || got.IsActive != product.IsActive {
		t.Fatalf("non-wholesale fields changed unexpectedly: got=%+v product=%+v", got, product)
	}
	if getTitle := got.TitleJSON["zh-CN"]; getTitle != "原商品名" {
		t.Fatalf("expected title to stay unchanged, got %v", getTitle)
	}
}

func TestProductServiceUpdateWholesalePricesClearsTiers(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := models.Product{
		CategoryID:  1,
		Slug:        "wholesale-clear-product",
		TitleJSON:   models.JSON{"zh-CN": "wholesale-clear-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
		},
		IsActive: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	updated, err := svc.UpdateWholesalePrices(strconv.FormatUint(uint64(product.ID), 10), []WholesalePriceInput{})
	if err != nil {
		t.Fatalf("clear wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 0 {
		t.Fatalf("expected wholesale prices cleared, got %+v", updated.WholesalePrices)
	}
}

func TestProductServiceUpdateWholesalePricesRejectsInvalidInputs(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := models.Product{
		CategoryID:  1,
		Slug:        "wholesale-invalid-product",
		TitleJSON:   models.JSON{"zh-CN": "wholesale-invalid-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: models.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
		},
		IsActive: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	cases := []struct {
		name   string
		inputs []WholesalePriceInput
	}{
		{name: "zero quantity", inputs: []WholesalePriceInput{{MinQuantity: 0, UnitPrice: decimal.NewFromInt(80)}}},
		{name: "zero price", inputs: []WholesalePriceInput{{MinQuantity: 5, UnitPrice: decimal.Zero}}},
		{name: "duplicate quantity", inputs: []WholesalePriceInput{
			{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
			{MinQuantity: 5, UnitPrice: decimal.NewFromInt(70)},
		}},
	}

	idStr := strconv.FormatUint(uint64(product.ID), 10)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.UpdateWholesalePrices(idStr, tc.inputs)
			if !errors.Is(err, ErrWholesalePriceInvalid) {
				t.Fatalf("expected ErrWholesalePriceInvalid, got %v", err)
			}
			var got models.Product
			if err := db.First(&got, product.ID).Error; err != nil {
				t.Fatalf("reload product failed: %v", err)
			}
			if len(got.WholesalePrices) != 1 || got.WholesalePrices[0].UnitPrice.String() != "80.00" {
				t.Fatalf("expected existing wholesale prices preserved after invalid update, got %+v", got.WholesalePrices)
			}
		})
	}
}

func TestProductServiceUpdateWholesalePricesValidatesSKUBelonging(t *testing.T) {
	svc, db := newProductServiceForTest(t)

	product := models.Product{
		CategoryID:  1,
		Slug:        "wholesale-sku-owner-product",
		TitleJSON:   models.JSON{"zh-CN": "wholesale-sku-owner-product"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	skuA := models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "SKU-A",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := db.Create(&skuA).Error; err != nil {
		t.Fatalf("create sku a failed: %v", err)
	}

	otherProduct := models.Product{
		CategoryID:  1,
		Slug:        "wholesale-sku-owner-other",
		TitleJSON:   models.JSON{"zh-CN": "wholesale-sku-owner-other"},
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
	}
	if err := db.Create(&otherProduct).Error; err != nil {
		t.Fatalf("create other product failed: %v", err)
	}
	foreignSKU := models.ProductSKU{
		ProductID:   otherProduct.ID,
		SKUCode:     "SKU-X",
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := db.Create(&foreignSKU).Error; err != nil {
		t.Fatalf("create foreign sku failed: %v", err)
	}

	idStr := strconv.FormatUint(uint64(product.ID), 10)
	if _, err := svc.UpdateWholesalePrices(idStr, []WholesalePriceInput{
		{SKUID: foreignSKU.ID, MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	}); !errors.Is(err, ErrWholesalePriceInvalid) {
		t.Fatalf("expected foreign sku_id to be rejected, got %v", err)
	}
	if _, err := svc.UpdateWholesalePrices(idStr, []WholesalePriceInput{
		{SKUID: skuA.ID, SKUCode: "SKU-X", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	}); !errors.Is(err, ErrWholesalePriceInvalid) {
		t.Fatalf("expected sku_id/sku_code mismatch to be rejected, got %v", err)
	}

	updated, err := svc.UpdateWholesalePrices(idStr, []WholesalePriceInput{
		{SKUCode: "SKU-A", MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	})
	if err != nil {
		t.Fatalf("update wholesale prices failed: %v", err)
	}
	if len(updated.WholesalePrices) != 1 {
		t.Fatalf("expected one tier, got %+v", updated.WholesalePrices)
	}
	if updated.WholesalePrices[0].SKUID != skuA.ID || updated.WholesalePrices[0].SKUCode != "SKU-A" {
		t.Fatalf("expected SKU code to be canonicalized, got %+v", updated.WholesalePrices[0])
	}
}

func TestProductServiceUpdateWholesalePricesReturnsNotFound(t *testing.T) {
	svc, _ := newProductServiceForTest(t)

	_, err := svc.UpdateWholesalePrices("999999", []WholesalePriceInput{
		{MinQuantity: 5, UnitPrice: decimal.NewFromInt(80)},
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
