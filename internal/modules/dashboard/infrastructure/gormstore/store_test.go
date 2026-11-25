package gormstore

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

func setupDashboardRepositoryTest(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &categorydomain.Category{}, &productdomain.Product{}, &orderdomain.Order{}, &orderdomain.OrderItem{}); err != nil {
		t.Fatalf("migrate dashboard models failed: %v", err)
	}
	if err := db.AutoMigrate(&productdomain.ProductSKU{}); err != nil {
		t.Fatalf("migrate dashboard sku models failed: %v", err)
	}
	if err := db.AutoMigrate(&paymentdomain.PaymentChannel{}, &paymentdomain.Payment{}, &orderdomain.OrderRefundRecord{}, &walletdomain.Transaction{}); err != nil {
		t.Fatalf("migrate dashboard models failed: %v", err)
	}
	return New(db), db
}

func createDashboardCategory(t *testing.T, db *gorm.DB, slug string) *categorydomain.Category {
	t.Helper()
	category := &categorydomain.Category{
		Slug:     slug,
		NameJSON: jsonmap.JSON{"zh-CN": "测试分类"},
	}
	if err := db.Create(category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	return category
}

func createDashboardProfitOrderWithItem(
	t *testing.T,
	db *gorm.DB,
	product *productdomain.Product,
	orderNo string,
	status string,
	amount int64,
	cost int64,
	title string,
	createdAt time.Time,
) *orderdomain.Order {
	t.Helper()
	order := &orderdomain.Order{
		OrderNo:        orderNo,
		UserID:         1,
		Status:         status,
		Currency:       "CNY",
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(amount)),
		DiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(amount)),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       product.ID,
		TitleJSON:       jsonmap.JSON{"zh-CN": title},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(amount)),
		CostPrice:       money.FromDecimal(decimal.NewFromInt(cost)),
		Quantity:        1,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(amount)),
		CouponDiscount:  money.FromDecimal(decimal.Zero),
		FulfillmentType: constants.FulfillmentTypeManual,
		CreatedAt:       createdAt,
		UpdatedAt:       createdAt,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	return order
}
