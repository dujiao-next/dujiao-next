package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/upstream"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type failingSKUMappingRepo struct {
	err error
}

func (r *failingSKUMappingRepo) GetByID(id uint) (*models.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) GetByLocalSKUID(skuID uint) (*models.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) GetByMappingAndUpstreamSKUID(productMappingID, upstreamSKUID uint) (*models.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) ListByProductMapping(productMappingID uint) ([]models.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) ListByProductMappingIDs(productMappingIDs []uint) ([]models.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) WithTx(tx *gorm.DB) repository.SKUMappingRepository {
	return r
}

func (r *failingSKUMappingRepo) Create(mapping *models.SKUMapping) error {
	return r.err
}

func (r *failingSKUMappingRepo) Update(mapping *models.SKUMapping) error {
	return nil
}

func (r *failingSKUMappingRepo) Delete(id uint) error {
	return nil
}

func (r *failingSKUMappingRepo) DeleteByProductMapping(productMappingID uint) error {
	return nil
}

func (r *failingSKUMappingRepo) BatchUpsert(mappings []models.SKUMapping) error {
	return r.err
}

func TestImportUpstreamProductRollbackWhenSKUMappingCreateFails(t *testing.T) {
	dsn := "file:product_mapping_import_rollback?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.SiteConnection{},
		&models.ProductMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	if err := categoryRepo.Create(&models.Category{
		ParentID: 0,
		Slug:     "test-category",
		NameJSON: models.JSON{"zh-CN": "Test Category"},
	}); err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/upstream/products/101" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"product": upstream.UpstreamProduct{
				ID:              101,
				Title:           models.JSON{"zh-CN": "映射测试商品"},
				Description:     models.JSON{"zh-CN": "描述"},
				Content:         models.JSON{"zh-CN": "内容"},
				Images:          []string{},
				Tags:            []string{"tag-a"},
				PriceAmount:     "10.00",
				Currency:        "CNY",
				FulfillmentType: constants.FulfillmentTypeAuto,
				IsActive:        true,
				SKUs: []upstream.UpstreamSKU{
					{
						ID:          201,
						SKUCode:     "SKU-A",
						SpecValues:  models.JSON{"name": "A"},
						PriceAmount: "10.00",
						IsActive:    true,
					},
				},
			},
		})
	}))
	defer server.Close()

	connService := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(CreateConnectionInput{
		Name:      "upstream-a",
		BaseURL:   server.URL,
		ApiKey:    "test-key",
		ApiSecret: "test-secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	svc := NewProductMappingService(
		repository.NewProductMappingRepository(db),
		&failingSKUMappingRepo{err: errors.New("inject sku mapping failure")},
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
		categoryRepo,
		connService,
	)

	if _, err := svc.ImportUpstreamProduct(conn.ID, 101, 1, "rollback-slug"); err == nil {
		t.Fatalf("expected import upstream product to fail")
	}

	var productCount int64
	if err := db.Model(&models.Product{}).Count(&productCount).Error; err != nil {
		t.Fatalf("count products failed: %v", err)
	}
	if productCount != 0 {
		t.Fatalf("expected product rollback, got %d products", productCount)
	}

	var skuCount int64
	if err := db.Model(&models.ProductSKU{}).Count(&skuCount).Error; err != nil {
		t.Fatalf("count product skus failed: %v", err)
	}
	if skuCount != 0 {
		t.Fatalf("expected sku rollback, got %d skus", skuCount)
	}

	var mappingCount int64
	if err := db.Model(&models.ProductMapping{}).Count(&mappingCount).Error; err != nil {
		t.Fatalf("count product mappings failed: %v", err)
	}
	if mappingCount != 0 {
		t.Fatalf("expected mapping rollback, got %d mappings", mappingCount)
	}
}

