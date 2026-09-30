package gormstore

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupProductStoreTest(t *testing.T) (*ProductStore, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:product_repository_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&cardsecretdomain.Secret{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("migrate product/sku/card_secret/mappings failed: %v", err)
	}
	defaultCategory := categorydomain.Category{
		ID:       1,
		Slug:     "default-test-category",
		NameJSON: jsonmap.JSON{"zh-CN": "default"},
		IsActive: true,
	}
	if err := db.Create(&defaultCategory).Error; err != nil {
		t.Fatalf("seed default category failed: %v", err)
	}
	return NewProductStore(db), db
}

func createManualProduct(t *testing.T, repo *ProductStore, slug string, total int, locked int, sold int) *productdomain.Product {
	t.Helper()
	product := &productdomain.Product{
		CategoryID:        1,
		Slug:              slug,
		TitleJSON:         jsonmap.JSON{"zh-CN": "测试商品"},
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(100)),
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

func createManualSKU(t *testing.T, db *gorm.DB, productID uint, code string, total int, locked int, sold int, isActive bool) *productdomain.ProductSKU {
	t.Helper()
	sku := &productdomain.ProductSKU{
		ProductID:         productID,
		SKUCode:           code,
		PriceAmount:       money.FromDecimal(decimal.NewFromInt(100)),
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

func createAutoProduct(t *testing.T, repo *ProductStore, slug string) *productdomain.Product {
	t.Helper()
	product := &productdomain.Product{
		CategoryID:      1,
		Slug:            slug,
		TitleJSON:       jsonmap.JSON{"zh-CN": "自动发货商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := repo.Create(product); err != nil {
		t.Fatalf("create auto product failed: %v", err)
	}
	return product
}

func createAvailableCardSecrets(t *testing.T, db *gorm.DB, productID uint, count int) {
	t.Helper()
	for i := 0; i < count; i++ {
		secret := &cardsecretdomain.Secret{
			ProductID: productID,
			SKUID:     0,
			Secret:    fmt.Sprintf("AUTO-SECRET-%d-%d", productID, i),
			Status:    cardsecretdomain.StatusAvailable,
		}
		if err := db.Create(secret).Error; err != nil {
			t.Fatalf("create card secret failed: %v", err)
		}
	}
}

func TestManualStockReserveReleaseConsumeLifecycle(t *testing.T) {
	repo, db := setupProductStoreTest(t)
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

	var got productdomain.Product
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
	repo, db := setupProductStoreTest(t)
	product := createManualProduct(t, repo, "manual-stock-legacy", 5, 0, 1)

	affected, err := repo.ConsumeManualStock(product.ID, 2)
	if err != nil {
		t.Fatalf("consume stock failed: %v", err)
	}
	if affected != 1 {
		t.Fatalf("consume affected want 1 got %d", affected)
	}

	var got productdomain.Product
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
	repo, _ := setupProductStoreTest(t)
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

func TestProductStoreSoftDeleteHidesProductAndRejectsStockMutations(t *testing.T) {
	repo, db := setupProductStoreTest(t)
	product := createManualProduct(t, repo, "soft-deleted-product", 10, 1, 0)

	if err := repo.Delete(strconv.FormatUint(uint64(product.ID), 10)); err != nil {
		t.Fatalf("soft delete product failed: %v", err)
	}

	var persisted productdomain.Product
	if err := db.Where("id = ?", product.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load persisted soft-deleted product failed: %v", err)
	}
	if persisted.DeletedAt == nil {
		t.Fatal("expected deleted_at to be persisted")
	}

	byID, err := repo.GetByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil || byID != nil {
		t.Fatalf("soft-deleted product must be hidden by id, product=%#v err=%v", byID, err)
	}
	adminByID, err := repo.GetAdminByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil || adminByID != nil {
		t.Fatalf("soft-deleted product must be hidden from admin lookup, product=%#v err=%v", adminByID, err)
	}
	bySlug, err := repo.GetBySlug(product.Slug, false)
	if err != nil || bySlug != nil {
		t.Fatalf("soft-deleted product must be hidden by slug, product=%#v err=%v", bySlug, err)
	}
	rows, total, err := repo.List(productcontract.ListFilter{})
	if err != nil {
		t.Fatalf("list products failed: %v", err)
	}
	if total != 0 || len(rows) != 0 {
		t.Fatalf("soft-deleted product must be absent from list, total=%d rows=%#v", total, rows)
	}
	count, err := repo.CountBySlug(product.Slug, nil)
	if err != nil || count != 0 {
		t.Fatalf("soft-deleted product must be absent from slug count, count=%d err=%v", count, err)
	}

	for operation, mutate := range map[string]func() (int64, error){
		"reserve": func() (int64, error) { return repo.ReserveManualStock(product.ID, 1) },
		"release": func() (int64, error) { return repo.ReleaseManualStock(product.ID, 1) },
		"consume": func() (int64, error) { return repo.ConsumeManualStock(product.ID, 1) },
	} {
		affected, err := mutate()
		if err != nil || affected != 0 {
			t.Fatalf("%s must not mutate soft-deleted product, affected=%d err=%v", operation, affected, err)
		}
	}
}

func TestProductStorePreloadsOnlyVisibleSKUs(t *testing.T) {
	repo, db := setupProductStoreTest(t)
	product := createManualProduct(t, repo, "visible-skus-only", 10, 0, 0)
	visible := createManualSKU(t, db, product.ID, "VISIBLE", 1, 0, 0, true)
	deleted := createManualSKU(t, db, product.ID, "DELETED", 1, 0, 0, true)
	deletedAt := time.Now()
	if err := db.Model(&productdomain.ProductSKU{}).Where("id = ?", deleted.ID).Update("deleted_at", deletedAt).Error; err != nil {
		t.Fatalf("soft delete sku fixture failed: %v", err)
	}

	got, err := repo.GetAdminByID(strconv.FormatUint(uint64(product.ID), 10))
	if err != nil {
		t.Fatalf("get product with skus failed: %v", err)
	}
	if got == nil || len(got.SKUs) != 1 || got.SKUs[0].ID != visible.ID {
		t.Fatalf("expected only visible sku %d, got %#v", visible.ID, got)
	}
}

func TestListManualStockStatusUsesActiveSKURemaining(t *testing.T) {
	repo, db := setupProductStoreTest(t)

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
		products, _, err := repo.List(productcontract.ListFilter{
			Page:              1,
			PageSize:          100,
			StockStatus:       status,
			LowStockThreshold: 5,
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

func TestListStockStatusAutoUsesLowStockThreshold(t *testing.T) {
	repo, db := setupProductStoreTest(t)

	createAutoProduct(t, repo, "auto-low-0")
	low3 := createAutoProduct(t, repo, "auto-low-3")
	normal6 := createAutoProduct(t, repo, "auto-normal-6")

	createAvailableCardSecrets(t, db, low3.ID, 3)
	createAvailableCardSecrets(t, db, normal6.ID, 6)
	// 仅 1 张循环卡密：视为无限库存，不应计入 low/normal
	loop1 := createAutoProduct(t, repo, "auto-loop-1")
	if err := db.Create(&cardsecretdomain.Secret{ProductID: loop1.ID, Secret: "LOOP", Status: cardsecretdomain.StatusAvailable, IsLoop: true}).Error; err != nil {
		t.Fatalf("create loop card secret failed: %v", err)
	}

	checkSlugs := func(status string, expected map[string]bool) {
		products, _, err := repo.List(productcontract.ListFilter{
			Page:              1,
			PageSize:          100,
			StockStatus:       status,
			LowStockThreshold: 5,
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
		"auto-low-0":    true,
		"auto-low-3":    true,
		"auto-normal-6": false,
		"auto-loop-1":   false,
	})

	checkSlugs("normal", map[string]bool{
		"auto-low-0":    false,
		"auto-low-3":    false,
		"auto-normal-6": true,
		"auto-loop-1":   false,
	})

	checkSlugs("unlimited", map[string]bool{
		"auto-low-0":    false,
		"auto-normal-6": false,
		"auto-loop-1":   true,
	})
}

func TestProductRepositoryListSortOrderDescending(t *testing.T) {
	repo, _ := setupProductStoreTest(t)

	high := &productdomain.Product{
		CategoryID:  1,
		Slug:        "high-sort-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "high"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		SortOrder:   100,
	}
	low := &productdomain.Product{
		CategoryID:  1,
		Slug:        "low-sort-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "low"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := repo.Create(high); err != nil {
		t.Fatalf("create high sort product failed: %v", err)
	}
	if err := repo.Create(low); err != nil {
		t.Fatalf("create low sort product failed: %v", err)
	}

	rows, total, err := repo.List(productcontract.ListFilter{
		Page:       1,
		PageSize:   20,
		OnlyActive: true,
	})
	if err != nil {
		t.Fatalf("list products failed: %v", err)
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

func TestProductRepositoryListSupportsNumericIDSearch(t *testing.T) {
	repo, _ := setupProductStoreTest(t)

	target := &productdomain.Product{
		CategoryID:      1,
		Slug:            "numeric-id-search-target",
		TitleJSON:       jsonmap.JSON{"zh-CN": "数字搜索目标"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := repo.Create(target); err != nil {
		t.Fatalf("create target product failed: %v", err)
	}

	other := &productdomain.Product{
		CategoryID:      1,
		Slug:            "numeric-id-search-other",
		TitleJSON:       jsonmap.JSON{"zh-CN": "另一个商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := repo.Create(other); err != nil {
		t.Fatalf("create other product failed: %v", err)
	}

	rows, total, err := repo.List(productcontract.ListFilter{
		Page:       1,
		PageSize:   20,
		Search:     strconv.FormatUint(uint64(target.ID), 10),
		OnlyActive: true,
	})
	if err != nil {
		t.Fatalf("search by numeric product id failed: %v", err)
	}
	if total == 0 || len(rows) == 0 {
		t.Fatalf("search by numeric product id should return target product")
	}
	if rows[0].ID != target.ID {
		found := false
		for _, row := range rows {
			if row.ID == target.ID {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("search result missing target product id=%d rows=%+v", target.ID, rows)
		}
	}
}

func TestProductRepositoryListFiltersWholesalePrices(t *testing.T) {
	repo, _ := setupProductStoreTest(t)

	withWholesale := &productdomain.Product{
		CategoryID:      1,
		Slug:            "with-wholesale",
		TitleJSON:       jsonmap.JSON{"zh-CN": "有批发价"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))}},
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := repo.Create(withWholesale); err != nil {
		t.Fatalf("create product with wholesale failed: %v", err)
	}

	withoutWholesale := &productdomain.Product{
		CategoryID:      1,
		Slug:            "without-wholesale",
		TitleJSON:       jsonmap.JSON{"zh-CN": "无批发价"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := repo.Create(withoutWholesale); err != nil {
		t.Fatalf("create product without wholesale failed: %v", err)
	}

	enabled := true
	rows, _, err := repo.List(productcontract.ListFilter{Page: 1, PageSize: 20, HasWholesalePrices: &enabled})
	if err != nil {
		t.Fatalf("list products with wholesale failed: %v", err)
	}
	if len(rows) != 1 || rows[0].Slug != withWholesale.Slug {
		t.Fatalf("expected only product with wholesale, got %+v", rows)
	}

	disabled := false
	rows, _, err = repo.List(productcontract.ListFilter{Page: 1, PageSize: 20, HasWholesalePrices: &disabled})
	if err != nil {
		t.Fatalf("list products without wholesale failed: %v", err)
	}
	got := make(map[string]bool, len(rows))
	for _, row := range rows {
		got[row.Slug] = true
	}
	if got[withWholesale.Slug] {
		t.Fatalf("product with wholesale should not be returned: %+v", rows)
	}
	if !got[withoutWholesale.Slug] {
		t.Fatalf("product without wholesale missing: %+v", rows)
	}
}

func createActiveStateProduct(t *testing.T, repo *ProductStore, slug string, title string, isActive bool, wholesale bool) *productdomain.Product {
	t.Helper()
	product := &productdomain.Product{
		CategoryID:       1,
		Slug:             slug,
		TitleJSON:        jsonmap.JSON{"zh-CN": title},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 10,
		IsActive:         isActive,
	}
	if wholesale {
		product.WholesalePrices = productdomain.WholesalePriceTiers{{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))}}
	}
	if err := repo.Create(product); err != nil {
		t.Fatalf("create product %s failed: %v", slug, err)
	}
	return product
}

func listSlugs(t *testing.T, repo *ProductStore, filter productcontract.ListFilter) map[string]bool {
	t.Helper()
	if filter.Page == 0 {
		filter.Page = 1
	}
	if filter.PageSize == 0 {
		filter.PageSize = 50
	}
	rows, _, err := repo.List(filter)
	if err != nil {
		t.Fatalf("list products failed: %v", err)
	}
	got := make(map[string]bool, len(rows))
	for _, row := range rows {
		got[row.Slug] = true
	}
	return got
}

// 上架状态筛选是三态的：nil 不限、true 仅已上架、false 仅已下架。
func TestProductRepositoryListFiltersActiveState(t *testing.T) {
	repo, _ := setupProductStoreTest(t)

	active := createActiveStateProduct(t, repo, "state-active", "上架商品", true, false)
	inactive := createActiveStateProduct(t, repo, "state-inactive", "下架商品", false, false)

	onlyActive := true
	got := listSlugs(t, repo, productcontract.ListFilter{IsActive: &onlyActive})
	if !got[active.Slug] || got[inactive.Slug] {
		t.Fatalf("IsActive=true should return only the active product, got %+v", got)
	}

	onlyInactive := false
	got = listSlugs(t, repo, productcontract.ListFilter{IsActive: &onlyInactive})
	if !got[inactive.Slug] || got[active.Slug] {
		t.Fatalf("IsActive=false should return only the inactive product, got %+v", got)
	}

	got = listSlugs(t, repo, productcontract.ListFilter{})
	if !got[active.Slug] || !got[inactive.Slug] {
		t.Fatalf("IsActive=nil should return both products, got %+v", got)
	}
}

// 上架状态必须与搜索、批发价、库存状态等条件按 AND 组合，不能互相覆盖。
func TestProductRepositoryListCombinesActiveStateWithOtherFilters(t *testing.T) {
	repo, _ := setupProductStoreTest(t)

	activeHit := createActiveStateProduct(t, repo, "combo-active-hit", "组合命中商品", true, true)
	inactiveHit := createActiveStateProduct(t, repo, "combo-inactive-hit", "组合命中商品", false, true)
	inactiveNoWholesale := createActiveStateProduct(t, repo, "combo-inactive-nowholesale", "组合命中商品", false, false)
	unrelated := createActiveStateProduct(t, repo, "other-active", "无关商品", true, true)

	onlyInactive := false
	withWholesale := true

	// 下架 + 有批发价：只剩 inactiveHit
	got := listSlugs(t, repo, productcontract.ListFilter{IsActive: &onlyInactive, HasWholesalePrices: &withWholesale})
	if !got[inactiveHit.Slug] || got[inactiveNoWholesale.Slug] || got[activeHit.Slug] || got[unrelated.Slug] {
		t.Fatalf("inactive+wholesale combination mismatch: %+v", got)
	}

	// 下架 + 搜索：命中两个下架商品，且排除已上架的 activeHit
	got = listSlugs(t, repo, productcontract.ListFilter{IsActive: &onlyInactive, Search: "组合命中"})
	if !got[inactiveHit.Slug] || !got[inactiveNoWholesale.Slug] || got[activeHit.Slug] || got[unrelated.Slug] {
		t.Fatalf("inactive+search combination mismatch: %+v", got)
	}

	// 已上架 + 搜索：只剩 activeHit
	onlyActive := true
	got = listSlugs(t, repo, productcontract.ListFilter{IsActive: &onlyActive, Search: "组合命中"})
	if !got[activeHit.Slug] || len(got) != 1 {
		t.Fatalf("active+search combination mismatch: %+v", got)
	}

	// 下架 + 库存正常：manual 商品剩余 10 > 0，两个下架商品都命中；再叠加批发价收窄到 1 个
	got = listSlugs(t, repo, productcontract.ListFilter{
		IsActive:           &onlyInactive,
		StockStatus:        "normal",
		LowStockThreshold:  5,
		HasWholesalePrices: &withWholesale,
	})
	if !got[inactiveHit.Slug] || len(got) != 1 {
		t.Fatalf("inactive+stock+wholesale combination mismatch: %+v", got)
	}

	// 搜索无命中时，即使上架状态匹配也必须返回空集
	got = listSlugs(t, repo, productcontract.ListFilter{IsActive: &onlyInactive, Search: "不存在的关键词"})
	if len(got) != 0 {
		t.Fatalf("non-matching search should return nothing, got %+v", got)
	}
}

// OnlyActive 是公开侧强制的「必须上架」，不应被 IsActive=false 反向覆盖成空集。
func TestProductRepositoryListOnlyActiveTakesPrecedenceOverIsActive(t *testing.T) {
	repo, _ := setupProductStoreTest(t)

	active := createActiveStateProduct(t, repo, "precedence-active", "上架商品", true, false)
	createActiveStateProduct(t, repo, "precedence-inactive", "下架商品", false, false)

	onlyInactive := false
	got := listSlugs(t, repo, productcontract.ListFilter{OnlyActive: true, IsActive: &onlyInactive})
	if !got[active.Slug] || len(got) != 1 {
		t.Fatalf("OnlyActive should win over IsActive, got %+v", got)
	}
}
