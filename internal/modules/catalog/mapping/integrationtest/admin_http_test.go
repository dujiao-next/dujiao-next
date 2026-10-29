package integrationtest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappingapp "github.com/dujiao-next/internal/modules/catalog/mapping/application"
	mappingcontract "github.com/dujiao-next/internal/modules/catalog/mapping/contract"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	mappinghttp "github.com/dujiao-next/internal/modules/catalog/mapping/transport/http"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/upstream"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type adminNoopLocalMediaRecorder struct{}

func (adminNoopLocalMediaRecorder) RecordLocalFile(context.Context, string, string) {}

type mappingConnectionProvider struct {
	db      *gorm.DB
	adapter upstream.Adapter
}

func (p *mappingConnectionProvider) GetByID(id uint) (*siteconnectiondomain.Connection, error) {
	var conn siteconnectiondomain.Connection
	if err := p.db.First(&conn, id).Error; err != nil {
		return nil, err
	}
	return &conn, nil
}

func (p *mappingConnectionProvider) GetAdapter(conn *siteconnectiondomain.Connection) (upstream.Adapter, error) {
	return p.adapter, nil
}

type mappingImportUoW struct {
	products    *productgormstore.ProductStore
	skus        *productgormstore.SKUStore
	mappings    *mappinggormstore.MappingStore
	skuMappings *mappinggormstore.SKUMappingStore
}

func (unit *mappingImportUoW) WithinTransaction(fn func(mappingcontract.ImportRepositories) error) error {
	if fn == nil {
		return nil
	}
	return unit.products.Transaction(func(tx *gorm.DB) error {
		return fn(mappingcontract.ImportRepositories{
			Products:    unit.products.BindTx(tx),
			SKUs:        unit.skus.BindTx(tx),
			Mappings:    unit.mappings.WithTx(tx),
			SKUMappings: unit.skuMappings.WithTx(tx),
		})
	})
}

func setupAdminHandlerTest(t *testing.T, upstreamHandler http.HandlerFunc) (*mappinghttp.AdminHandler, *gorm.DB, uint, func()) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:admin_product_mapping_handler_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
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

	server := httptest.NewServer(upstreamHandler)
	categoryRepo := categorygormstore.NewCategoryStore(db)
	categoryService := categoryapp.NewService(categoryRepo)

	conn := siteconnectiondomain.Connection{
		Name:      "upstream",
		BaseURL:   server.URL,
		ApiKey:    "test-key",
		ApiSecret: "test-secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
		Status:    constants.ConnectionStatusActive,
	}
	if err := db.Create(&conn).Error; err != nil {
		t.Fatalf("create connection failed: %v", err)
	}

	adapter, err := upstream.NewAdapter(&conn, t.TempDir())
	if err != nil {
		t.Fatalf("create upstream adapter failed: %v", err)
	}

	productStore := productgormstore.NewProductStore(db)
	skuStore := productgormstore.NewSKUStore(db)
	mappingStore := mappinggormstore.NewMappingStore(db)
	skuMappingStore := mappinggormstore.NewSKUMappingStore(db)

	mappingService, err := mappingapp.NewService(mappingapp.Options{
		Mappings:    mappingStore,
		SKUMappings: skuMappingStore,
		Products:    productStore,
		SKUs:        skuStore,
		Categories:  categoryRepo,
		Connections: &mappingConnectionProvider{db: db, adapter: adapter},
		Media:       adminNoopLocalMediaRecorder{},
		Transactions: &mappingImportUoW{
			products:    productStore,
			skus:        skuStore,
			mappings:    mappingStore,
			skuMappings: skuMappingStore,
		},
	})
	if err != nil {
		t.Fatalf("create product mapping service: %v", err)
	}
	mappingService.SetCategoryCreator(categoryService)

	h := mappinghttp.NewAdminHandler(mappingService)
	return h, db, conn.ID, server.Close
}

func TestBatchImportUpstreamProductsAutoCreatesCategory(t *testing.T) {
	h, db, connID, cleanup := setupAdminHandlerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/upstream/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"categories": []upstream.UpstreamCategory{
					{ID: 9, Slug: "upstream-streaming", Name: jsonmap.JSON{"zh-CN": "流媒体"}},
				},
			})
		case "/api/v1/upstream/products/101":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:              101,
					CategoryID:      9,
					Title:           jsonmap.JSON{"zh-CN": "上游商品"},
					Description:     jsonmap.JSON{"zh-CN": "描述"},
					Content:         jsonmap.JSON{"zh-CN": "内容"},
					Images:          []string{},
					Tags:            []string{},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", SpecValues: jsonmap.JSON{"name": "A"}, PriceAmount: "10.00", IsActive: true},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	body := fmt.Sprintf(`{"connection_id":%d,"upstream_product_ids":[101],"auto_create_category":true}`, connID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-mappings/batch-import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BatchImportUpstreamProducts(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var imported productdomain.Product
	if err := db.First(&imported).Error; err != nil {
		t.Fatalf("load imported product failed: %v", err)
	}
	if imported.CategoryID == 0 {
		t.Fatalf("expected imported product to be assigned to auto-created category")
	}

	var category categorydomain.Category
	if err := db.First(&category, imported.CategoryID).Error; err != nil {
		t.Fatalf("load auto-created category failed: %v", err)
	}
	if category.Slug != "upstream-streaming" {
		t.Fatalf("expected auto-created category slug upstream-streaming, got %q", category.Slug)
	}
}

