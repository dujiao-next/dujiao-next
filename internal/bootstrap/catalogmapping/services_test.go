package catalogmappingbootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorycontract "github.com/dujiao-next/internal/modules/catalog/category/contract"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	"github.com/dujiao-next/internal/constants"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappingapp "github.com/dujiao-next/internal/modules/catalog/mapping/application"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
	siteconnectiongormstore "github.com/dujiao-next/internal/modules/siteconnection/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/dujiao-next/internal/upstream"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type noopLocalMediaRecorder struct{}

func (noopLocalMediaRecorder) RecordLocalFile(context.Context, string, string) {}

func newTestMappingService(
	mappingRepo MappingStore,
	skuMappingRepo SKUMappingStore,
	productRepo ProductStore,
	productSKURepo SKUStore,
	categoryRepo categorycontract.Repository,
	connService *siteconnectionapp.Service,
	mediaRecorder mappingcontract.MediaRecorder,
) (*mappingapp.Service, error) {
	return New(Dependencies{
		Mappings:    mappingRepo,
		SKUMappings: skuMappingRepo,
		Products:    productRepo,
		SKUs:        productSKURepo,
		Categories:  categoryRepo,
		Connections: connService,
		Media:       mediaRecorder,
	})
}

func TestNewRejectsNilMediaRecorder(t *testing.T) {
	if _, err := newTestMappingService(nil, nil, nil, nil, nil, nil, nil); !errors.Is(err, mappingcontract.ErrMediaRecorderRequired) {
		t.Fatalf("error = %v, want ErrMediaRecorderRequired", err)
	}
}

type failingSKUMappingRepo struct {
	err error
}

func (r *failingSKUMappingRepo) GetByLocalSKUID(skuID uint) (*mappingdomain.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) ListByProductMapping(productMappingID uint) ([]mappingdomain.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) ListByProductMappingIDs(productMappingIDs []uint) ([]mappingdomain.SKUMapping, error) {
	return nil, nil
}

func (r *failingSKUMappingRepo) BindTx(tx *gorm.DB) mappingcontract.SKUMappingRepository {
	return r
}

func (r *failingSKUMappingRepo) Create(mapping *mappingdomain.SKUMapping) error {
	return r.err
}

func (r *failingSKUMappingRepo) Update(mapping *mappingdomain.SKUMapping) error {
	return nil
}

func (r *failingSKUMappingRepo) DeleteByProductMapping(productMappingID uint) error {
	return nil
}

