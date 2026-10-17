package integrationtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	categoryapp "github.com/dujiao-next/internal/modules/catalog/category/application"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	productapplication "github.com/dujiao-next/internal/modules/catalog/product/application"
	channeltransport "github.com/dujiao-next/internal/modules/channelapi/transport/http"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type channelCatalogTestResponse struct {
	StatusCode int            `json:"status_code"`
	Data       map[string]any `json:"data"`
}

func TestGetCategoriesIncludesParentIDAndVisibleParent(t *testing.T) {
	dsn := fmt.Sprintf("file:channel_catalog_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	parent := categorydomain.Category{
		Slug:     "games",
		NameJSON: jsonmap.JSON{"zh-CN": "游戏"},
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent category failed: %v", err)
	}
	child := categorydomain.Category{
		ParentID: parent.ID,
		Slug:     "steam",
		NameJSON: jsonmap.JSON{"zh-CN": "Steam"},
	}
	if err := db.Create(&child).Error; err != nil {
		t.Fatalf("create child category failed: %v", err)
	}
	hidden := categorydomain.Category{
		Slug:     "hidden",
		NameJSON: jsonmap.JSON{"zh-CN": "hidden"},
	}
	if err := db.Create(&hidden).Error; err != nil {
		t.Fatalf("create hidden category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:  child.ID,
		Slug:        "steam-product",
		TitleJSON:   jsonmap.JSON{"zh-CN": "Steam Product"},
		PriceAmount: money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	categoryRepo := categorygormstore.NewCategoryStore(db)
	handler := &channeltransport.Handler{Dependencies: channeltransport.Dependencies{
		CategoryRepo: categoryRepo, CategoryService: categoryapp.NewService(categoryRepo),
	}}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/channel/catalog/categories", handler.GetCategories)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/channel/catalog/categories?locale=zh-CN", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http 200, got %d", recorder.Code)
	}

	var payload channelCatalogTestResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	itemsValue, ok := payload.Data["items"].([]any)
	if !ok {
		t.Fatalf("expected items array, got %T", payload.Data["items"])
	}
	if len(itemsValue) != 2 {
		t.Fatalf("expected 2 visible categories, got %d", len(itemsValue))
	}

	itemsBySlug := make(map[string]map[string]any, len(itemsValue))
	for _, item := range itemsValue {
		row, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("expected item object, got %T", item)
		}
		slug, _ := row["slug"].(string)
		itemsBySlug[slug] = row
	}

	parentItem, ok := itemsBySlug["games"]
	if !ok {
		t.Fatalf("expected parent category to be returned")
	}
	if parentItem["parent_id"] != float64(0) {
		t.Fatalf("expected parent parent_id=0, got %v", parentItem["parent_id"])
	}
	if parentItem["product_count"] != float64(0) {
		t.Fatalf("expected parent product_count=0, got %v", parentItem["product_count"])
	}

	childItem, ok := itemsBySlug["steam"]
	if !ok {
		t.Fatalf("expected child category to be returned")
	}
	if childItem["parent_id"] != float64(parent.ID) {
		t.Fatalf("expected child parent_id=%d, got %v", parent.ID, childItem["parent_id"])
	}
	if childItem["product_count"] != float64(1) {
		t.Fatalf("expected child product_count=1, got %v", childItem["product_count"])
	}

	if _, ok := itemsBySlug["hidden"]; ok {
		t.Fatalf("expected hidden category without products or visible children to be filtered out")
	}
}