// setupMappingWithUpstreamHandler 准备一份本地映射 + 启动可定制响应的上游 httptest server
func setupMappingWithUpstreamHandler(t *testing.T, dsn string, handler http.HandlerFunc) (*ProductMappingService, *gorm.DB, *models.ProductMapping, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.SiteConnection{},
		&models.ProductMapping{},
		&models.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	server := httptest.NewServer(handler)

	categoryRepo := repository.NewCategoryRepository(db)
	if err := categoryRepo.Create(&models.Category{Slug: "c", NameJSON: models.JSON{"zh-CN": "C"}}); err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	productRepo := repository.NewProductRepository(db)
	product := models.Product{
		CategoryID:      1,
		Slug:            "p",
		TitleJSON:       models.JSON{"zh-CN": "P"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsActive:        true,
		IsMapped:        true,
	}
	if err := productRepo.Create(&product); err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	skuRepo := repository.NewProductSKURepository(db)
	sku := models.ProductSKU{ProductID: product.ID, SKUCode: "SKU-A", PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)), IsActive: true}
	if err := skuRepo.Create(&sku); err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	connService := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(CreateConnectionInput{
		Name:      "upstream",
		BaseURL:   server.URL,
		ApiKey:    "k",
		ApiSecret: "s",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	mappingRepo := repository.NewProductMappingRepository(db)
	skuMappingRepo := repository.NewSKUMappingRepository(db)
	mapping := &models.ProductMapping{
		ConnectionID:      conn.ID,
		LocalProductID:    product.ID,
		UpstreamProductID: 101,
		IsActive:          true,
		UpstreamStatus:    models.UpstreamStatusActive,
	}
	if err := mappingRepo.Create(mapping); err != nil {
		t.Fatalf("create mapping failed: %v", err)
	}
	if err := skuMappingRepo.Create(&models.SKUMapping{
		ProductMappingID: mapping.ID,
		LocalSKUID:       sku.ID,
		UpstreamSKUID:    201,
		UpstreamIsActive: true,
		UpstreamStock:    100,
	}); err != nil {
		t.Fatalf("create sku mapping failed: %v", err)
	}

	svc := NewProductMappingService(mappingRepo, skuMappingRepo, productRepo, skuRepo, categoryRepo, connService)
	return svc, db, mapping, server.Close
}

func TestSyncProductMarksDeletedWhenUpstreamSoftDeleted(t *testing.T) {
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_deleted?mode=memory&cache=shared",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":            false,
				"error_code":    "product_deleted",
				"error_message": "product has been deleted",
			})
		},
	)
	defer cleanup()

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var got models.ProductMapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != models.UpstreamStatusDeleted {
		t.Fatalf("expected upstream_status=deleted, got %q", got.UpstreamStatus)
	}
	if got.IsActive {
		t.Fatalf("expected mapping to be deactivated for deleted upstream")
	}

	var product models.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if product.IsActive {
		t.Fatalf("expected local product to be deactivated")
	}

	var skuMapping models.SKUMapping
	if err := db.Where("product_mapping_id = ?", mapping.ID).First(&skuMapping).Error; err != nil {
		t.Fatalf("reload sku mapping failed: %v", err)
	}
	if skuMapping.UpstreamIsActive || skuMapping.UpstreamStock != 0 {
		t.Fatalf("expected sku mapping to be marked unavailable, got is_active=%v stock=%d", skuMapping.UpstreamIsActive, skuMapping.UpstreamStock)
	}
}

func TestSyncProductMarksInactiveWhenUpstreamReturnsInactive(t *testing.T) {
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_inactive?mode=memory&cache=shared",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:       101,
					IsActive: false, // 上游下架
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", PriceAmount: "10.00", IsActive: false},
					},
				},
			})
		},
	)
	defer cleanup()

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var got models.ProductMapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != models.UpstreamStatusInactive {
		t.Fatalf("expected upstream_status=inactive, got %q", got.UpstreamStatus)
	}
	if !got.IsActive {
		t.Fatalf("expected mapping to remain active for inactive upstream (only deleted should auto-disable)")
	}

	var product models.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if product.IsActive {
		t.Fatalf("expected local product to be deactivated")
	}
}

