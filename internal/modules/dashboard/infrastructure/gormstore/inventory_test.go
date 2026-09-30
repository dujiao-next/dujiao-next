package gormstore

import (
	"fmt"
	"testing"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestGetStockStatsUsesActiveManualSKUs(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	category := createDashboardCategory(t, db, "dashboard-manual-stock")

	lowStockProduct := &productdomain.Product{
		CategoryID:       category.ID,
		Slug:             "manual-low-stock",
		TitleJSON:        jsonmap.JSON{"zh-CN": "多 SKU 手动商品"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(99)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 999,
		IsActive:         true,
	}
	if err := db.Create(lowStockProduct).Error; err != nil {
		t.Fatalf("create low stock product failed: %v", err)
	}
	for idx, sku := range []productdomain.ProductSKU{
		{ProductID: lowStockProduct.ID, SKUCode: "A", PriceAmount: money.FromDecimal(decimal.NewFromInt(99)), ManualStockTotal: 2, IsActive: true},
		{ProductID: lowStockProduct.ID, SKUCode: "B", PriceAmount: money.FromDecimal(decimal.NewFromInt(99)), ManualStockTotal: 3, IsActive: true},
		{ProductID: lowStockProduct.ID, SKUCode: "DISABLED", PriceAmount: money.FromDecimal(decimal.NewFromInt(99)), ManualStockTotal: 100, IsActive: false},
	} {
		row := sku
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create manual sku failed: %v", err)
		}
		if idx == 2 {
			if err := db.Model(&row).Update("is_active", false).Error; err != nil {
				t.Fatalf("disable manual sku failed: %v", err)
			}
		}
	}

	unlimitedProduct := &productdomain.Product{
		CategoryID:       category.ID,
		Slug:             "manual-unlimited-sku",
		TitleJSON:        jsonmap.JSON{"zh-CN": "无限库存商品"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(88)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 0,
		IsActive:         true,
	}
	if err := db.Create(unlimitedProduct).Error; err != nil {
		t.Fatalf("create unlimited product failed: %v", err)
	}
	unlimitedSKU := &productdomain.ProductSKU{
		ProductID:        unlimitedProduct.ID,
		SKUCode:          "UNLIMITED",
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(88)),
		ManualStockTotal: constants.ManualStockUnlimited,
		IsActive:         true,
	}
	if err := db.Create(unlimitedSKU).Error; err != nil {
		t.Fatalf("create unlimited sku failed: %v", err)
	}

	outOfStockProduct := &productdomain.Product{
		CategoryID:       category.ID,
		Slug:             "manual-fallback-zero",
		TitleJSON:        jsonmap.JSON{"zh-CN": "回退零库存商品"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(77)),
		PurchaseType:     constants.ProductPurchaseMember,
		FulfillmentType:  constants.FulfillmentTypeManual,
		ManualStockTotal: 0,
		IsActive:         true,
	}
	if err := db.Create(outOfStockProduct).Error; err != nil {
		t.Fatalf("create fallback product failed: %v", err)
	}

	stats, err := repo.GetStockStats(5)
	if err != nil {
		t.Fatalf("get stock stats failed: %v", err)
	}
	if stats.ManualAvailableUnits != 5 {
		t.Fatalf("manual available units want 5 got %d", stats.ManualAvailableUnits)
	}
	if stats.LowStockProducts != 1 {
		t.Fatalf("low stock products want 1 got %d", stats.LowStockProducts)
	}
	if stats.OutOfStockProducts != 1 {
		t.Fatalf("out of stock products want 1 got %d", stats.OutOfStockProducts)
	}
}

func TestGetInventoryAlertItemsIncludesUpstreamProducts(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)

	// Migrate mapping tables for test
	if err := db.AutoMigrate(&mappingdomain.Mapping{}, &mappingdomain.SKUMapping{}); err != nil {
		t.Fatalf("migrate mapping tables failed: %v", err)
	}

	category := createDashboardCategory(t, db, "dashboard-upstream-alert")

	// Create upstream product with low stock
	upstreamProduct := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "upstream-low-stock",
		TitleJSON:       jsonmap.JSON{"zh-CN": "对接低库存商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(99)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
		IsActive:        true,
	}
	if err := db.Create(upstreamProduct).Error; err != nil {
		t.Fatalf("create upstream product failed: %v", err)
	}

	upstreamSKU := &productdomain.ProductSKU{
		ProductID:   upstreamProduct.ID,
		SKUCode:     "UP-SKU",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(99)),
		IsActive:    true,
		SortOrder:   0,
	}
	if err := db.Create(upstreamSKU).Error; err != nil {
		t.Fatalf("create upstream sku failed: %v", err)
	}

	// Create product mapping
	productMapping := &mappingdomain.Mapping{
		ConnectionID:   1,
		LocalProductID: upstreamProduct.ID,
	}
	if err := db.Create(productMapping).Error; err != nil {
		t.Fatalf("create product mapping failed: %v", err)
	}

	// Create SKU mapping with low stock
	skuMapping := &mappingdomain.SKUMapping{
		ProductMappingID: productMapping.ID,
		LocalSKUID:       upstreamSKU.ID,
		UpstreamSKUID:    999,
		UpstreamStock:    2,
		UpstreamIsActive: true,
	}
	if err := db.Create(skuMapping).Error; err != nil {
		t.Fatalf("create sku mapping failed: %v", err)
	}

	rows, err := repo.GetInventoryAlertItems(5)
	if err != nil {
		t.Fatalf("get inventory alert items failed: %v", err)
	}

	// Should include the upstream product
	if len(rows) != 1 {
		t.Fatalf("inventory alert rows want 1 got %d: %+v", len(rows), rows)
	}
	if rows[0].ProductID != upstreamProduct.ID {
		t.Fatalf("alert should be for upstream product %d, got %d", upstreamProduct.ID, rows[0].ProductID)
	}
	if rows[0].FulfillmentType != constants.FulfillmentTypeUpstream {
		t.Fatalf("fulfillment type want %s got %s", constants.FulfillmentTypeUpstream, rows[0].FulfillmentType)
	}
	if rows[0].AvailableStock != 2 {
		t.Fatalf("available stock want 2 got %d", rows[0].AvailableStock)
	}
	if rows[0].AlertType != constants.NotificationAlertTypeLowStockProducts {
		t.Fatalf("alert type want %s got %s", constants.NotificationAlertTypeLowStockProducts, rows[0].AlertType)
	}
}