func TestImportUpstreamProductRollbackWhenSKUMappingCreateFails(t *testing.T) {
	dsn := "file:product_mapping_import_rollback?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	categoryRepo := categorygormstore.NewCategoryStore(db)
	if err := categoryRepo.Create(&categorydomain.Category{
		ParentID: 0,
		Slug:     "test-category",
		NameJSON: jsonmap.JSON{"zh-CN": "Test Category"},
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
				Title:           jsonmap.JSON{"zh-CN": "映射测试商品"},
				Description:     jsonmap.JSON{"zh-CN": "描述"},
				Content:         jsonmap.JSON{"zh-CN": "内容"},
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
						SpecValues:  jsonmap.JSON{"name": "A"},
						PriceAmount: "10.00",
						IsActive:    true,
					},
				},
			},
		})
	}))
	defer server.Close()

	connService := siteconnectionapp.NewService(siteconnectiongormstore.New(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(siteconnectionapp.CreateInput{
		Name:      "upstream-a",
		BaseURL:   server.URL,
		ApiKey:    "test-key",
		ApiSecret: "test-secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	svc, err := newTestMappingService(
		mappinggormstore.NewMappingStore(db),
		&failingSKUMappingRepo{err: errors.New("inject sku mapping failure")},
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
		categoryRepo,
		connService,
		noopLocalMediaRecorder{},
	)
	if err != nil {
		t.Fatalf("create product mapping service: %v", err)
	}

	if _, err := svc.ImportUpstreamProduct(conn.ID, 101, 1, "rollback-slug"); err == nil {
		t.Fatalf("expected import upstream product to fail")
	}

	var productCount int64
	if err := db.Model(&productdomain.Product{}).Count(&productCount).Error; err != nil {
		t.Fatalf("count products failed: %v", err)
	}
	if productCount != 0 {
		t.Fatalf("expected product rollback, got %d products", productCount)
	}

	var skuCount int64
	if err := db.Model(&productdomain.ProductSKU{}).Count(&skuCount).Error; err != nil {
		t.Fatalf("count product skus failed: %v", err)
	}
	if skuCount != 0 {
		t.Fatalf("expected sku rollback, got %d skus", skuCount)
	}

	var mappingCount int64
	if err := db.Model(&mappingdomain.Mapping{}).Count(&mappingCount).Error; err != nil {
		t.Fatalf("count product mappings failed: %v", err)
	}
	if mappingCount != 0 {
		t.Fatalf("expected mapping rollback, got %d mappings", mappingCount)
	}
}

// setupMappingWithUpstreamHandler 准备一份本地映射 + 启动可定制响应的上游 httptest server
func setupMappingWithUpstreamHandler(t *testing.T, dsn string, handler http.HandlerFunc) (*mappingapp.Service, *gorm.DB, *mappingdomain.Mapping, func()) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	server := httptest.NewServer(handler)

	categoryRepo := categorygormstore.NewCategoryStore(db)
	if err := categoryRepo.Create(&categorydomain.Category{Slug: "c", NameJSON: jsonmap.JSON{"zh-CN": "C"}}); err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	productRepo := productgormstore.NewProductStore(db)
	product := productdomain.Product{
		CategoryID:      1,
		Slug:            "p",
		TitleJSON:       jsonmap.JSON{"zh-CN": "P"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsActive:        true,
		IsMapped:        true,
	}
	if err := productRepo.Create(&product); err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	skuRepo := productgormstore.NewSKUStore(db)
	sku := productdomain.ProductSKU{ProductID: product.ID, SKUCode: "SKU-A", PriceAmount: money.FromDecimal(decimal.NewFromInt(10)), IsActive: true}
	if err := skuRepo.Create(&sku); err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	connService := siteconnectionapp.NewService(siteconnectiongormstore.New(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(siteconnectionapp.CreateInput{
		Name:      "upstream",
		BaseURL:   server.URL,
		ApiKey:    "k",
		ApiSecret: "s",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	mappingRepo := mappinggormstore.NewMappingStore(db)
	skuMappingRepo := mappinggormstore.NewSKUMappingStore(db)
	mapping := &mappingdomain.Mapping{
		ConnectionID:      conn.ID,
		LocalProductID:    product.ID,
		UpstreamProductID: 101,
		IsActive:          true,
		UpstreamStatus:    mappingdomain.UpstreamStatusActive,
	}
	if err := mappingRepo.Create(mapping); err != nil {
		t.Fatalf("create mapping failed: %v", err)
	}
	if err := skuMappingRepo.Create(&mappingdomain.SKUMapping{
		ProductMappingID: mapping.ID,
		LocalSKUID:       sku.ID,
		UpstreamSKUID:    201,
		UpstreamIsActive: true,
		UpstreamStock:    100,
	}); err != nil {
		t.Fatalf("create sku mapping failed: %v", err)
	}

	svc, err := newTestMappingService(mappingRepo, skuMappingRepo, productRepo, skuRepo, categoryRepo, connService, noopLocalMediaRecorder{})
	if err != nil {
		t.Fatalf("create product mapping service: %v", err)
	}
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

	var got mappingdomain.Mapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != mappingdomain.UpstreamStatusDeleted {
		t.Fatalf("expected upstream_status=deleted, got %q", got.UpstreamStatus)
	}
	if got.IsActive {
		t.Fatalf("expected mapping to be deactivated for deleted upstream")
	}

	var product productdomain.Product
	if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if product.IsActive {
		t.Fatalf("expected local product to be deactivated")
	}

	var skuMapping mappingdomain.SKUMapping
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

	var got mappingdomain.Mapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != mappingdomain.UpstreamStatusInactive {
		t.Fatalf("expected upstream_status=inactive, got %q", got.UpstreamStatus)
	}
	if !got.IsActive {
		t.Fatalf("expected mapping to remain active for inactive upstream (only deleted should auto-disable)")
	}

	var product productdomain.Product
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
					Title:           jsonmap.JSON{"zh-CN": "测试"},
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

	localWholesalePrices := productdomain.WholesalePriceTiers{
		{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
	}
	if err := db.Model(&productdomain.Product{}).
		Where("id = ?", mapping.LocalProductID).
		Update("wholesale_prices", localWholesalePrices).Error; err != nil {
		t.Fatalf("seed local wholesale prices failed: %v", err)
	}

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var product productdomain.Product
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
					Title:           jsonmap.JSON{"zh-CN": "测试"},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", PriceAmount: "10.00", IsActive: true, StockQuantity: 100},
					},
					WholesalePrices: productdomain.WholesalePriceTiers{
						{SKUID: 201, MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(8))},
					},
				},
			})
		},
	)
	defer cleanup()

	var localSKU productdomain.ProductSKU
	if err := db.Where("product_id = ? AND sku_code = ?", mapping.LocalProductID, "SKU-A").First(&localSKU).Error; err != nil {
		t.Fatalf("load local sku failed: %v", err)
	}
	if localSKU.ID == 201 {
		t.Fatalf("test setup invalid: local sku id should differ from upstream id")
	}

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var product productdomain.Product
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

	if err := svc.SyncConnectionStock(mapping.ConnectionID, []mappingdomain.Mapping{*mapping}, 50, 200); err != nil {
		t.Fatalf("syncConnectionStock returned error: %v", err)
	}

	var got mappingdomain.Mapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != mappingdomain.UpstreamStatusDeleted {
		t.Fatalf("expected upstream_status=deleted, got %q", got.UpstreamStatus)
	}
	if got.IsActive {
		t.Fatalf("expected mapping to be deactivated when upstream marks deleted")
	}

	var product productdomain.Product
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

	if err := svc.SyncConnectionStock(mapping.ConnectionID, []mappingdomain.Mapping{*mapping}, 50, 200); err != nil {
		t.Fatalf("syncConnectionStock returned error: %v", err)
	}

	var got mappingdomain.Mapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != mappingdomain.UpstreamStatusActive {
		t.Fatalf("legacy upstream missing must not change status, got %q", got.UpstreamStatus)
	}
	if !got.IsActive {
		t.Fatalf("legacy upstream missing must not deactivate mapping")
	}

	var product productdomain.Product
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
					{ID: 999, Title: jsonmap.JSON{"zh-CN": "其他商品"}, PriceAmount: "1.00", IsActive: true},
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

	if err := svc.SyncConnectionStock(mapping.ConnectionID, []mappingdomain.Mapping{*mapping}, 50, 200); err != nil {
		t.Fatalf("syncConnectionStock returned error: %v", err)
	}

	var got mappingdomain.Mapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != mappingdomain.UpstreamStatusActive {
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

	var sku productdomain.ProductSKU
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
				Title:           jsonmap.JSON{"zh-CN": "测试"},
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
	if err := db.Model(&mappingdomain.SKUMapping{}).Where("upstream_sku_id = ?", 201).
		Update("upstream_stock", 0).Error; err != nil {
		t.Fatalf("reset stock failed: %v", err)
	}

	var sku productdomain.ProductSKU
	if err := db.First(&sku).Error; err != nil {
		t.Fatalf("load sku failed: %v", err)
	}

	err := svc.EnsureUpstreamStockForOrder(sku.ID, 1)
	if !errors.Is(err, mappingcontract.ErrUpstreamStockInsufficient) {
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

	if err := db.Model(&mappingdomain.SKUMapping{}).Where("upstream_sku_id = ?", 201).
		Update("upstream_stock", 0).Error; err != nil {
		t.Fatalf("reset stock failed: %v", err)
	}

	var sku productdomain.ProductSKU
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
					Title:           jsonmap.JSON{"zh-CN": "P"},
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
	if err := db.Model(&mappingdomain.Mapping{}).Where("id = ?", mapping.ID).
		Update("upstream_status", mappingdomain.UpstreamStatusInactive).Error; err != nil {
		t.Fatalf("preset inactive failed: %v", err)
	}

	if err := svc.SyncProduct(mapping.ID); err != nil {
		t.Fatalf("SyncProduct returned error: %v", err)
	}

	var got mappingdomain.Mapping
	if err := db.First(&got, mapping.ID).Error; err != nil {
		t.Fatalf("reload mapping failed: %v", err)
	}
	if got.UpstreamStatus != mappingdomain.UpstreamStatusActive {
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
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	categoryRepo := categorygormstore.NewCategoryStore(db)
	if err := categoryRepo.Create(&categorydomain.Category{Slug: "c", NameJSON: jsonmap.JSON{"zh-CN": "C"}}); err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"product": upstream.UpstreamProduct{
				ID:          202,
				Title:       jsonmap.JSON{"zh-CN": "已下架商品"},
				PriceAmount: "10.00",
				IsActive:    false,
			},
		})
	}))
	defer server.Close()

	connService := siteconnectionapp.NewService(siteconnectiongormstore.New(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(siteconnectionapp.CreateInput{
		Name: "u", BaseURL: server.URL, ApiKey: "k", ApiSecret: "s",
		Protocol: constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	svc, err := newTestMappingService(
		mappinggormstore.NewMappingStore(db),
		mappinggormstore.NewSKUMappingStore(db),
		productgormstore.NewProductStore(db),
		productgormstore.NewSKUStore(db),
		categoryRepo,
		connService,
		noopLocalMediaRecorder{},
	)
	if err != nil {
		t.Fatalf("create product mapping service: %v", err)
	}

	_, importErr := svc.ImportUpstreamProduct(conn.ID, 202, 1, "")
	if !errors.Is(importErr, mappingcontract.ErrUpstreamProductNotFound) {
		t.Fatalf("expected ErrUpstreamProductNotFound for inactive upstream product, got %v", importErr)
	}

	var productCount int64
	if err := db.Model(&productdomain.Product{}).Count(&productCount).Error; err != nil {
		t.Fatalf("count products failed: %v", err)
	}
	if productCount != 0 {
		t.Fatalf("expected no local product created when import rejected, got %d", productCount)
	}
}

func TestImportUpstreamProductPrefersUpstreamSlug(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:import_upstream_slug?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&siteconnectiondomain.Connection{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	categoryRepo := categorygormstore.NewCategoryStore(db)
	if err := categoryRepo.Create(&categorydomain.Category{Slug: "c", NameJSON: jsonmap.JSON{"zh-CN": "C"}}); err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	// 已软删除的商品仍占用唯一索引，导入必须避开
	if err := db.Exec("INSERT INTO products (category_id, slug, title_json, price_amount, deleted_at) VALUES (1, 'taken', '{}', 0, CURRENT_TIMESTAMP)").Error; err != nil {
		t.Fatalf("seed deleted product failed: %v", err)
	}

	upstreamSlugs := map[string]string{
		"/api/v1/upstream/products/1": "telegram-asia-iq964",
		"/api/v1/upstream/products/2": "taken",
		"/api/v1/upstream/products/3": "Bad Slug!",
		"/api/v1/upstream/products/4": "",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/v1/upstream/products/"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"product": upstream.UpstreamProduct{
				ID:          uint(id),
				Slug:        upstreamSlugs[r.URL.Path],
				Title:       jsonmap.JSON{"zh-CN": "P"},
				PriceAmount: "10.00",
				IsActive:    true,
			},
		})
	}))
	defer server.Close()

	connService := siteconnectionapp.NewService(siteconnectiongormstore.New(db), "test-secret-key", t.TempDir())
	conn, err := connService.Create(siteconnectionapp.CreateInput{
		Name: "u", BaseURL: server.URL, ApiKey: "k", ApiSecret: "s",
		Protocol: constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection failed: %v", err)
	}
	productRepo := productgormstore.NewProductStore(db)
	svc, err := newTestMappingService(
		mappinggormstore.NewMappingStore(db),
		mappinggormstore.NewSKUMappingStore(db),
		productRepo,
		productgormstore.NewSKUStore(db),
		categoryRepo,
		connService,
		noopLocalMediaRecorder{},
	)
	if err != nil {
		t.Fatalf("create product mapping service: %v", err)
	}

	cases := []struct {
		upstreamID uint
		want       string
	}{
		{1, "telegram-asia-iq964"},
		{2, "taken-2"},
		{3, ""}, // 非法 slug → 回退自动生成
		{4, ""}, // 未提供 slug → 回退自动生成
	}
	for _, tc := range cases {
		mapping, err := svc.ImportUpstreamProduct(conn.ID, tc.upstreamID, 1, "")
		if err != nil {
			t.Fatalf("import upstream %d failed: %v", tc.upstreamID, err)
		}
		var product productdomain.Product
		if err := db.First(&product, mapping.LocalProductID).Error; err != nil {
			t.Fatalf("load product failed: %v", err)
		}
		if tc.want != "" && product.Slug != tc.want {
			t.Fatalf("upstream %d: expected slug %q, got %q", tc.upstreamID, tc.want, product.Slug)
		}
		if tc.want == "" && !strings.HasPrefix(product.Slug, "upstream-") {
			t.Fatalf("upstream %d: expected fallback slug, got %q", tc.upstreamID, product.Slug)
		}
	}
}
