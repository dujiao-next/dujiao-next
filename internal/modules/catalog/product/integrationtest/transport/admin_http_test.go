package integrationtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	mappingdomain "github.com/dujiao-next/internal/modules/catalog/mapping/domain"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	cardsecretgormstore "github.com/dujiao-next/internal/modules/cardsecret/infrastructure/gormstore"
	cartdomain "github.com/dujiao-next/internal/modules/cart/domain"
	cartgormstore "github.com/dujiao-next/internal/modules/cart/infrastructure/gormstore"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	mappinggormstore "github.com/dujiao-next/internal/modules/catalog/mapping/infrastructure/gormstore"
	productapplication "github.com/dujiao-next/internal/modules/catalog/product/application"
	productadmin "github.com/dujiao-next/internal/modules/catalog/product/application/admin"
	productwrite "github.com/dujiao-next/internal/modules/catalog/product/application/write"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type productHandlerFacade struct {
	*productapplication.Service
	*productadmin.AdminService
	*productwrite.WriteService
}

type productWriteUoW struct {
	products    *productgormstore.ProductStore
	skus        *productgormstore.SKUStore
	cardSecrets *cardsecretgormstore.Store
}

func (unit *productWriteUoW) WithinTransaction(fn func(productwrite.TransactionRepositories) error) error {
	if fn == nil {
		return nil
	}
	return unit.products.Transaction(func(tx *gorm.DB) error {
		return fn(productwrite.TransactionRepositories{
			Products:    unit.products.BindTx(tx),
			SKUs:        unit.skus.BindTx(tx),
			CardSecrets: unit.cardSecrets.BindTx(tx),
		})
	})
}

type productAdminUoW struct {
	products          *productgormstore.ProductStore
	productSKUs       *productgormstore.SKUStore
	cardSecrets       *cardsecretgormstore.Store
	cardSecretBatches *cardsecretgormstore.BatchStore
	memberLevelPrices *memberlevelgormstore.PriceStore
	carts             *cartgormstore.Store
	productMappings   *mappinggormstore.MappingStore
}

func (unit *productAdminUoW) WithinTransaction(fn func(productadmin.DeleteRepositories) error) error {
	if fn == nil {
		return nil
	}
	return unit.products.Transaction(func(tx *gorm.DB) error {
		return fn(productadmin.DeleteRepositories{
			Products:          unit.products.BindTx(tx),
			CardSecrets:       unit.cardSecrets.BindTx(tx),
			CardSecretBatches: unit.cardSecretBatches.BindTx(tx),
			SKUs:              unit.productSKUs.BindTx(tx),
			MemberLevelPrices: memberlevelgormstore.NewPriceStore(tx),
			Carts:             unit.carts.WithTx(tx),
			ProductMappings:   unit.productMappings.WithTx(tx),
		})
	})
}

type orderHistoryStore struct{ db *gorm.DB }

func (s *orderHistoryStore) CountOrderItemsByProduct(productID uint) (int64, error) {
	var count int64
	err := s.db.Model(&orderdomain.OrderItem{}).Where("product_id = ?", productID).Count(&count).Error
	return count, err
}

type paymentChannelStore struct{ db *gorm.DB }

func (s *paymentChannelStore) ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []paymentdomain.PaymentChannel
	err := s.db.Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