func TestGetProductDetailIncludesStockDisplayMetadataAndKeepsRealStockCount(t *testing.T) {
	dsn := fmt.Sprintf("file:channel_catalog_stock_display_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	category := categorydomain.Category{
		Slug:     "games",
		NameJSON: jsonmap.JSON{"zh-CN": "游戏"},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := productdomain.Product{
		CategoryID:          category.ID,
		Slug:                "stock-display-product",
		TitleJSON:           jsonmap.JSON{"zh-CN": "库存展示商品"},
		ContentJSON:         jsonmap.JSON{"zh-CN": "商品详情"},
		PriceAmount:         money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType:     constants.FulfillmentTypeManual,
		StockDisplayMode:    constants.ProductStockDisplayRange,
		ManualStockTotal:    42,
		ManualStockSold:     0,
		IsActive:            true,
		MinPurchaseQuantity: 1,
		MaxPurchaseQuantity: 9,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := productdomain.ProductSKU{
		ProductID:        product.ID,
		SKUCode:          "vip-year",
		SpecValuesJSON:   jsonmap.JSON{"zh-CN": "年卡"},
		PriceAmount:      money.FromDecimal(decimal.NewFromInt(10)),
		ManualStockTotal: 42,
		ManualStockSold:  0,
		IsActive:         true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	productRepo := productgormstore.NewProductStore(db)
	handler := &channeltransport.Handler{Dependencies: channeltransport.Dependencies{
		ProductRepo:    productRepo,
		ProductService: productapplication.NewService(productapplication.Options{Products: productRepo}),
		SettingService: settingsapp.NewService(settingsstore.New(db)),
	}}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/channel/catalog/products/:id", handler.GetProductDetail)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/channel/catalog/products/%d?locale=zh-CN", product.ID), nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected http 200, got %d body=%s", recorder.Code, recorder.Body.String())
	}

	var payload channelCatalogTestResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	data := payload.Data
	if data["stock_count"] != float64(42) {
		t.Fatalf("expected channel stock_count to remain real stock 42, got %v", data["stock_count"])
	}
	if data["stock_display_mode"] != constants.ProductStockDisplayRange {
		t.Fatalf("expected stock_display_mode=range, got %v", data["stock_display_mode"])
	}
	if data["stock_display"] != constants.ProductStockDisplayRange21To50 {
		t.Fatalf("expected stock_display range_21_50, got %v", data["stock_display"])
	}
	if data["stock_range_min"] != float64(21) {
		t.Fatalf("expected stock_range_min=21, got %v", data["stock_range_min"])
	}
	if data["stock_range_max"] != float64(50) {
		t.Fatalf("expected stock_range_max=50, got %v", data["stock_range_max"])
	}
	if data["stock_quantity_hidden"] != true {
		t.Fatalf("expected stock_quantity_hidden=true, got %v", data["stock_quantity_hidden"])
	}

	skus, ok := data["skus"].([]any)
	if !ok || len(skus) != 1 {
		t.Fatalf("expected one sku, got %T len=%d", data["skus"], len(skus))
	}
	skuData, ok := skus[0].(map[string]any)
	if !ok {
		t.Fatalf("expected sku object, got %T", skus[0])
	}
	if skuData["stock_count"] != float64(42) {
		t.Fatalf("expected sku stock_count to remain real stock 42, got %v", skuData["stock_count"])
	}
	if skuData["stock_display"] != constants.ProductStockDisplayRange21To50 {
		t.Fatalf("expected sku stock_display range_21_50, got %v", skuData["stock_display"])
	}
	if skuData["stock_quantity_hidden"] != true {
		t.Fatalf("expected sku stock_quantity_hidden=true, got %v", skuData["stock_quantity_hidden"])
	}

	if err := db.Model(&productdomain.Product{}).Where("id = ?", product.ID).Updates(map[string]interface{}{
		"manual_stock_total": 5,
		"stock_display_mode": constants.ProductStockDisplayStatus,
	}).Error; err != nil {
		t.Fatalf("update product to low stock: %v", err)
	}
	if err := db.Model(&productdomain.ProductSKU{}).Where("id = ?", sku.ID).Update("manual_stock_total", 5).Error; err != nil {
		t.Fatalf("update sku to low stock: %v", err)
	}

	lowStockReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/channel/catalog/products/%d?locale=zh-CN", product.ID), nil)
	lowStockRecorder := httptest.NewRecorder()
	router.ServeHTTP(lowStockRecorder, lowStockReq)
	if lowStockRecorder.Code != http.StatusOK {
		t.Fatalf("expected low stock request HTTP 200, got %d body=%s", lowStockRecorder.Code, lowStockRecorder.Body.String())
	}

	var lowStockPayload channelCatalogTestResponse
	if err := json.Unmarshal(lowStockRecorder.Body.Bytes(), &lowStockPayload); err != nil {
		t.Fatalf("decode low stock response: %v", err)
	}
	if lowStockPayload.Data["stock_status"] != constants.ProductStockStatusLowStock {
		t.Fatalf("expected product stock_status=low_stock at storefront threshold, got %v", lowStockPayload.Data["stock_status"])
	}
	if lowStockPayload.Data["stock_display"] != constants.ProductStockStatusLowStock {
		t.Fatalf("expected product stock_display=low_stock in status mode, got %v", lowStockPayload.Data["stock_display"])
	}
	lowStockSKUs, ok := lowStockPayload.Data["skus"].([]any)
	if !ok || len(lowStockSKUs) != 1 {
		t.Fatalf("expected one low stock sku, got %T len=%d", lowStockPayload.Data["skus"], len(lowStockSKUs))
	}
	lowStockSKU, ok := lowStockSKUs[0].(map[string]any)
	if !ok {
		t.Fatalf("expected low stock sku object, got %T", lowStockSKUs[0])
	}
	if lowStockSKU["stock_status"] != constants.ProductStockStatusLowStock {
		t.Fatalf("expected sku stock_status=low_stock at storefront threshold, got %v", lowStockSKU["stock_status"])
	}
}

type priceTestIdentity struct {
	channeltransport.IdentityService
	users map[string]userdomain.User
}

func (s priceTestIdentity) ResolveTelegramChannelIdentity(in channeltransport.TelegramIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, error) {
	user, ok := s.users[in.ChannelUserID]
	if !ok {
		return nil, nil, nil
	}
	return &user, nil, nil
}

type priceTestLevels struct{}

func (priceTestLevels) ResolveUserPrice(userID, _, _ uint, base decimal.Decimal) (decimal.Decimal, bool) {
	if userID == 7 {
		return decimal.NewFromInt(95), true
	}
	return base, false
}

func (priceTestLevels) ResolveMemberPrice(levelID, _, _ uint, base decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	if levelID == 3 {
		return decimal.NewFromInt(90), base.Sub(decimal.NewFromInt(90))
	}
	return base, decimal.Zero
}

// 渠道目录价口径必须与下单一致：用户专属价优先于会员等级价，未命中时回退到等级价。
func TestChannelCatalogMemberPricePrefersUserPrice(t *testing.T) {
	dsn := fmt.Sprintf("file:channel_catalog_user_price_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	category := categorydomain.Category{Slug: "user-price", NameJSON: jsonmap.JSON{"zh-CN": "分类"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "user-price-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "专属价商品"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(100)),
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID: product.ID, SKUCode: "default", PriceAmount: money.FromDecimal(decimal.NewFromInt(100)), IsActive: true,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	productRepo := productgormstore.NewProductStore(db)
	handler := &channeltransport.Handler{Dependencies: channeltransport.Dependencies{
		ProductRepo:    productRepo,
		ProductService: productapplication.NewService(productapplication.Options{Products: productRepo}),
		SettingService: settingsapp.NewService(settingsstore.New(db)),
		UserAuthService: priceTestIdentity{users: map[string]userdomain.User{
			"with-user-price": {ID: 7, MemberLevelID: 3},
			"level-only":      {ID: 8, MemberLevelID: 3},
			"no-discount":     {ID: 9},
		}},
		MemberLevelService: priceTestLevels{},
	}}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/v1/channel/catalog/products/:id", handler.GetProductDetail)

	fetch := func(channelUserID string) map[string]any {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/channel/catalog/products/%d?locale=zh-CN&channel_user_id=%s", product.ID, channelUserID), nil)
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Fatalf("%s: http %d body=%s", channelUserID, recorder.Code, recorder.Body.String())
		}
		var payload channelCatalogTestResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode response failed: %v", err)
		}
		return payload.Data
	}
	skuPrice := func(data map[string]any) any {
		skus, _ := data["skus"].([]any)
		if len(skus) != 1 {
			t.Fatalf("expected one sku, got %v", data["skus"])
		}
		return skus[0].(map[string]any)["member_price"]
	}
	assertPrice := func(label string, got any, want string) {
		t.Helper()
		if want == "" {
			if got != nil && got != "" {
				t.Fatalf("%s: expected no member price, got %v", label, got)
			}
			return
		}
		text, _ := got.(string)
		parsed, err := decimal.NewFromString(text)
		if err != nil || !parsed.Equal(decimal.RequireFromString(want)) {
			t.Fatalf("%s: member price = %v, want %s", label, got, want)
		}
	}

	exclusive := fetch("with-user-price")
	assertPrice("exclusive product", exclusive["member_price_from"], "95")
	assertPrice("exclusive sku", skuPrice(exclusive), "95")

	levelOnly := fetch("level-only")
	assertPrice("level product", levelOnly["member_price_from"], "90")
	assertPrice("level sku", skuPrice(levelOnly), "90")

	none := fetch("no-discount")
	assertPrice("none product", none["member_price_from"], "")
	assertPrice("none sku", skuPrice(none), "")
}