func TestSyncProductKeepsLocalWholesalePricesWhenUpstreamOmitsWholesalePrices(t *testing.T) {
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_keep_local_wholesale?mode=memory&cache=shared",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:              101,
					Title:           models.JSON{"zh-CN": "测试"},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", PriceAmount: "10.00", IsActive: true, StockQuantity: 100},
					},
				},
			})
		},
	)
	defer cleanup()

	localWholesalePrices := models.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(80))},
	}
	if err := db.Model(&models.Product{}).
		Where("id = ?", mapping.LocalProductID).
		Update("wholesale_prices", localWholesalePrices).Error; err != nil {
		t.Fatalf("seed local wholesale prices failed: %v", err)
	}

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var product models.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if len(product.WholesalePrices) != 1 {
		t.Fatalf("expected local wholesale prices to be kept, got %+v", product.WholesalePrices)
	}
	if product.WholesalePrices[0].MinQuantity != 5 || product.WholesalePrices[0].UnitPrice.String() != "80.00" {
		t.Fatalf("unexpected wholesale tier: %+v", product.WholesalePrices[0])
	}
}

func TestSyncProductRemapsUpstreamWholesaleSKUID(t *testing.T) {
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_remap_wholesale_sku?mode=memory&cache=shared",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:              101,
					Title:           models.JSON{"zh-CN": "测试"},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", PriceAmount: "10.00", IsActive: true, StockQuantity: 100},
					},
					WholesalePrices: models.WholesalePriceTiers{
						{SKUID: 201, MinQuantity: 5, UnitPrice: models.NewMoneyFromDecimal(decimal.NewFromInt(8))},
					},
				},
			})
		},
	)
	defer cleanup()

	var localSKU models.ProductSKU
	if err := db.Where("product_id = ? AND sku_code = ?", mapping.LocalProductID, "SKU-A").First(&localSKU).Error; err != nil {
		t.Fatalf("load local sku failed: %v", err)
	}
	if localSKU.ID == 201 {
		t.Fatalf("test setup invalid: local sku id should differ from upstream id")
	}

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var product models.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if len(product.WholesalePrices) != 1 {
		t.Fatalf("expected one wholesale tier, got %+v", product.WholesalePrices)
	}
	if product.WholesalePrices[0].SKUID != localSKU.ID || product.WholesalePrices[0].SKUCode != localSKU.SKUCode {
		t.Fatalf("expected upstream sku_id remapped to local sku, got %+v local=%+v", product.WholesalePrices[0], localSKU)
	}
}

// listProductsHandler 构造一个 /api/v1/upstream/products 列表响应 handler
func listProductsHandler(items []upstream.UpstreamProduct, includesInactive bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"items":             items,
			"total":             len(items),
			"page":              1,
			"page_size":         50,
			"includes_inactive": includesInactive,
		})
	}
}

func TestSyncConnectionStockMarksDeletedWhenFullSyncMissing(t *testing.T) {
	// 上游 ListProducts 返回空列表 + includes_inactive=true →
	// 全量模式下 mapping 在列表中 missing 必定意味着上游已软删
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_full_missing_deleted?mode=memory&cache=shared",
		listProductsHandler([]upstream.UpstreamProduct{}, true),
	)
	defer cleanup()

	if err := svc.syncConnectionStock(mapping.ConnectionID, []models.ProductMapping{*mapping}, 50, 200); err != nil {
		t.Fatalf("syncConnectionStock returned error: %v", err)
	}

	var got models.ProductMapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != models.UpstreamStatusDeleted {
		t.Fatalf("expected upstream_status=deleted, got %q", got.UpstreamStatus)
	}
	if got.IsActive {
		t.Fatalf("expected mapping to be deactivated when upstream marks deleted")
	}

	var product models.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if product.IsActive {
		t.Fatalf("expected local product to be deactivated")
	}
}

