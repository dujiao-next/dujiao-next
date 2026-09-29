package application_test

import (
	"strings"
	"testing"
	"time"

	. "github.com/dujiao-next/internal/modules/fulfillment/application"
	fulfillmentgormstore "github.com/dujiao-next/internal/modules/fulfillment/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	cardsecretdomain "github.com/dujiao-next/internal/modules/cardsecret/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func createPaidAutoOrder(t *testing.T, db *gorm.DB, orderNo string, productID, skuID uint, quantity int) *orderdomain.Order {
	t.Helper()
	now := time.Now()
	order := &orderdomain.Order{
		OrderNo:                 orderNo,
		UserID:                  1,
		Status:                  constants.OrderStatusPaid,
		Currency:                "CNY",
		OriginalAmount:          money.FromDecimal(decimal.NewFromInt(10)),
		DiscountAmount:          money.FromDecimal(decimal.Zero),
		PromotionDiscountAmount: money.FromDecimal(decimal.Zero),
		TotalAmount:             money.FromDecimal(decimal.NewFromInt(10)),
		WalletPaidAmount:        money.FromDecimal(decimal.Zero),
		OnlinePaidAmount:        money.FromDecimal(decimal.NewFromInt(10)),
		RefundedAmount:          money.FromDecimal(decimal.Zero),
		CreatedAt:               now,
		UpdatedAt:               now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       productID,
		SKUID:           skuID,
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		UnitPrice:       money.FromDecimal(decimal.NewFromInt(10)),
		Quantity:        quantity,
		TotalPrice:      money.FromDecimal(decimal.NewFromInt(10)),
		FulfillmentType: constants.FulfillmentTypeAuto,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	return order
}

func createSecret(t *testing.T, db *gorm.DB, secret cardsecretdomain.Secret) cardsecretdomain.Secret {
	t.Helper()
	secret.CreatedAt = time.Now()
	secret.UpdatedAt = secret.CreatedAt
	if err := db.Create(&secret).Error; err != nil {
		t.Fatalf("create secret failed: %v", err)
	}
	return secret
}

func newFulfillmentService(db *gorm.DB) *Service {
	return New(Options{
		OrderStore:       ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
		FulfillmentStore: fulfillmentgormstore.New(db),
	})
}

// 循环卡密按数量重复发货，不消耗状态；同商品的普通卡密不受影响。
func TestCreateAutoFulfillmentLoopSecretRepeatsWithoutConsuming(t *testing.T) {
	db := setupFulfillmentServiceTestDB(t)
	loop := createSecret(t, db, cardsecretdomain.Secret{ProductID: 200, SKUID: 2001, Secret: "LOOP-SECRET", Status: cardsecretdomain.StatusAvailable, IsLoop: true})
	normal := createSecret(t, db, cardsecretdomain.Secret{ProductID: 200, SKUID: 2001, Secret: "NORMAL-SECRET", Status: cardsecretdomain.StatusAvailable})
	svc := newFulfillmentService(db)

	for i, quantity := range []int{3, 1} {
		order := createPaidAutoOrder(t, db, "FULFILL-LOOP-"+string(rune('A'+i)), 200, 2001, quantity)
		result, err := svc.CreateAuto(order.ID)
		if err != nil {
			t.Fatalf("create auto fulfillment failed: %v", err)
		}
		if got := strings.Split(result.Payload, "\n"); len(got) != quantity || strings.Count(result.Payload, "LOOP-SECRET") != quantity {
			t.Fatalf("payload should repeat loop secret %d times, got %q", quantity, result.Payload)
		}
		if strings.Contains(result.Payload, "NORMAL-SECRET") {
			t.Fatalf("payload should not contain normal secret, got %q", result.Payload)
		}
	}

	for _, id := range []uint{loop.ID, normal.ID} {
		var after cardsecretdomain.Secret
		if err := db.First(&after, id).Error; err != nil {
			t.Fatalf("query secret failed: %v", err)
		}
		if after.Status != cardsecretdomain.StatusAvailable || after.OrderID != nil {
			t.Fatalf("secret %d should stay available and unbound, got status=%s order_id=%v", id, after.Status, after.OrderID)
		}
	}
}

// 录入循环卡密之前已占用普通卡密的订单，仍发放并消耗已占用的卡密。
func TestCreateAutoFulfillmentPrefersReservedSecretsOverLoop(t *testing.T) {
	db := setupFulfillmentServiceTestDB(t)
	order := createPaidAutoOrder(t, db, "FULFILL-LOOP-RESERVED", 300, 3001, 1)
	reservedAt := time.Now()
	reserved := createSecret(t, db, cardsecretdomain.Secret{
		ProductID: 300, SKUID: 3001, Secret: "RESERVED-SECRET",
		Status: cardsecretdomain.StatusReserved, OrderID: &order.ID, ReservedAt: &reservedAt,
	})
	createSecret(t, db, cardsecretdomain.Secret{ProductID: 300, SKUID: 3001, Secret: "LOOP-SECRET", Status: cardsecretdomain.StatusAvailable, IsLoop: true})

	result, err := newFulfillmentService(db).CreateAuto(order.ID)
	if err != nil {
		t.Fatalf("create auto fulfillment failed: %v", err)
	}
	if result.Payload != "RESERVED-SECRET" {
		t.Fatalf("payload should be the reserved secret, got %q", result.Payload)
	}

	var after cardsecretdomain.Secret
	if err := db.First(&after, reserved.ID).Error; err != nil {
		t.Fatalf("query reserved secret failed: %v", err)
	}
	if after.Status != cardsecretdomain.StatusUsed {
		t.Fatalf("reserved secret should be used, got %s", after.Status)
	}
}
