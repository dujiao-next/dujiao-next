package application_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	. "github.com/dujiao-next/internal/modules/order/application"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestPreviewOrderUsesAgencyPriceForApiOrder(t *testing.T) {
	dsn := fmt.Sprintf("file:api_order_pricing_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&categorydomain.Category{}, &productdomain.Product{}, &productdomain.ProductSKU{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := categorydomain.Category{Slug: "cat-api", NameJSON: jsonmap.JSON{"zh-CN": "测试"}, SortOrder: 0, CreatedAt: now}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:        category.ID,
		Slug:              "prod-api",
		TitleJSON:         jsonmap.JSON{"zh-CN": "API测试商品"},
		PriceAmount:       money.FromDecimal(decimal.RequireFromString("100.00")),
		AgencyPriceAmount: money.FromDecimal(decimal.RequireFromString("65.00")),
		PurchaseType:      constants.ProductPurchaseMember,
		StockDisplayMode:  constants.ProductStockDisplayStatus,
		FulfillmentType:   constants.FulfillmentTypeManual,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           "DEFAULT",
		PriceAmount:       money.FromDecimal(decimal.RequireFromString("100.00")),
		AgencyPriceAmount: money.FromDecimal(decimal.RequireFromString("65.00")),
		ManualStockTotal:  10,
		IsActive:          true,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductStore:    productgormstore.NewProductStore(db),
		ProductSKUStore: productgormstore.NewSKUStore(db),
		ExpireMinutes:   15,
	})

	// 1. 普通订单结算价为 100
	previewNormal, err := svc.PreviewOrder(CreateOrderInput{
		UserID: 1,
		Items:  []CreateOrderItem{{ProductID: product.ID, SKUID: sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("preview normal order failed: %v", err)
	}
	if !previewNormal.TotalAmount.Equal(decimal.RequireFromString("200.00")) {
		t.Fatalf("expected normal total 200.00, got %s", previewNormal.TotalAmount)
	}

	// 2. API 订单结算价为 65 (代理价)
	previewAPI, err := svc.PreviewOrder(CreateOrderInput{
		UserID:     1,
		IsApiOrder: true,
		Items:      []CreateOrderItem{{ProductID: product.ID, SKUID: sku.ID, Quantity: 2}},
	})
	if err != nil {
		t.Fatalf("preview api order failed: %v", err)
	}
	if !previewAPI.TotalAmount.Equal(decimal.RequireFromString("130.00")) {
		t.Fatalf("expected api order total 130.00, got %s", previewAPI.TotalAmount)
	}
}