func TestBatchImportUpstreamProductsAutoCategoryRejectsUpstreamProductWithoutCategory(t *testing.T) {
	h, db, connID, cleanup := setupAdminHandlerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/upstream/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":         true,
				"categories": []upstream.UpstreamCategory{},
			})
		case "/api/v1/upstream/products/101":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:              101,
					CategoryID:      0,
					Title:           jsonmap.JSON{"zh-CN": "无分类上游商品"},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", SpecValues: jsonmap.JSON{"name": "A"}, PriceAmount: "10.00", IsActive: true},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	body := fmt.Sprintf(`{"connection_id":%d,"upstream_product_ids":[101],"auto_create_category":true}`, connID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-mappings/batch-import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BatchImportUpstreamProducts(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			SuccessCount int `json:"success_count"`
			Results      []struct {
				UpstreamProductID uint   `json:"upstream_product_id"`
				Success           bool   `json:"success"`
				Error             string `json:"error"`
			} `json:"results"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if resp.Data.SuccessCount != 0 || len(resp.Data.Results) != 1 || resp.Data.Results[0].Success {
		t.Fatalf("expected the category-less product to fail, got %+v", resp.Data)
	}
	if resp.Data.Results[0].Error != mappingcontract.ErrUpstreamProductNoCategory.Error() {
		t.Fatalf("unexpected error message %q", resp.Data.Results[0].Error)
	}

	var productCount, mappingCount int64
	if err := db.Model(&productdomain.Product{}).Count(&productCount).Error; err != nil {
		t.Fatalf("count products failed: %v", err)
	}
	if err := db.Model(&mappingdomain.Mapping{}).Count(&mappingCount).Error; err != nil {
		t.Fatalf("count mappings failed: %v", err)
	}
	if productCount != 0 || mappingCount != 0 {
		t.Fatalf("expected nothing imported, got products=%d mappings=%d", productCount, mappingCount)
	}
}

func TestBatchImportUpstreamProductsRestoresSoftDeletedAutoCategory(t *testing.T) {
	h, db, connID, cleanup := setupAdminHandlerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/upstream/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"categories": []upstream.UpstreamCategory{
					{ID: 9, Slug: "upstream-streaming", Name: jsonmap.JSON{"zh-CN": "流媒体"}},
				},
			})
		case "/api/v1/upstream/products/101":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID:              101,
					CategoryID:      9,
					Title:           jsonmap.JSON{"zh-CN": "上游商品"},
					PriceAmount:     "10.00",
					Currency:        "CNY",
					FulfillmentType: constants.FulfillmentTypeAuto,
					IsActive:        true,
					SKUs: []upstream.UpstreamSKU{
						{ID: 201, SKUCode: "SKU-A", SpecValues: jsonmap.JSON{"name": "A"}, PriceAmount: "10.00", IsActive: true},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	deletedCategory := categorydomain.Category{
		Slug:     "upstream-streaming",
		NameJSON: jsonmap.JSON{"zh-CN": "已删除分类"},
		IsActive: true,
	}
	if err := db.Create(&deletedCategory).Error; err != nil {
		t.Fatalf("create soft-delete target category failed: %v", err)
	}
	if err := categorygormstore.NewCategoryStore(db).Delete(fmt.Sprintf("%d", deletedCategory.ID)); err != nil {
		t.Fatalf("soft delete category failed: %v", err)
	}

	body := fmt.Sprintf(`{"connection_id":%d,"upstream_product_ids":[101],"auto_create_category":true}`, connID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-mappings/batch-import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BatchImportUpstreamProducts(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var imported productdomain.Product
	if err := db.First(&imported).Error; err != nil {
		t.Fatalf("load imported product failed: %v", err)
	}
	if imported.CategoryID != deletedCategory.ID {
		t.Fatalf("expected imported product category %d, got %d", deletedCategory.ID, imported.CategoryID)
	}

	var restored categorydomain.Category
	if err := db.First(&restored, deletedCategory.ID).Error; err != nil {
		t.Fatalf("expected category to be restored, got %v", err)
	}
	if restored.NameJSON["zh-CN"] != "流媒体" {
		t.Fatalf("expected restored category name to be refreshed, got %+v", restored.NameJSON)
	}
}

func TestBatchImportByCategoryUncategorizedGroup(t *testing.T) {
	newHandler := func(t *testing.T) (*mappinghttp.AdminHandler, *gorm.DB, uint, func()) {
		return setupAdminHandlerTest(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/api/v1/upstream/categories":
				_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "categories": []upstream.UpstreamCategory{}})
			case "/api/v1/upstream/products":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":       true,
					"items":    []upstream.UpstreamProduct{{ID: 101, CategoryID: 0, Title: jsonmap.JSON{"zh-CN": "无分类"}, IsActive: true}},
					"total":    1,
					"page":     1,
					"per_page": 50,
				})
			case "/api/v1/upstream/products/101":
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": true,
					"product": upstream.UpstreamProduct{
						ID: 101, CategoryID: 0, Title: jsonmap.JSON{"zh-CN": "无分类"},
						PriceAmount: "10.00", Currency: "CNY", FulfillmentType: constants.FulfillmentTypeAuto, IsActive: true,
						SKUs: []upstream.UpstreamSKU{{ID: 201, SKUCode: "SKU-A", SpecValues: jsonmap.JSON{"name": "A"}, PriceAmount: "10.00", IsActive: true}},
					},
				})
			default:
				http.NotFound(w, r)
			}
		})
	}
	call := func(h *mappinghttp.AdminHandler, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-mappings/batch-import-by-category", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		h.BatchImportByCategory(c)
		return w
	}

	t.Run("auto create without local category is rejected", func(t *testing.T) {
		h, db, connID, cleanup := newHandler(t)
		defer cleanup()
		w := call(h, fmt.Sprintf(`{"connection_id":%d,"upstream_category_id":0,"auto_create_category":true}`, connID))
		// 业务错误码在响应体 status_code 里，HTTP 状态码仍是 200
		var resp struct {
			StatusCode int `json:"status_code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
		}
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status_code 400, got %d body=%s", resp.StatusCode, w.Body.String())
		}
		var n int64
		db.Model(&productdomain.Product{}).Count(&n)
		if n != 0 {
			t.Fatalf("expected nothing imported, got %d products", n)
		}
	})

	t.Run("local category imports uncategorized products", func(t *testing.T) {
		h, db, connID, cleanup := newHandler(t)
		defer cleanup()
		local := categorydomain.Category{Slug: "local", NameJSON: jsonmap.JSON{"zh-CN": "本地"}, IsActive: true}
		if err := db.Create(&local).Error; err != nil {
			t.Fatalf("create category failed: %v", err)
		}
		w := call(h, fmt.Sprintf(`{"connection_id":%d,"upstream_category_id":0,"local_category_id":%d}`, connID, local.ID))
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
		}
		var p productdomain.Product
		if err := db.First(&p).Error; err != nil {
			t.Fatalf("load product failed: %v", err)
		}
		if p.CategoryID != local.ID {
			t.Fatalf("expected category %d, got %d", local.ID, p.CategoryID)
		}
	})
}

