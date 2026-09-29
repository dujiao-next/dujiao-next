package application_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"
	coupondomain "github.com/dujiao-next/internal/modules/coupon/domain"
	coupongormstore "github.com/dujiao-next/internal/modules/coupon/infrastructure/gormstore"
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	. "github.com/dujiao-next/internal/modules/order/application"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	promotiongormstore "github.com/dujiao-next/internal/modules/promotion/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type loopSecretOrderFixture struct {
	db      *gorm.DB
	svc     *OrderService
	buyer   userdomain.User
	product productdomain.Product
	sku     productdomain.ProductSKU
}

func newLoopSecretOrderFixture(t *testing.T) loopSecretOrderFixture {
	t.Helper()

	dsn := fmt.Sprintf("file:order_loop_secret_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&fulfillmentdomain.Fulfillment{},
		&coupondomain.Coupon{},
		&coupondomain.CouponUsage{},
		&promotiondomain.Promotion{},
		&paymentdomain.Payment{},
		&cardsecretdomain.Secret{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	category := categorydomain.Category{Slug: "loop-secret", NameJSON: jsonmap.JSON{"zh-CN": "loop-secret"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	buyer := userdomain.User{Email: "loop-secret-buyer@example.com", PasswordHash: "hash"}
	if err := db.Create(&buyer).Error; err != nil {
		t.Fatalf("create buyer failed: %v", err)
	}
	product := productdomain.Product{
		CategoryID:      category.ID,
		Slug:            "loop-secret-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "loop-secret-product"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseGuest,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := productdomain.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     productdomain.DefaultSKUCode,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(10)),
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		UserStore:        userstore.New(db),
		ProductStore:     productgormstore.NewProductStore(db),
		ProductSKUStore:  productgormstore.NewSKUStore(db),
		CouponStore:      coupongormstore.New(db),
		CouponUsageStore: coupongormstore.NewUsageStore(db),
		PromotionRepo:    promotiongormstore.New(db),
		Queue:            &fakeOrderTimeoutQueue{},
		ExpireMinutes:    15,
	})
	return loopSecretOrderFixture{db: db, svc: svc, buyer: buyer, product: product, sku: sku}
}

func (f loopSecretOrderFixture) createOrder(quantity int) (*orderdomain.Order, error) {
	return f.svc.CreateOrder(CreateOrderInput{
		UserID: f.buyer.ID,
		Items:  []CreateOrderItem{{ProductID: f.product.ID, SKUID: f.sku.ID, Quantity: quantity}},
	})
}

func (f loopSecretOrderFixture) addSecret(t *testing.T, content string, isLoop bool) cardsecretdomain.Secret {
	t.Helper()
	secret := cardsecretdomain.Secret{
		ProductID: f.product.ID,
		SKUID:     f.sku.ID,
		Secret:    content,
		Status:    cardsecretdomain.StatusAvailable,
		IsLoop:    isLoop,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := f.db.Create(&secret).Error; err != nil {
		t.Fatalf("create secret failed: %v", err)
	}
	return secret
}

// 循环卡密下单不占用库存：连续下单、一次买多件都应成功，卡密保持可用。
func TestCreateOrderWithLoopSecretSkipsReservation(t *testing.T) {
	f := newLoopSecretOrderFixture(t)
	loop := f.addSecret(t, "LOOP-SECRET", true)

	for _, quantity := range []int{3, 2} {
		if _, err := f.createOrder(quantity); err != nil {
			t.Fatalf("create order with quantity %d failed: %v", quantity, err)
		}
	}

	var after cardsecretdomain.Secret
	if err := f.db.First(&after, loop.ID).Error; err != nil {
		t.Fatalf("query loop secret failed: %v", err)
	}
	if after.Status != cardsecretdomain.StatusAvailable || after.OrderID != nil || after.ReservedAt != nil {
		t.Fatalf("loop secret should stay available and unbound, got status=%s order_id=%v reserved_at=%v",
			after.Status, after.OrderID, after.ReservedAt)
	}
}

// 没有循环卡密时保持原有行为：按数量占用普通卡密，不足则报错。
func TestCreateOrderWithoutLoopSecretKeepsReservation(t *testing.T) {
	f := newLoopSecretOrderFixture(t)
	normal := f.addSecret(t, "NORMAL-SECRET", false)

	if _, err := f.createOrder(2); !errors.Is(err, ErrCardSecretInsufficient) {
		t.Fatalf("want ErrCardSecretInsufficient for quantity 2, got %v", err)
	}
	if _, err := f.createOrder(1); err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	var after cardsecretdomain.Secret
	if err := f.db.First(&after, normal.ID).Error; err != nil {
		t.Fatalf("query normal secret failed: %v", err)
	}
	if after.Status != cardsecretdomain.StatusReserved {
		t.Fatalf("normal secret should be reserved, got %s", after.Status)
	}
}

// 停用（非 available）的循环卡密不再生效，回到普通库存逻辑。
func TestCreateOrderIgnoresDisabledLoopSecret(t *testing.T) {
	f := newLoopSecretOrderFixture(t)
	loop := f.addSecret(t, "LOOP-SECRET", true)
	if err := f.db.Model(&cardsecretdomain.Secret{}).Where("id = ?", loop.ID).
		Update("status", cardsecretdomain.StatusUsed).Error; err != nil {
		t.Fatalf("disable loop secret failed: %v", err)
	}

	if _, err := f.createOrder(1); !errors.Is(err, ErrCardSecretInsufficient) {
		t.Fatalf("want ErrCardSecretInsufficient when loop secret disabled, got %v", err)
	}
}