func TestGetInventoryAlertItemsFallsBackToProductLevelWhenOnlyInactiveAutoSKUHasStock(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	if err := db.AutoMigrate(&cardsecretdomain.Secret{}); err != nil {
		t.Fatalf("migrate card secret failed: %v", err)
	}

	category := createDashboardCategory(t, db, "dashboard-auto-legacy-stock")
	product := &productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "dashboard-auto-legacy-stock",
		TitleJSON:       jsonmap.JSON{"zh-CN": "自动发货商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(99)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
	}
	if err := db.Create(product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	legacySKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(99)),
		IsActive:    false,
		SortOrder:   0,
	}
	activeSKU := &productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     "SKU-2",
		PriceAmount: money.FromDecimal(decimal.NewFromInt(99)),
		IsActive:    true,
		SortOrder:   1,
	}
	if err := db.Create(legacySKU).Error; err != nil {
		t.Fatalf("create legacy sku failed: %v", err)
	}
	if err := db.Model(legacySKU).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable legacy sku failed: %v", err)
	}
	if err := db.Create(activeSKU).Error; err != nil {
		t.Fatalf("create active sku failed: %v", err)
	}

	for idx := 0; idx < 2; idx++ {
		row := &cardsecretdomain.Secret{
			ProductID: product.ID,
			SKUID:     legacySKU.ID,
			Secret:    fmt.Sprintf("LEGACY-%d", idx),
			Status:    cardsecretdomain.StatusAvailable,
		}
		if err := db.Create(row).Error; err != nil {
			t.Fatalf("create legacy card secret failed: %v", err)
		}
	}

	rows, err := repo.GetInventoryAlertItems(5)
	if err != nil {
		t.Fatalf("get inventory alert items failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("inventory alert rows want 1 got %d: %+v", len(rows), rows)
	}
	if rows[0].SKUID != activeSKU.ID {
		t.Fatalf("fallback row should reuse the only active sku %d, got skuid=%d", activeSKU.ID, rows[0].SKUID)
	}
	if rows[0].AvailableStock != 2 {
		t.Fatalf("fallback row stock want 2 got %d", rows[0].AvailableStock)
	}
	if rows[0].AlertType != constants.NotificationAlertTypeLowStockProducts {
		t.Fatalf("fallback row alert type want low_stock_products got %s", rows[0].AlertType)
	}
}