func setupAdminProductHandlerTest(t *testing.T) (*producthttp.AdminProductHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:admin_product_handler_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&cardsecretdomain.Secret{},
		&cardsecretdomain.Batch{},
		&memberleveldomain.MemberLevelPrice{},
		&cartdomain.Item{},
		&mappingdomain.Mapping{},
		&mappingdomain.SKUMapping{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&paymentdomain.PaymentChannel{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	productStore := productgormstore.NewProductStore(db)
	skuStore := productgormstore.NewSKUStore(db)
	cardSecretStore := cardsecretgormstore.New(db)
	cardSecretBatchStore := cardsecretgormstore.NewBatch(db)
	categoryStore := categorygormstore.NewCategoryStore(db)
	memberLevelPriceStore := memberlevelgormstore.NewPriceStore(db)
	mappingStore := mappinggormstore.NewMappingStore(db)
	skuMappingStore := mappinggormstore.NewSKUMappingStore(db)
	cartStore := cartgormstore.New(db)

	facade := &productHandlerFacade{
		Service: productapplication.NewService(productapplication.Options{
			Products:   productStore,
			Categories: categoryStore,
			Stock:      cardSecretStore,
		}),
		AdminService: productadmin.NewAdminService(productadmin.Options{
			Products:    productStore,
			Categories:  categoryStore,
			CardSecrets: cardSecretStore,
			Orders:      &orderHistoryStore{db: db},
			Transactions: &productAdminUoW{
				products:          productStore,
				productSKUs:       skuStore,
				cardSecrets:       cardSecretStore,
				cardSecretBatches: cardSecretBatchStore,
				memberLevelPrices: memberLevelPriceStore,
				carts:             cartStore,
				productMappings:   mappingStore,
			},
		}),
		WriteService: productwrite.NewWriteService(productwrite.Options{
			Products:        productStore,
			SKUs:            skuStore,
			Categories:      categoryStore,
			PaymentChannels: &paymentChannelStore{db: db},
			Transactions: &productWriteUoW{
				products:    productStore,
				skus:        skuStore,
				cardSecrets: cardSecretStore,
			},
		}),
	}

	h := producthttp.NewAdminProductHandler(facade, facade, facade, nil, mappingStore, skuMappingStore)
	return h, db
}

func TestBatchUpdateProductStatusReturnsFailureReasons(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	product := productdomain.Product{
		CategoryID:      0,
		Slug:            "batch-uncategorized-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "batch-uncategorized-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
		IsActive:        false,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create uncategorized product failed: %v", err)
	}

	body := fmt.Sprintf(`{"ids":[%d],"is_active":true}`, product.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/batch-status", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req

	h.BatchUpdateProductStatus(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			Total        int `json:"total"`
			SuccessCount int `json:"success_count"`
			FailedItems  []struct {
				ID        uint   `json:"id"`
				ErrorCode string `json:"error_code"`
				Message   string `json:"message"`
			} `json:"failed_items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if resp.Data.Total != 1 || resp.Data.SuccessCount != 0 {
		t.Fatalf("unexpected counts: total=%d success=%d", resp.Data.Total, resp.Data.SuccessCount)
	}
	if len(resp.Data.FailedItems) != 1 {
		t.Fatalf("expected one failed item, got %+v", resp.Data.FailedItems)
	}
	if resp.Data.FailedItems[0].ID != product.ID {
		t.Fatalf("expected failed product id %d, got %d", product.ID, resp.Data.FailedItems[0].ID)
	}
	if resp.Data.FailedItems[0].ErrorCode != "product_category_invalid" {
		t.Fatalf("expected product_category_invalid, got %q", resp.Data.FailedItems[0].ErrorCode)
	}
}

func TestBatchUpdateProductStockDisplay(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	product := productdomain.Product{
		Slug:            "batch-stock-display-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "batch-stock-display-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/batch-stock-display", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		h.BatchUpdateProductStockDisplay(c)
		var resp struct {
			StatusCode int `json:"status_code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
		}
		return resp.StatusCode
	}

	if code := post(fmt.Sprintf(`{"ids":[%d],"stock_display_mode":"bogus"}`, product.ID)); code == 0 {
		t.Fatal("expected invalid mode to be rejected")
	}
	if code := post(fmt.Sprintf(`{"ids":[%d],"stock_display_mode":"range"}`, product.ID)); code != 0 {
		t.Fatalf("expected success, got status_code %d", code)
	}
	var reloaded productdomain.Product
	if err := db.First(&reloaded, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if reloaded.StockDisplayMode != constants.ProductStockDisplayRange {
		t.Fatalf("expected range, got %q", reloaded.StockDisplayMode)
	}
}

func TestBatchUpdateProductPurchaseType(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	product := productdomain.Product{
		Slug:            "batch-purchase-type-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "batch-purchase-type-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeUpstream,
		IsMapped:        true,
		PurchaseType:    constants.ProductPurchaseMember,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	post := func(body string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products/batch-purchase-type", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		h.BatchUpdateProductPurchaseType(c)
		var resp struct {
			StatusCode int `json:"status_code"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
		}
		return resp.StatusCode
	}

	if code := post(fmt.Sprintf(`{"ids":[%d],"purchase_type":"bogus"}`, product.ID)); code == 0 {
		t.Fatal("expected invalid purchase type to be rejected")
	}
	if code := post(fmt.Sprintf(`{"ids":[%d],"purchase_type":"guest"}`, product.ID)); code != 0 {
		t.Fatalf("expected success, got status_code %d", code)
	}
	var reloaded productdomain.Product
	if err := db.First(&reloaded, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if reloaded.PurchaseType != constants.ProductPurchaseGuest {
		t.Fatalf("expected guest, got %q", reloaded.PurchaseType)
	}
}

func TestUpdateProductWholesalePricesHandlerUpdatesTiers(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	product := productdomain.Product{
		CategoryID:  1,
		Slug:        "handler-wholesale-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "handler-wholesale-product"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	body := `{"wholesale_prices":[{"min_quantity":10,"unit_price":70},{"min_quantity":5,"unit_price":80}]}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/1/wholesale-prices", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}

	h.UpdateProductWholesalePrices(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data productdomain.Product `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if len(resp.Data.WholesalePrices) != 2 {
		t.Fatalf("expected 2 wholesale tiers, got %+v", resp.Data.WholesalePrices)
	}
	if resp.Data.WholesalePrices[0].MinQuantity != 5 || resp.Data.WholesalePrices[0].UnitPrice.String() != "80.00" {
		t.Fatalf("expected sorted first tier min=5 price=80.00, got %+v", resp.Data.WholesalePrices[0])
	}
}

func TestUpdateProductWholesalePricesHandlerAllowsClear(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	product := productdomain.Product{
		CategoryID:  1,
		Slug:        "handler-wholesale-clear",
		TitleJSON:   jsonmap.JSON{"zh-CN": "handler-wholesale-clear"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		WholesalePrices: productdomain.WholesalePriceTiers{
			{MinQuantity: 5, UnitPrice: money.FromDecimal(decimal.NewFromInt(80))},
		},
		IsActive: true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/1/wholesale-prices", strings.NewReader(`{"wholesale_prices":[]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}

	h.UpdateProductWholesalePrices(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
	}
	var got productdomain.Product
	if err := db.First(&got, product.ID).Error; err != nil {
		t.Fatalf("reload product failed: %v", err)
	}
	if len(got.WholesalePrices) != 0 {
		t.Fatalf("expected wholesale prices cleared, got %+v", got.WholesalePrices)
	}
}

func TestUpdateProductWholesalePricesHandlerRejectsInvalidTier(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	product := productdomain.Product{
		CategoryID:  1,
		Slug:        "handler-wholesale-invalid",
		TitleJSON:   jsonmap.JSON{"zh-CN": "handler-wholesale-invalid"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/1/wholesale-prices", strings.NewReader(`{"wholesale_prices":[{"min_quantity":0,"unit_price":80}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}

	h.UpdateProductWholesalePrices(c)

	if w.Code != http.StatusOK {
		t.Fatalf("project response wrapper should still return HTTP 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		StatusCode int    `json:"status_code"`
		Msg        string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected wholesale invalid response, got status_code=%d body=%s", resp.StatusCode, w.Body.String())
	}
}

func TestUpdateProductWholesalePricesHandlerReturnsNotFound(t *testing.T) {
	h, _ := setupAdminProductHandlerTest(t)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/999999/wholesale-prices", strings.NewReader(`{"wholesale_prices":[{"min_quantity":5,"unit_price":80}]}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: "999999"}}

	h.UpdateProductWholesalePrices(c)

	if w.Code != http.StatusOK {
		t.Fatalf("project response wrapper should still return HTTP 200, got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected product not found response, got body=%s", w.Body.String())
	}
}

// 后台商品列表的上架状态筛选：is_active=1 只看已上架，is_active=0 只看已下架。
func TestAdminProductListFiltersByActiveState(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	seed := func(slug string, isActive bool) productdomain.Product {
		row := productdomain.Product{
			CategoryID:       1,
			Slug:             slug,
			TitleJSON:        jsonmap.JSON{"zh-CN": "上架状态筛选商品"},
			PriceAmount:      money.FromDecimal(decimal.NewFromInt(10)),
			FulfillmentType:  constants.FulfillmentTypeManual,
			ManualStockTotal: 5,
			IsActive:         isActive,
		}
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("create product %s failed: %v", slug, err)
		}
		return row
	}
	active := seed("api-state-active", true)
	inactive := seed("api-state-inactive", false)

	listSlugs := func(query string) map[string]bool {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/products?"+query, nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req

		h.GetAdminProducts(c)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200 for %q, got %d body=%s", query, w.Code, w.Body.String())
		}
		var resp struct {
			Data []productdomain.Product `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response failed: %v body=%s", err, w.Body.String())
		}
		got := make(map[string]bool, len(resp.Data))
		for _, row := range resp.Data {
			got[row.Slug] = true
		}
		return got
	}

	got := listSlugs("is_active=1")
	if !got[active.Slug] || got[inactive.Slug] {
		t.Fatalf("is_active=1 should return only active rows, got %+v", got)
	}

	got = listSlugs("is_active=0")
	if !got[inactive.Slug] || got[active.Slug] {
		t.Fatalf("is_active=0 should return only inactive rows, got %+v", got)
	}

	got = listSlugs("is_active=all")
	if !got[active.Slug] || !got[inactive.Slug] {
		t.Fatalf("is_active=all should return both rows, got %+v", got)
	}

	// 与其他条件联动：is_active=0 叠加 search，仍应 AND 生效
	got = listSlugs("is_active=0&search=" + url.QueryEscape("上架状态筛选"))
	if !got[inactive.Slug] || got[active.Slug] {
		t.Fatalf("is_active=0 + search should still exclude active rows, got %+v", got)
	}

	got = listSlugs("is_active=0&search=" + url.QueryEscape("绝不匹配的关键词"))
	if len(got) != 0 {
		t.Fatalf("is_active=0 + non-matching search should be empty, got %+v", got)
	}
}

// 返利比例字段遵循三态语义：未携带保留原值，显式 null 清除为继承，数值表示覆盖。
// 旧后台页面与只改价格的脚本不会发送该字段，此时不能把「不参与返利」的商品改成发佣金。
func TestUpdateProductAffiliateRateTriState(t *testing.T) {
	h, db := setupAdminProductHandlerTest(t)

	category := categorydomain.Category{
		Slug:     "affiliate-tri-state-category",
		NameJSON: jsonmap.JSON{"zh-CN": "affiliate-tri-state-category"},
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	rate := money.FromDecimal(decimal.Zero)
	product := productdomain.Product{
		CategoryID:           category.ID,
		Slug:                 "affiliate-tri-state",
		TitleJSON:            jsonmap.JSON{"zh-CN": "affiliate-tri-state"},
		PriceAmount:          money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType:      constants.FulfillmentTypeManual,
		PurchaseType:         constants.ProductPurchaseMember,
		ManualStockTotal:     0,
		IsActive:             true,
		AffiliateRatePercent: &rate,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	update := func(t *testing.T, extra string) *productdomain.Product {
		t.Helper()
		body := fmt.Sprintf(`{"category_id":%d,"slug":"affiliate-tri-state","title":{"zh-CN":"affiliate-tri-state"},"price_amount":10,"purchase_type":"member","fulfillment_type":"manual","manual_stock_total":0,%s"sort_order":0}`, category.ID, extra)
		req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = req
		c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}

		h.UpdateProduct(c)
		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d body=%s", w.Code, w.Body.String())
		}
		reloaded, err := productgormstore.NewProductStore(db).GetByID(fmt.Sprintf("%d", product.ID))
		if err != nil {
			t.Fatalf("reload product failed: %v", err)
		}
		return reloaded
	}

	// 未携带字段：保留原来的 0（不参与返利）。
	if got := update(t, ""); got.AffiliateRatePercent == nil || !got.AffiliateRatePercent.IsZero() {
		t.Fatalf("missing field should keep 0, got %+v", got.AffiliateRatePercent)
	}

	// 数值：覆盖为自定义比例。
	if got := update(t, `"affiliate_rate_percent":25,`); got.AffiliateRatePercent == nil || !got.AffiliateRatePercent.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("explicit rate should be stored, got %+v", got.AffiliateRatePercent)
	}

	// 显式 null：清除为继承。
	if got := update(t, `"affiliate_rate_percent":null,`); got.AffiliateRatePercent != nil {
		t.Fatalf("explicit null should clear the override, got %+v", got.AffiliateRatePercent)
	}

	// 旧开关 is_affiliate_enabled：未刷新的旧后台页面只会发它，false 必须真的关掉返利。
	if got := update(t, `"is_affiliate_enabled":false,`); got.AffiliateRatePercent == nil || !got.AffiliateRatePercent.IsZero() {
		t.Fatalf("legacy false should store 0, got %+v", got.AffiliateRatePercent)
	}
	if got := update(t, `"is_affiliate_enabled":true,`); got.AffiliateRatePercent != nil {
		t.Fatalf("legacy true should follow the site-wide rate, got %+v", got.AffiliateRatePercent)
	}
	// 两个字段同时出现时以新字段为准。
	if got := update(t, `"is_affiliate_enabled":false,"affiliate_rate_percent":25,`); got.AffiliateRatePercent == nil || !got.AffiliateRatePercent.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("new field should win over the legacy switch, got %+v", got.AffiliateRatePercent)
	}

	// 超出范围仍返回 400 而不是 500。
	body := fmt.Sprintf(`{"category_id":%d,"slug":"affiliate-tri-state","title":{"zh-CN":"affiliate-tri-state"},"price_amount":10,"purchase_type":"member","fulfillment_type":"manual","manual_stock_total":0,"affiliate_rate_percent":150,"sort_order":0}`, category.ID)
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/products/%d", product.ID), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Params = gin.Params{{Key: "id", Value: fmt.Sprintf("%d", product.ID)}}
	h.UpdateProduct(c)
	// 接口统一用响应体里的 status_code 表达业务错误码。
	var errResp struct {
		StatusCode int    `json:"status_code"`
		Msg        string `json:"msg"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("decode error response failed: %v body=%s", err, w.Body.String())
	}
	if errResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("out-of-range rate should return 400, got %d body=%s", errResp.StatusCode, w.Body.String())
	}
	if !strings.Contains(errResp.Msg, "0-100") {
		t.Fatalf("expected affiliate rate error message, got %q", errResp.Msg)
	}
}
