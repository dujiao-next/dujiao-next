package repository

import (
	"strconv"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupProductRepositoryTest(t *testing.T) (*GormProductRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductSKU{}); err != nil {
		t.Fatalf("migrate category/product/sku failed: %v", err)
	}
	return NewProductRepository(db), db
}

func createManualProduct(t *testing.T, repo *GormProductRepository, slug string, total int, locked int, sold int) *models.Product {
	t.Helper()
	product := &models.Product{
		CategoryID:        1,
		Slug:              slug,
		TitleJSON:         models.JSON{"zh-CN": "测试商品"},
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		PurchaseType:      constants.ProductPurchaseMember,
		FulfillmentType:   constants.FulfillmentTypeManual,
		ManualStockTotal:  total,
		ManualStockLocked: locked,
		ManualStockSold:   sold,
		IsActive:          true,
	}
	if err := repo.Create(product); err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	return product
}

func createManualSKU(t *testing.T, db *gorm.DB, productID uint, code string, total int, locked int, sold int, isActive bool) *models.ProductSKU {
	t.Helper()
	sku := &models.ProductSKU{
		ProductID:         productID,
		SKUCode:           code,
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		ManualStockTotal:  total,
		ManualStockLocked: locked,
		ManualStockSold:   sold,
		IsActive:          true,
		SortOrder:         0,
	}
	if err := db.Create(sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	if !isActive {
		sku.IsActive = false
		if err := db.Save(sku).Error; err != nil {
			t.Fatalf("update inactive sku failed: %v", err)
		}
	}
	return sku
}

func TestManualStockReserveReleaseConsumeLifecycle(t *testing.T) {
	repo, db := setupProductRepositoryTest(t)
	product := createManualProduct(t, repo, "manual-stock-lifecycle", 10, 0, 0)

	affected, err := repo.ReserveManualStock(product.ID, 3)
	if err != nil {
		t.Fatalf("reserve stock failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("reserve affected want 1 got %d", affected)
	}

	affected, err = repo.ConsumeManualStock(product.ID, 2)
	if err != nil {
		t.Fatalf("consume stock failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("consume affected want 1 got %d", affected)
	}

	affected, err = repo.ReleaseManualStock(product.ID, 1)
	if err != nil {
		t.Fatalf("release stock failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("release affected want 1 got %d", affected)
	}

	var got models.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if got.ManualStockTotal != 8 {
		t.Fatalf("total want 8 got %d", got.ManualStockTotal)
	}
	if got.ManualStockLocked != 0 {
		t.Fatalf("locked want 0 got %d", got.ManualStockLocked)
	}
	if got.ManualStockSold != 2 {
		t.Fatalf("sold want 2 got %d", got.ManualStockSold)
	}

	affected, err = repo.ReserveManualStock(product.ID, 9)
	if err != nil {
		t.Fatalf("reserve over available failed: %v", err)
	}
	if affected != 0 {
		t.Fatalf("reserve over available affected want 0 got %d", affected)
	}

	affected, err = repo.ReserveManualStock(product.ID, 8)
	if err != nil {
		t.Fatalf("reserve exact available failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("reserve exact available affected want 1 got %d", affected)
	}
}

func TestManualStockConsumeWithLegacyUnreservedOrder(t *testing.T) {
	repo, db := setupProductRepositoryTest(t)
	product := createManualProduct(t, repo, "manual-stock-legacy", 5, 0, 1)

	affected, err := repo.ConsumeManualStock(product.ID, 2)
	if err != nil {
		t.Fatalf("consume stock failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("consume affected want 1 got %d", affected)
	}

	var got models.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if got.ManualStockTotal != 3 {
		t.Fatalf("total want 3 got %d", got.ManualStockTotal)
	}
	if got.ManualStockLocked != 0 {
		t.Fatalf("locked want 0 got %d", got.ManualStockLocked)
	}
	if got.ManualStockSold != 3 {
		t.Fatalf("sold want 3 got %d", got.ManualStockSold)
	}
}

func TestManualStockUnlimitedDoesNotReserve(t *testing.T) {
	repo, _ := setupProductRepositoryTest(t)
	product := createManualProduct(t, repo, "manual-stock-unlimited", constants.ManualStockUnlimited, 0, 0)

	affected, err := repo.ReserveManualStock(product.ID, 1)
	if err != nil {
		t.Fatalf("reserve unlimited stock failed: %v", err)
	}
	if affected != 0 {
		t.Fatalf("reserve unlimited affected want 0 got %d", affected)
	}

	affected, err = repo.ConsumeManualStock(product.ID, 1)
	if err != nil {
		t.Fatalf("consume unlimited stock failed: %v", err)
	}
	if affected != 0 {
		t.Fatalf("consume unlimited affected want 0 got %d", affected)
	}
}

func TestUpdatePersistsChangedCategoryIDWithPreloadedCategory(t *testing.T) {
	repo, db := setupProductRepositoryTest(t)

	sourceCategory := &models.Category{
		Slug:     "source-category",
		NameJSON: models.JSON{"zh-CN": "source"},
	}
	targetCategory := &models.Category{
		Slug:     "target-category",
		NameJSON: models.JSON{"zh-CN": "target"},
	}
	if err := db.Create(sourceCategory).Error; err != nil {
		t.Fatalf("create source category failed: %v", err)
	}
	if err := db.Create(targetCategory).Error; err != nil {
		t.Fatalf("create target category failed: %v", err)
	}

	product := &models.Product{
		CategoryID:      sourceCategory.ID,
		Slug:            "category-switch-product",
		TitleJSON:       models.JSON{"zh-CN": "测试商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := repo.Create(product); err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	loaded, err := repo.GetByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil {
		t.Fatalf("load product failed: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected product to exist")
	}
	if loaded.Category.ID != sourceCategory.ID {
		t.Fatalf("expected preloaded category id=%d, got %d", sourceCategory.ID, loaded.Category.ID)
	}

	loaded.CategoryID = targetCategory.ID
	if err := repo.Update(loaded); err != nil {
		t.Fatalf("update product failed: %v", err)
	}

	reloaded, err := repo.GetByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if reloaded == nil {
		t.Fatal("expected reloaded product to exist")
	}
	if reloaded.CategoryID != targetCategory.ID {
		t.Fatalf("expected category_id=%d, got %d", targetCategory.ID, reloaded.CategoryID)
	}
	if reloaded.Category.ID != targetCategory.ID {
		t.Fatalf("expected preloaded category id=%d after update, got %d", targetCategory.ID, reloaded.Category.ID)
	}
}

func TestListManualStockStatusUsesActiveSKURemaining(t *testing.T) {
	repo, db := setupProductRepositoryTest(t)

	lowBySKU := createManualProduct(t, repo, "low-by-sku", 1, 0, 0)
	createManualSKU(t, db, lowBySKU.ID, "LOW", 0, 0, 1, true)

	normalBySKU := createManualProduct(t, repo, "normal-by-sku", 0, 0, 0)
	createManualSKU(t, db, normalBySKU.ID, "NORMAL", 2, 0, 0, true)

	unlimitedBySKU := createManualProduct(t, repo, "unlimited-by-sku", 0, 0, 0)
	createManualSKU(t, db, unlimitedBySKU.ID, "UNLIMITED", constants.ManualStockUnlimited, 0, 0, true)

	lowByFallback := createManualProduct(t, repo, "low-by-fallback", 0, 0, 0)
	createManualSKU(t, db, lowByFallback.ID, "INACTIVE-LOW", 5, 0, 0, false)

	normalByFallback := createManualProduct(t, repo, "normal-by-fallback", 3, 0, 0)
	createManualSKU(t, db, normalByFallback.ID, "INACTIVE-NORMAL", 0, 0, 0, false)

	unlimitedByFallback := createManualProduct(t, repo, "unlimited-by-fallback", constants.ManualStockUnlimited, 0, 0)
	createManualSKU(t, db, unlimitedByFallback.ID, "INACTIVE-UNLIMITED", 0, 0, 0, false)

	checkSlugs := func(status string, expected map[string]bool) {
		products, _, err := repo.List(ProductListFilter{
			Page:              1,
			PageSize:          100,
			ManualStockStatus: status,
		})
		if err != nil {
			t.Fatalf("list products by status=%s failed: %v", status, err)
		}
		got := make(map[string]bool, len(products))
		for _, item := range products {
			got[item.Slug] = true
		}
		for slug, want := range expected {
			if got[slug] != want {
				t.Fatalf("status=%s expect slug=%s present=%v got=%v", status, slug, want, got[slug])
			}
		}
	}

	checkSlugs("low", map[string]bool{
		lowBySKU.Slug:            true,
		lowByFallback.Slug:       true,
		normalBySKU.Slug:         false,
		normalByFallback.Slug:    false,
		unlimitedBySKU.Slug:      false,
		unlimitedByFallback.Slug: false,
	})

	checkSlugs("normal", map[string]bool{
		normalBySKU.Slug:         true,
		normalByFallback.Slug:    true,
		lowBySKU.Slug:            false,
		lowByFallback.Slug:       false,
		unlimitedBySKU.Slug:      false,
		unlimitedByFallback.Slug: false,
	})

	checkSlugs("unlimited", map[string]bool{
		unlimitedBySKU.Slug:      true,
		unlimitedByFallback.Slug: true,
		normalBySKU.Slug:         false,
		normalByFallback.Slug:    false,
		lowBySKU.Slug:            false,
		lowByFallback.Slug:       false,
	})
}