func TestSyncConnectionStockKeepsLegacyUpstreamMissing(t *testing.T) {
	// 上游空列表 + includes_inactive=false（旧上游不支持新参数）→
	// 不能据此推断"missing=已删除"，本地状态应保持不变
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_full_missing_legacy?mode=memory&cache=shared",
		listProductsHandler([]upstream.UpstreamProduct{}, false),
	)
	defer cleanup()

	if err := svc.syncConnectionStock(mapping.ConnectionID, []models.ProductMapping{*mapping}, 50, 200); err != nil {
		t.Fatalf("syncConnectionStock returned error: %v", err)
	}

	var got models.ProductMapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != models.UpstreamStatusActive {
		t.Fatalf("legacy upstream missing must not change status, got %q", got.UpstreamStatus)
	}
	if !got.IsActive {
		t.Fatalf("legacy upstream missing must not deactivate mapping")
	}

	var product models.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if !product.IsActive {
		t.Fatalf("legacy upstream missing must not deactivate local product")
	}
}

func TestSyncConnectionStockKeepsMappingWhenFullSyncIncomplete(t *testing.T) {
	// 模拟上游分页返回不完整：page=1 返回 total=10 但只回 1 条（不含我们的 mapping），
	// page=2 异常返回 items=[] —— 当前实现会因 items==0 提前 break，
	// 然后把所有未在列表中的 mapping 错误地标为 deleted。
	// 期望：检测到拉取不完整 → 跳过删除判定，保留原状态。
	var pageCalls int
	handler := func(w http.ResponseWriter, r *http.Request) {
		pageCalls++
		w.Header().Set("Content-Type", "application/json")
		if pageCalls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"items": []upstream.UpstreamProduct{
					{ID: 999, Title: models.JSON{"zh-CN": "其他商品"}, PriceAmount: "1.00", IsActive: true},
				},
				"total":             10,
				"page":              1,
				"page_size":         50,
				"includes_inactive": true,
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":                true,
			"items":             []upstream.UpstreamProduct{},
			"total":             10,
			"page":              pageCalls,
			"page_size":         50,
			"includes_inactive": true,
		})
	}

	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_incomplete_no_delete?mode=memory&cache=shared",
		handler,
	)
	defer cleanup()

	if err := svc.syncConnectionStock(mapping.ConnectionID, []models.ProductMapping{*mapping}, 50, 200); err != nil {
		t.Fatalf("syncConnectionStock returned error: %v", err)
	}

	var got models.ProductMapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != models.UpstreamStatusActive {
		t.Fatalf("incomplete full sync must not mark missing mapping as deleted, got %q", got.UpstreamStatus)
	}
	if !got.IsActive {
		t.Fatalf("incomplete full sync must not deactivate missing mapping")
	}
}

func TestEnsureUpstreamStockReturnsNilWhenCachedStockSufficient(t *testing.T) {
	// 缓存库存=100，下单需要 1 → 直接放行，不应触发上游调用
	var callCount int
	handler := func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
	svc, db, _, cleanup := setupMappingWithUpstreamHandler(t,
		"file:preorder_cache_sufficient?mode=memory&cache=shared",
		handler,
	)
	defer cleanup()

	var sku models.ProductSKU
	if err := db.First(&sku).Error; err != nil {
		t.Fatalf("load sku failed: %v", err)
	}

	if err := svc.EnsureUpstreamStockForOrder(sku.ID, 1); err != nil {
		t.Fatalf("expected nil for sufficient cached stock, got %v", err)
	}
	if callCount != 0 {
		t.Fatalf("expected zero upstream calls when cache is sufficient, got %d", callCount)
	}
}

func TestEnsureUpstreamStockReturnsNilWhenNoMapping(t *testing.T) {
	// 非上游商品（没有 sku_mapping）→ 放行
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}
	svc, _, _, cleanup := setupMappingWithUpstreamHandler(t,
		"file:preorder_no_mapping?mode=memory&cache=shared",
		handler,
	)
	defer cleanup()

	if err := svc.EnsureUpstreamStockForOrder(99999, 1); err != nil {
		t.Fatalf("expected nil for non-upstream sku, got %v", err)
	}
}