func TestBatchImportUsesCategoryMarkupWithParentInheritance(t *testing.T) {
	h, db, connID, cleanup := setupAdminHandlerTest(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/upstream/categories":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"categories": []upstream.UpstreamCategory{
					{ID: 9, Slug: "parent", Name: jsonmap.JSON{"zh-CN": "父"}},
					{ID: 10, ParentID: 9, Slug: "child", Name: jsonmap.JSON{"zh-CN": "子"}},
				},
			})
		case "/api/v1/upstream/products/101":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"product": upstream.UpstreamProduct{
					ID: 101, CategoryID: 10, Title: jsonmap.JSON{"zh-CN": "商品"},
					PriceAmount: "10.00", Currency: "CNY", FulfillmentType: constants.FulfillmentTypeAuto, IsActive: true,
					SKUs: []upstream.UpstreamSKU{{ID: 201, SKUCode: "SKU-A", SpecValues: jsonmap.JSON{"name": "A"}, PriceAmount: "10.00", IsActive: true}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	})
	defer cleanup()

	// 连接默认加价 10%，父分类 9 单独设置 50%；商品在子分类 10 下，应继承父分类的 50%。
	if err := db.Model(&siteconnectiondomain.Connection{}).Where("id = ?", connID).Updates(map[string]interface{}{
		"price_markup_percent": 10,
		"category_markups":     jsonmap.JSON{"9": float64(50)},
	}).Error; err != nil {
		t.Fatalf("update connection failed: %v", err)
	}

	body := fmt.Sprintf(`{"connection_id":%d,"upstream_product_ids":[101],"auto_create_category":true}`, connID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/product-mappings/batch-import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	h.BatchImportUpstreamProducts(c)
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}

	var sku productdomain.ProductSKU
	if err := db.First(&sku).Error; err != nil {
		t.Fatalf("load sku failed: %v", err)
	}
	if !sku.PriceAmount.Decimal.Equal(decimal.RequireFromString("15")) {
		t.Fatalf("expected inherited 50%% markup => 15.00, got %s", sku.PriceAmount.Decimal.String())
	}
	var mapping mappingdomain.Mapping
	if err := db.First(&mapping).Error; err != nil {
		t.Fatalf("load mapping failed: %v", err)
	}
	if mapping.UpstreamCategoryID != 10 {
		t.Fatalf("expected mapping upstream category 10, got %d", mapping.UpstreamCategoryID)
	}
}