func TestInventoryStatsSkipSKUsWithLoopCardSecret(t *testing.T) {
	repo, db := setupDashboardRepositoryTest(t)
	if err := db.AutoMigrate(&cardsecretdomain.Secret{}); err != nil {
		t.Fatalf("migrate card secret failed: %v", err)
	}

	category := createDashboardCategory(t, db, "dashboard-auto-loop")
	type skuSpec struct {
		code    string
		secrets []cardsecretdomain.Secret
	}
	createAuto := func(slug string, skus ...skuSpec) (*productdomain.Product, map[string]uint) {
		product := &productdomain.Product{
			CategoryID:      category.ID,
			Slug:            slug,
			TitleJSON:       jsonmap.JSON{"zh-CN": slug},
			PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
			PurchaseType:    constants.ProductPurchaseMember,
			FulfillmentType: constants.FulfillmentTypeAuto,
			IsActive:        true,
		}
		if err := db.Create(product).Error; err != nil {
			t.Fatalf("create product failed: %v", err)
		}
		skuIDs := make(map[string]uint, len(skus))
		for _, spec := range skus {
			sku := &productdomain.ProductSKU{ProductID: product.ID, SKUCode: spec.code, PriceAmount: product.PriceAmount, IsActive: true}
			if err := db.Create(sku).Error; err != nil {
				t.Fatalf("create sku failed: %v", err)
			}
			skuIDs[spec.code] = sku.ID
			for i := range spec.secrets {
				spec.secrets[i].ProductID, spec.secrets[i].SKUID = product.ID, sku.ID
				if err := db.Create(&spec.secrets[i]).Error; err != nil {
					t.Fatalf("create card secret failed: %v", err)
				}
			}
		}
		return product, skuIDs
	}
	loop := func(secret string) cardsecretdomain.Secret {
		return cardsecretdomain.Secret{Secret: secret, Status: cardsecretdomain.StatusAvailable, IsLoop: true}
	}
	normal := func(secret string) cardsecretdomain.Secret {
		return cardsecretdomain.Secret{Secret: secret, Status: cardsecretdomain.StatusAvailable}
	}

	// 全部 SKU 为循环卡密：不预警
	createAuto("dashboard-auto-loop", skuSpec{productdomain.DefaultSKUCode, []cardsecretdomain.Secret{loop("LOOP")}})
	// 普通卡密低于阈值：照常预警
	normalProduct, _ := createAuto("dashboard-auto-normal", skuSpec{productdomain.DefaultSKUCode, []cardsecretdomain.Secret{normal("N-1"), normal("N-2")}})
	// 混合商品：循环 SKU 不预警，普通 SKU 照常预警
	mixedProduct, mixedSKUs := createAuto("dashboard-auto-mixed",
		skuSpec{"LOOP", []cardsecretdomain.Secret{loop("M-LOOP")}},
		skuSpec{"NORMAL", []cardsecretdomain.Secret{normal("M-N-1")}},
	)

	rows, err := repo.GetInventoryAlertItems(5)
	if err != nil {
		t.Fatalf("get inventory alert items failed: %v", err)
	}
	got := make(map[[2]uint]int64, len(rows))
	for _, row := range rows {
		if row.AlertType != constants.NotificationAlertTypeLowStockProducts {
			t.Fatalf("unexpected alert type: %+v", row)
		}
		got[[2]uint{row.ProductID, row.SKUID}] = row.AvailableStock
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 low stock rows (normal product + mixed normal sku), got %+v", rows)
	}
	if stock, ok := got[[2]uint{mixedProduct.ID, mixedSKUs["NORMAL"]}]; !ok || stock != 1 {
		t.Fatalf("mixed product normal sku should alert with stock 1, got %+v", rows)
	}
	if _, ok := got[[2]uint{mixedProduct.ID, mixedSKUs["LOOP"]}]; ok {
		t.Fatalf("mixed product loop sku should not alert, got %+v", rows)
	}
	normalFound := false
	for key := range got {
		if key[0] == normalProduct.ID {
			normalFound = true
		}
	}
	if !normalFound {
		t.Fatalf("normal product should alert, got %+v", rows)
	}

	stats, err := repo.GetStockStats(5)
	if err != nil {
		t.Fatalf("get stock stats failed: %v", err)
	}
	// 商品级：含循环 SKU 的商品视为无限，只有普通商品计入；SKU 级：普通商品 + 混合商品普通 SKU
	if stats.LowStockProducts != 1 || stats.OutOfStockProducts != 0 || stats.LowStockSKUs != 2 || stats.OutOfStockSKUs != 0 {
		t.Fatalf("unexpected stock stats: %+v", stats)
	}
	if stats.AutoAvailableSecrets != 3 {
		t.Fatalf("loop secrets should not be counted as available units, want 3 got %d", stats.AutoAvailableSecrets)
	}
}