func TestEnsureUpstreamStockRejectsWhenUpstreamReportsZero(t *testing.T) {
	// 缓存=0，实时同步上游返回 stock=0 → 拒绝
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// SyncProduct 走 GetProduct (/api/v1/upstream/products/:id)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"product": upstream.UpstreamProduct{
				ID:              101,
				Title:           models.JSON{"zh-CN": "测试"},
				PriceAmount:     "10.00",
				Currency:        "CNY",
				FulfillmentType: constants.FulfillmentTypeAuto,
				IsActive:        true,
				SKUs: []upstream.UpstreamSKU{
					{ID: 201, SKUCode: "SKU-A", PriceAmount: "10.00", IsActive: true, StockQuantity: 0},
				},
			},
		})
	}
	svc, db, _, cleanup := setupMappingWithUpstreamHandler(t,
		"file:preorder_rejects_zero?mode=memory&cache=shared",
		handler,
	)
	defer cleanup()

	// 强制把缓存 stock 降到不足
	if err := db.Model(&models.SKUMapping{}).Where("upstream_sku_id = ?", 201).
		Update("upstream_stock", 0).Error; err != nil {
		t.Fatalf("reset stock failed: %v", err)
	}

	var sku models.ProductSKU
	if err := db.First(&sku).Error; err != nil {
		t.Fatalf("load sku failed: %v", err)
	}

	err := svc.EnsureUpstreamStockForOrder(sku.ID, 1)
	if !errors.Is(err, ErrUpstreamStockInsufficient) {
		t.Fatalf("expected ErrUpstreamStockInsufficient, got %v", err)
	}
}

func TestEnsureUpstreamStockFailsOpenWhenUpstreamDown(t *testing.T) {
	// 缓存=0，上游 500 → fail-open，下单放行
	handler := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream down", http.StatusInternalServerError)
	}
	svc, db, _, cleanup := setupMappingWithUpstreamHandler(t,
		"file:preorder_fail_open?mode=memory&cache=shared",
		handler,
	)
	defer cleanup()

	if err := db.Model(&models.SKUMapping{}).Where("upstream_sku_id = ?", 201).
		Update("upstream_stock", 0).Error; err != nil {
		t.Fatalf("reset stock failed: %v", err)
	}

	var sku models.ProductSKU
	if err := db.First(&sku).Error; err != nil {
		t.Fatalf("load sku failed: %v", err)
	}

	if err := svc.EnsureUpstreamStockForOrder(sku.ID, 1); err != nil {
		t.Fatalf("expected nil (fail-open) when upstream is down, got %v", err)
	}
}

func TestSyncProductRestoresStatusWhenUpstreamRecovers(t *testing.T) {
	// 之前已被标 inactive，上游 GetProduct 返回 IsActive=true → UpstreamStatus 应恢复为 active
	svc, db, mapping, cleanup := setupMappingWithUpstreamHandler(t,
		"file:sync_recover_active?mode=memory&cache=shared",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:              101,
					Title:           models.JSON{"zh-CN": "P"},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", PriceAmount: "10.00", IsActive: true, StockQuantity: 50},
					},
				},
			})
		},
	)
	defer cleanup()

	// 先把 mapping 状态改为 inactive 模拟"之前已下架"
	if err := db.Model(&models.ProductMapping{}).Where("id = ?", mapping.ID).
		Update("upstream_status", models.UpstreamStatusInactive).Error; err != nil {
		t.Fatalf("preset inactive failed: %v", err)
	}

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var got models.ProductMapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != models.UpstreamStatusActive {
		t.Fatalf("expected upstream_status to recover to active, got %q", got.UpstreamStatus)
	}
}

func TestImportUpstreamProductRejectsInactive(t *testing.T) {
	// 上游 GetProduct 返回 200 + is_active=false → 拒绝导入
	dsn := "file:import_reject_inactive?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.SiteConnection{},
		&models.ProductMapping{},
		&models.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	if err := categoryRepo.Create(&models.Category{Slug: "c", NameJSON: models.JSON{"zh-CN": "C"}}); err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"product": upstream.UpstreamProduct{
				ID:          202,
				Title:       models.JSON{"zh-CN": "已下架商品"},
				PriceAmount: "10.00",
				IsActive:    false,
			},
		})
	}))
	defer server.Close()

	connService := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(CreateConnectionInput{
		Name: "u", BaseURL: server.URL, ApiKey: "k", ApiSecret: "s",
		Protocol: constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	svc := NewProductMappingService(
		repository.NewProductMappingRepository(db),
		repository.NewSKUMappingRepository(db),
		repository.NewProductRepository(db),
		repository.NewProductSKURepository(db),
		categoryRepo,
		connService,
	)

	_, importErr := svc.ImportUpstreamProduct(conn.ID, 202, 1, "")
	if !errors.Is(importErr, ErrUpstreamProductNotFound) {
		t.Fatalf("expected ErrUpstreamProductNotFound for inactive upstream product, got %v", importErr)
	}

	var productCount int64
	if err := db.Model(&models.Product{}).Count(&productCount).Error; err != nil {
		t.Fatalf("count products failed: %v", err)
	}
	if productCount != 0 {
		t.Fatalf("expected no local product created when import rejected, got %d", productCount)
	}
}

// TestFindOrCreateLocalCategoryRestoresSoftDeleted 验证当存在同 slug 的软删除分类时，
// 自动创建路径会复活该分类而非触发 UNIQUE 约束失败。
func TestFindOrCreateLocalCategoryRestoresSoftDeleted(t *testing.T) {
	dsn := "file:product_mapping_restore_soft_deleted?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	categoryRepo := repository.NewCategoryRepository(db)
	categoryService := NewCategoryService(categoryRepo)

	// 创建一个分类后软删除
	existing := &models.Category{
		ParentID: 0,
		Slug:     "softdel-streaming",
		NameJSON: models.JSON{"zh-CN": "旧名"},
		IsActive: true,
	}
	if err := categoryRepo.Create(existing); err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	if err := categoryRepo.Delete(strconv.FormatUint(uint64(existing.ID), 10)); err != nil {
		t.Fatalf("soft delete category failed: %v", err)
	}

	// 确认软删除后 GetBySlug 找不到，但 Unscoped 能找到
	if got, err := categoryRepo.GetBySlug("softdel-streaming"); err != nil || got != nil {
		t.Fatalf("expected slug to be invisible after soft delete, got=%v err=%v", got, err)
	}

	svc := &ProductMappingService{categoryRepo: categoryRepo}
	svc.SetCategoryService(categoryService)

	restored, err := svc.findOrCreateLocalCategory("softdel-streaming", models.JSON{"zh-CN": "新名"}, 0)
	if err != nil {
		t.Fatalf("findOrCreateLocalCategory failed: %v", err)
	}
	if restored == nil || restored.ID != existing.ID {
		t.Fatalf("expected restored category id=%d, got=%+v", existing.ID, restored)
	}

	// 复活后应再次可见，且 NameJSON 已刷新
	visible, err := categoryRepo.GetBySlug("softdel-streaming")
	if err != nil {
		t.Fatalf("GetBySlug after restore failed: %v", err)
	}
	if visible == nil {
		t.Fatalf("expected category to be visible after restore")
	}
	if name, _ := visible.NameJSON["zh-CN"].(string); name != "新名" {
		t.Fatalf("expected NameJSON refreshed to 新名, got %q", name)
	}
	if !visible.IsActive {
		t.Fatalf("expected restored category to be active")
	}

	// 数据库中应该只有一行（且未被软删除）
	var total int64
	if err := db.Unscoped().Model(&models.Category{}).Where("slug = ?", "softdel-streaming").Count(&total).Error; err != nil {
		t.Fatalf("count categories failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 row for slug after restore, got %d", total)
	}
}
