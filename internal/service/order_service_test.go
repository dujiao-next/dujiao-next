package service

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestMergeCreateOrderItems(t *testing.T) {
	items := []CreateOrderItem{
		{ProductID: 1, SKUID: 10, Quantity: 1, FulfillmentType: "auto"},
		{ProductID: 1, SKUID: 10, Quantity: 2, FulfillmentType: "auto"},
		{ProductID: 1, SKUID: 11, Quantity: 1, FulfillmentType: "auto"},
		{ProductID: 2, SKUID: 20, Quantity: 1, FulfillmentType: ""},
	}
	merged, err := mergeCreateOrderItems(items)
	if err != nil {
		t.Fatalf("mergeCreateOrderItems error: %v", err)
	}
	if len(merged) != 3 {
		t.Fatalf("expected 3 items, got %d", len(merged))
	}
	if merged[0].ProductID != 1 || merged[0].SKUID != 10 || merged[0].Quantity != 3 {
		t.Fatalf("unexpected merged item: %+v", merged[0])
	}
	if merged[0].FulfillmentType != "" {
		t.Fatalf("expected empty fulfillment type, got: %s", merged[0].FulfillmentType)
	}
}

func TestMergeCreateOrderItemsConflict(t *testing.T) {
	items := []CreateOrderItem{
		{ProductID: 1, SKUID: 10, Quantity: 1, FulfillmentType: "auto"},
		{ProductID: 1, SKUID: 11, Quantity: 1, FulfillmentType: "manual"},
	}
	merged, err := mergeCreateOrderItems(items)
	if err != nil {
		t.Fatalf("expected no error for conflicting fulfillment type input, got: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("unexpected merged result: %+v", merged)
	}
}

func TestApplyCouponDiscountToItems(t *testing.T) {
	plans := []childOrderPlan{
		{Item: models.OrderItem{ProductID: 1}, TotalAmount: decimal.NewFromInt(100)},
		{Item: models.OrderItem{ProductID: 2}, TotalAmount: decimal.NewFromInt(50)},
		{Item: models.OrderItem{ProductID: 3}, TotalAmount: decimal.NewFromInt(50)},
	}
	coupon := &models.Coupon{
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: "[1,2]",
	}
	if err := applyCouponDiscountToItems(plans, coupon, decimal.NewFromInt(30)); err != nil {
		t.Fatalf("applyCouponDiscountToItems error: %v", err)
	}
	if !plans[0].CouponDiscount.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("expected 20, got %s", plans[0].CouponDiscount.String())
	}
	if !plans[1].CouponDiscount.Equal(decimal.NewFromInt(10)) {
		t.Fatalf("expected 10, got %s", plans[1].CouponDiscount.String())
	}
	if !plans[2].CouponDiscount.Equal(decimal.Zero) {
		t.Fatalf("expected 0, got %s", plans[2].CouponDiscount.String())
	}
}

func TestResolveManualFormSubmissionPreferOrderItemKey(t *testing.T) {
	data := map[string]models.JSON{
		"1":    {"legacy": "legacy"},
		"1:10": {"current": "current"},
	}
	got := resolveManualFormSubmission(data, 1, 10)
	if got["current"] != "current" {
		t.Fatalf("expected order item key value, got: %+v", got)
	}
}

func TestResolveManualFormSubmissionFallbackLegacyProductKey(t *testing.T) {
	data := map[string]models.JSON{
		"1": {"legacy": "legacy"},
	}
	got := resolveManualFormSubmission(data, 1, 99)
	if got["legacy"] != "legacy" {
		t.Fatalf("expected legacy product key value, got: %+v", got)
	}
}

type orderPurchaseQuantityLimitFixture struct {
	dsnPrefix       string
	categorySlug    string
	productSlug     string
	minQuantity     int
	maxQuantity     int
	requestQuantity int
	expectedErr     error
}

func assertBuildOrderResultRejectsPurchaseQuantity(t *testing.T, fixture orderPurchaseQuantityLimitFixture) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s_%d?mode=memory&cache=shared", fixture.dsnPrefix, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductSKU{}, &models.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      fixture.categorySlug,
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:          category.ID,
		Slug:                fixture.productSlug,
		TitleJSON:           models.JSON{"zh-CN": "测试商品"},
		PriceAmount:         models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:        constants.ProductPurchaseMember,
		FulfillmentType:     constants.FulfillmentTypeManual,
		MinPurchaseQuantity: fixture.minQuantity,
		MaxPurchaseQuantity: fixture.maxQuantity,
		IsActive:            true,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           models.DefaultSKUCode,
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductRepo:    repository.NewProductRepository(db),
		ProductSKURepo: repository.NewProductSKURepository(db),
		PromotionRepo:  repository.NewPromotionRepository(db),
		ExpireMinutes:  15,
	})

	_, err = svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  fixture.requestQuantity,
			},
		},
	})
	if !errors.Is(err, fixture.expectedErr) {
		t.Fatalf("expected %v, got: %v", fixture.expectedErr, err)
	}
}

func TestCalcParentStatus(t *testing.T) {
	children := []models.Order{
		{Status: constants.OrderStatusDelivered},
		{Status: constants.OrderStatusPaid},
	}
	status := calcParentStatus(children, constants.OrderStatusPaid)
	if status != constants.OrderStatusPartiallyDelivered {
		t.Fatalf("expected partially_delivered, got %s", status)
	}

	children = []models.Order{
		{Status: constants.OrderStatusDelivered},
		{Status: constants.OrderStatusDelivered},
	}
	status = calcParentStatus(children, constants.OrderStatusPaid)
	if status != constants.OrderStatusDelivered {
		t.Fatalf("expected delivered, got %s", status)
	}

	children = []models.Order{
		{Status: constants.OrderStatusFulfilled},
		{Status: constants.OrderStatusFulfilled},
	}
	status = calcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusFulfilled {
		t.Fatalf("expected fulfilled, got %s", status)
	}

	children = []models.Order{
		{Status: constants.OrderStatusCompleted},
		{Status: constants.OrderStatusCompleted},
	}
	status = calcParentStatus(children, constants.OrderStatusFulfilled)
	if status != constants.OrderStatusCompleted {
		t.Fatalf("expected completed, got %s", status)
	}
}

func TestCalcParentStatusAllRefunded(t *testing.T) {
	children := []models.Order{
		{Status: constants.OrderStatusRefunded},
		{Status: constants.OrderStatusRefunded},
	}
	status := calcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusRefunded {
		t.Fatalf("expected refunded, got %s", status)
	}
}

func TestCalcParentStatusPartiallyRefunded(t *testing.T) {
	children := []models.Order{
		{Status: constants.OrderStatusRefunded},
		{Status: constants.OrderStatusDelivered},
	}
	status := calcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected partially_refunded, got %s", status)
	}
}

func TestExpectedRefundStatus(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		order  models.Order
		expect string
	}{
		{
			name: "partial refund",
			order: models.Order{
				Status:         constants.OrderStatusCompleted,
				PaidAt:         &now,
				TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
			},
			expect: constants.OrderStatusPartiallyRefunded,
		},
		{
			name: "full refund",
			order: models.Order{
				Status:         constants.OrderStatusCompleted,
				PaidAt:         &now,
				TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
			},
			expect: constants.OrderStatusRefunded,
		},
		{
			name: "canceled should keep",
			order: models.Order{
				Status:         constants.OrderStatusCanceled,
				PaidAt:         &now,
				TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
			},
			expect: "",
		},
	}

	for _, tc := range tests {
		got := expectedRefundStatus(&tc.order)
		if got != tc.expect {
			t.Fatalf("%s: expected %q, got %q", tc.name, tc.expect, got)
		}
	}
}

func TestResolvedParentStatusPrefersOwnRefund(t *testing.T) {
	now := time.Now()
	order := &models.Order{
		Status:         constants.OrderStatusCompleted,
		PaidAt:         &now,
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(40)),
		RefundedAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		Children: []models.Order{
			{Status: constants.OrderStatusCompleted},
			{Status: constants.OrderStatusCompleted},
		},
	}
	if got := resolvedParentStatus(order); got != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected partially_refunded, got %s", got)
	}
}

func TestIsTransitionAllowedRefunded(t *testing.T) {
	if !isTransitionAllowed(constants.OrderStatusDelivered, constants.OrderStatusPartiallyRefunded) {
		t.Fatalf("expected delivered to partially_refunded transition to be allowed")
	}
	if !isTransitionAllowed(constants.OrderStatusPartiallyRefunded, constants.OrderStatusRefunded) {
		t.Fatalf("expected partially_refunded to refunded transition to be allowed")
	}
	if !isTransitionAllowed(constants.OrderStatusDelivered, constants.OrderStatusRefunded) {
		t.Fatalf("expected delivered to refunded transition to be allowed")
	}
	if !isTransitionAllowed(constants.OrderStatusCompleted, constants.OrderStatusRefunded) {
		t.Fatalf("expected completed to refunded transition to be allowed")
	}
	if !isTransitionAllowed(constants.OrderStatusFulfilled, constants.OrderStatusCompleted) {
		t.Fatalf("expected fulfilled to completed transition to be allowed")
	}
	if isTransitionAllowed(constants.OrderStatusCanceled, constants.OrderStatusRefunded) {
		t.Fatalf("expected canceled to refunded transition to be rejected")
	}
}

func TestCancelExpiredOrderExpiresPendingPayments(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_expire_payments_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Order{},
		&models.OrderItem{},
		&models.Fulfillment{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Coupon{},
		&models.CouponUsage{},
		&models.Payment{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	expiresAt := now.Add(-time.Hour)
	order := &models.Order{
		OrderNo:          "EXPIRE-PAYMENT-001",
		Status:           constants.OrderStatusPendingPayment,
		Currency:         "CNY",
		OriginalAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		TotalAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		ExpiresAt:        &expiresAt,
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pendingPayment := &models.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusPending,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
	}
	initiatedPayment := &models.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusInitiated,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
	}
	successPayment := &models.Payment{
		OrderID:         order.ID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          constants.PaymentStatusSuccess,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
		PaidAt:          &now,
	}
	if err := db.Create([]*models.Payment{pendingPayment, initiatedPayment, successPayment}).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderRepo:       repository.NewOrderRepository(db),
		PaymentRepo:     repository.NewPaymentRepository(db),
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
	})
	updated, err := svc.CancelExpiredOrder(order.ID)
	if err != nil {
		t.Fatalf("cancel expired order failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled order, got: %+v", updated)
	}

	var reloaded []models.Payment
	if err := db.Order("id asc").Find(&reloaded, "order_id = ?", order.ID).Error; err != nil {
		t.Fatalf("reload payments failed: %v", err)
	}
	if len(reloaded) != 3 {
		t.Fatalf("expected 3 payments, got %d", len(reloaded))
	}
	if reloaded[0].Status != constants.PaymentStatusExpired || reloaded[0].ExpiredAt == nil {
		t.Fatalf("pending payment should expire, got status=%s expired_at=%v", reloaded[0].Status, reloaded[0].ExpiredAt)
	}
	if reloaded[1].Status != constants.PaymentStatusExpired || reloaded[1].ExpiredAt == nil {
		t.Fatalf("initiated payment should expire, got status=%s expired_at=%v", reloaded[1].Status, reloaded[1].ExpiredAt)
	}
	if reloaded[2].Status != constants.PaymentStatusSuccess || reloaded[2].ExpiredAt != nil {
		t.Fatalf("success payment should stay success without expired_at, got status=%s expired_at=%v", reloaded[2].Status, reloaded[2].ExpiredAt)
	}
}

func setupCancelPaymentTestDB(t *testing.T, namespace string) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:order_service_%s_%d?mode=memory&cache=shared", namespace, time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Order{},
		&models.OrderItem{},
		&models.Fulfillment{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Coupon{},
		&models.CouponUsage{},
		&models.Payment{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	return db
}

func newPendingOrderForCancel(orderNo string, userID uint, parentID *uint, now time.Time) *models.Order {
	return &models.Order{
		OrderNo:          orderNo,
		UserID:           userID,
		ParentID:         parentID,
		Status:           constants.OrderStatusPendingPayment,
		Currency:         "CNY",
		OriginalAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		TotalAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		CreatedAt:        now.Add(-2 * time.Hour),
		UpdatedAt:        now.Add(-2 * time.Hour),
	}
}

func newPaymentForOrder(orderID uint, status string, now time.Time) *models.Payment {
	return &models.Payment{
		OrderID:         orderID,
		ChannelID:       1,
		ProviderType:    constants.PaymentProviderEpay,
		ChannelType:     "alipay",
		InteractionMode: constants.PaymentInteractionQR,
		Amount:          models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		Status:          status,
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now.Add(-2 * time.Hour),
	}
}

func TestCancelOrderExpiresPendingPayments(t *testing.T) {
	db := setupCancelPaymentTestDB(t, "user_cancel_expire_payments")

	now := time.Now()
	const userID uint = 42
	order := newPendingOrderForCancel("USER-CANCEL-001", userID, nil, now)
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pending := newPaymentForOrder(order.ID, constants.PaymentStatusPending, now)
	initiated := newPaymentForOrder(order.ID, constants.PaymentStatusInitiated, now)
	if err := db.Create([]*models.Payment{pending, initiated}).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderRepo:       repository.NewOrderRepository(db),
		PaymentRepo:     repository.NewPaymentRepository(db),
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
	})
	updated, err := svc.CancelOrder(order.ID, userID)
	if err != nil {
		t.Fatalf("cancel order failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled order, got: %+v", updated)
	}

	var reloaded []models.Payment
	if err := db.Order("id asc").Find(&reloaded, "order_id = ?", order.ID).Error; err != nil {
		t.Fatalf("reload payments failed: %v", err)
	}
	if len(reloaded) != 2 {
		t.Fatalf("expected 2 payments, got %d", len(reloaded))
	}
	for _, p := range reloaded {
		if p.Status != constants.PaymentStatusExpired || p.ExpiredAt == nil {
			t.Fatalf("payment %d should expire, got status=%s expired_at=%v", p.ID, p.Status, p.ExpiredAt)
		}
	}
}

func TestUpdateOrderStatusAdminCancelExpiresPendingPaymentsSingleOrder(t *testing.T) {
	db := setupCancelPaymentTestDB(t, "admin_cancel_expire_payments_single")

	now := time.Now()
	order := newPendingOrderForCancel("ADMIN-CANCEL-SINGLE-001", 0, nil, now)
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}

	pending := newPaymentForOrder(order.ID, constants.PaymentStatusPending, now)
	if err := db.Create(pending).Error; err != nil {
		t.Fatalf("create payment failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderRepo:       repository.NewOrderRepository(db),
		PaymentRepo:     repository.NewPaymentRepository(db),
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
	})
	updated, err := svc.UpdateOrderStatus(order.ID, constants.OrderStatusCanceled)
	if err != nil {
		t.Fatalf("update order status failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled order, got: %+v", updated)
	}

	var reloaded models.Payment
	if err := db.First(&reloaded, pending.ID).Error; err != nil {
		t.Fatalf("reload payment failed: %v", err)
	}
	if reloaded.Status != constants.PaymentStatusExpired || reloaded.ExpiredAt == nil {
		t.Fatalf("payment should expire, got status=%s expired_at=%v", reloaded.Status, reloaded.ExpiredAt)
	}
}

func TestCancelExpiredOrderExpiresPaymentsForParentAndChildren(t *testing.T) {
	db := setupCancelPaymentTestDB(t, "expire_payments_parent_children")

	now := time.Now()
	expiresAt := now.Add(-time.Hour)
	parent := newPendingOrderForCancel("PARENT-EXPIRE-001", 0, nil, now)
	parent.ExpiresAt = &expiresAt
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent failed: %v", err)
	}

	childA := newPendingOrderForCancel("PARENT-EXPIRE-001-A", 0, &parent.ID, now)
	childA.ExpiresAt = &expiresAt
	if err := db.Create(childA).Error; err != nil {
		t.Fatalf("create childA failed: %v", err)
	}
	childB := newPendingOrderForCancel("PARENT-EXPIRE-001-B", 0, &parent.ID, now)
	childB.ExpiresAt = &expiresAt
	if err := db.Create(childB).Error; err != nil {
		t.Fatalf("create childB failed: %v", err)
	}

	parentPayment := newPaymentForOrder(parent.ID, constants.PaymentStatusPending, now)
	childAPayment := newPaymentForOrder(childA.ID, constants.PaymentStatusInitiated, now)
	childBPayment := newPaymentForOrder(childB.ID, constants.PaymentStatusPending, now)
	if err := db.Create([]*models.Payment{parentPayment, childAPayment, childBPayment}).Error; err != nil {
		t.Fatalf("create payments failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderRepo:       repository.NewOrderRepository(db),
		PaymentRepo:     repository.NewPaymentRepository(db),
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
	})
	updated, err := svc.CancelExpiredOrder(parent.ID)
	if err != nil {
		t.Fatalf("cancel expired parent failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusCanceled {
		t.Fatalf("expected canceled parent, got: %+v", updated)
	}

	for _, pid := range []uint{parentPayment.ID, childAPayment.ID, childBPayment.ID} {
		var p models.Payment
		if err := db.First(&p, pid).Error; err != nil {
			t.Fatalf("reload payment %d failed: %v", pid, err)
		}
		if p.Status != constants.PaymentStatusExpired || p.ExpiredAt == nil {
			t.Fatalf("payment %d should expire, got status=%s expired_at=%v", pid, p.Status, p.ExpiredAt)
		}
	}
}

func TestUpdateOrderStatusParentToPartiallyRefundedSyncsChildren(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_parent_partial_refund_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Order{}, &models.OrderItem{}, &models.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	paidAt := now
	parent := &models.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001",
		UserID:           0,
		Status:           constants.OrderStatusDelivered,
		Currency:         "CNY",
		OriginalAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		TotalAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(30)),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}

	childA := &models.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001-A",
		ParentID:         &parent.ID,
		UserID:           0,
		Status:           constants.OrderStatusDelivered,
		Currency:         "CNY",
		OriginalAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(60)),
		DiscountAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		TotalAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(60)),
		WalletPaidAmount: models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(60)),
		RefundedAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(childA).Error; err != nil {
		t.Fatalf("create childA order failed: %v", err)
	}

	childB := &models.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001-B",
		ParentID:         &parent.ID,
		UserID:           0,
		Status:           constants.OrderStatusCompleted,
		Currency:         "CNY",
		OriginalAmount:   models.NewMoneyFromDecimal(decimal.NewFromInt(40)),
		DiscountAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		TotalAmount:      models.NewMoneyFromDecimal(decimal.NewFromInt(40)),
		WalletPaidAmount: models.NewMoneyFromDecimal(decimal.Zero),
		OnlinePaidAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(40)),
		RefundedAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(childB).Error; err != nil {
		t.Fatalf("create childB order failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderRepo: repository.NewOrderRepository(db),
	})
	updated, err := svc.UpdateOrderStatus(parent.ID, constants.OrderStatusPartiallyRefunded)
	if err != nil {
		t.Fatalf("update parent status failed: %v", err)
	}
	if updated == nil || updated.Status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected parent partially_refunded, got: %+v", updated)
	}
	if len(updated.Children) != 2 {
		t.Fatalf("expected 2 children in updated order, got: %d", len(updated.Children))
	}
	for _, child := range updated.Children {
		if child.Status != constants.OrderStatusPartiallyRefunded {
			t.Fatalf("expected child partially_refunded, got: %s", child.Status)
		}
	}

	var reloadedA models.Order
	if err := db.First(&reloadedA, childA.ID).Error; err != nil {
		t.Fatalf("reload childA failed: %v", err)
	}
	if reloadedA.Status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected childA partially_refunded, got: %s", reloadedA.Status)
	}
	var reloadedB models.Order
	if err := db.First(&reloadedB, childB.ID).Error; err != nil {
		t.Fatalf("reload childB failed: %v", err)
	}
	if reloadedB.Status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected childB partially_refunded, got: %s", reloadedB.Status)
	}
}

func TestCanCompleteParentOrder(t *testing.T) {
	order := &models.Order{
		Status: constants.OrderStatusDelivered,
		Children: []models.Order{
			{Status: constants.OrderStatusDelivered},
			{Status: constants.OrderStatusCompleted},
		},
	}
	if !canCompleteParentOrder(order) {
		t.Fatalf("expected delivered parent order to be completable")
	}
}

func TestCanCompleteParentOrderRejectInvalidStatus(t *testing.T) {
	order := &models.Order{
		Status: constants.OrderStatusPartiallyDelivered,
		Children: []models.Order{
			{Status: constants.OrderStatusDelivered},
		},
	}
	if canCompleteParentOrder(order) {
		t.Fatalf("expected partially_delivered parent order to be rejected")
	}
}

func TestCanCompleteParentOrderRejectInvalidChild(t *testing.T) {
	order := &models.Order{
		Status: constants.OrderStatusDelivered,
		Children: []models.Order{
			{Status: constants.OrderStatusPaid},
		},
	}
	if canCompleteParentOrder(order) {
		t.Fatalf("expected parent order with paid child to be rejected")
	}
}

func TestBuildOrderResultRejectsZeroPromotionPrice(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_promo_zero_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductSKU{}, &models.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      "test-category",
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            "test-product",
		TitleJSON:       models.JSON{"zh-CN": "测试商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           models.DefaultSKUCode,
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	promotion := models.Promotion{
		Name:       "test-100-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		MinAmount:  models.NewMoneyFromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductRepo:    repository.NewProductRepository(db),
		ProductSKURepo: repository.NewProductSKURepository(db),
		PromotionRepo:  repository.NewPromotionRepository(db),
		ExpireMinutes:  15,
	})

	_, err = svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  1,
			},
		},
	})
	if !errors.Is(err, ErrProductPriceInvalid) {
		t.Fatalf("expected product price invalid, got: %v", err)
	}
}

func TestPreviewOrderAppliesMemberDiscountForManualProductBeforeFormCompleted(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_manual_member_preview_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Promotion{},
		&models.User{},
		&models.MemberLevel{},
		&models.MemberLevelPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      "manual-member-preview-category",
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	level := models.MemberLevel{
		NameJSON:     models.JSON{"zh-CN": "金牌会员"},
		Slug:         "gold",
		DiscountRate: models.NewMoneyFromDecimal(decimal.NewFromInt(80)),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("create member level failed: %v", err)
	}
	user := models.User{
		Email:         "manual-preview@example.com",
		PasswordHash:  "hash",
		Status:        "active",
		MemberLevelID: level.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            "manual-member-preview-product",
		TitleJSON:       models.JSON{"zh-CN": "人工发货商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		ManualFormSchemaJSON: models.JSON{
			"fields": []interface{}{
				map[string]interface{}{
					"key":      "account",
					"type":     "text",
					"required": true,
					"label":    map[string]interface{}{"zh-CN": "账号"},
				},
			},
		},
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           models.DefaultSKUCode,
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	levelRepo := repository.NewMemberLevelRepository(db)
	priceRepo := repository.NewMemberLevelPriceRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewOrderService(OrderServiceOptions{
		UserRepo:           userRepo,
		ProductRepo:        repository.NewProductRepository(db),
		ProductSKURepo:     repository.NewProductSKURepository(db),
		PromotionRepo:      repository.NewPromotionRepository(db),
		MemberLevelService: NewMemberLevelService(levelRepo, priceRepo, userRepo),
		ExpireMinutes:      15,
	})

	preview, err := svc.PreviewOrder(CreateOrderInput{
		UserID: user.ID,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("preview order failed: %v", err)
	}

	expectedOriginal := decimal.NewFromInt(200)
	expectedMemberDiscount := decimal.NewFromInt(40)
	expectedTotal := decimal.NewFromInt(160)
	if !preview.OriginalAmount.Decimal.Equal(expectedOriginal) {
		t.Fatalf("expected original amount %s, got: %s", expectedOriginal.String(), preview.OriginalAmount.String())
	}
	if !preview.MemberDiscountAmount.Decimal.Equal(expectedMemberDiscount) {
		t.Fatalf("expected member discount amount %s, got: %s", expectedMemberDiscount.String(), preview.MemberDiscountAmount.String())
	}
	if !preview.TotalAmount.Decimal.Equal(expectedTotal) {
		t.Fatalf("expected total amount %s, got: %s", expectedTotal.String(), preview.TotalAmount.String())
	}
}

func TestBuildOrderResultStacksPromotionAndMemberDiscount(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_stack_promo_member_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Promotion{},
		&models.User{},
		&models.MemberLevel{},
		&models.MemberLevelPrice{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      "stack-promo-member-category",
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}
	level := models.MemberLevel{
		NameJSON:     models.JSON{"zh-CN": "金牌会员"},
		Slug:         "stack-gold",
		DiscountRate: models.NewMoneyFromDecimal(decimal.NewFromInt(80)),
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("create member level failed: %v", err)
	}
	user := models.User{
		Email:         "stack-promo-member@example.com",
		PasswordHash:  "hash",
		Status:        "active",
		MemberLevelID: level.ID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            "stack-promo-member-product",
		TitleJSON:       models.JSON{"zh-CN": "叠加优惠商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeAuto,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}
	sku := models.ProductSKU{
		ProductID:   product.ID,
		SKUCode:     models.DefaultSKUCode,
		PriceAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}
	promotion := models.Promotion{
		Name:       "test-10-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		MinAmount:  models.NewMoneyFromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	levelRepo := repository.NewMemberLevelRepository(db)
	priceRepo := repository.NewMemberLevelPriceRepository(db)
	userRepo := repository.NewUserRepository(db)
	svc := NewOrderService(OrderServiceOptions{
		UserRepo:           userRepo,
		ProductRepo:        repository.NewProductRepository(db),
		ProductSKURepo:     repository.NewProductSKURepository(db),
		PromotionRepo:      repository.NewPromotionRepository(db),
		MemberLevelService: NewMemberLevelService(levelRepo, priceRepo, userRepo),
		ExpireMinutes:      15,
	})

	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: user.ID,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	expectedOriginal := decimal.NewFromInt(200)
	expectedPromotion := decimal.NewFromInt(20)
	expectedMemberDiscount := decimal.NewFromInt(36)
	expectedTotal := decimal.NewFromInt(144)
	if !result.OriginalAmount.Equal(expectedOriginal) {
		t.Fatalf("expected original amount %s, got: %s", expectedOriginal.String(), result.OriginalAmount.String())
	}
	if !result.PromotionDiscountAmount.Equal(expectedPromotion) {
		t.Fatalf("expected promotion discount amount %s, got: %s", expectedPromotion.String(), result.PromotionDiscountAmount.String())
	}
	if !result.MemberDiscountAmount.Equal(expectedMemberDiscount) {
		t.Fatalf("expected member discount amount %s, got: %s", expectedMemberDiscount.String(), result.MemberDiscountAmount.String())
	}
	if !result.TotalAmount.Equal(expectedTotal) {
		t.Fatalf("expected total amount %s, got: %s", expectedTotal.String(), result.TotalAmount.String())
	}
	if len(result.Plans) != 1 {
		t.Fatalf("expected one plan, got %d", len(result.Plans))
	}
	item := result.Plans[0].Item
	if item.OriginalUnitPrice.String() != "100.00" {
		t.Fatalf("expected original unit price 100.00, got %s", item.OriginalUnitPrice.String())
	}
	if item.OriginalTotalPrice.String() != "200.00" {
		t.Fatalf("expected original total price 200.00, got %s", item.OriginalTotalPrice.String())
	}
	if item.UnitPrice.String() != "72.00" {
		t.Fatalf("expected final unit price 72.00, got %s", item.UnitPrice.String())
	}
	if item.TotalPrice.String() != "144.00" {
		t.Fatalf("expected final total price 144.00, got %s", item.TotalPrice.String())
	}
}

func TestBuildOrderResultRejectsProductMaxPurchaseQuantityExceeded(t *testing.T) {
	assertBuildOrderResultRejectsPurchaseQuantity(t, orderPurchaseQuantityLimitFixture{
		dsnPrefix:       "order_service_purchase_limit",
		categorySlug:    "test-category-limit",
		productSlug:     "test-product-limit",
		maxQuantity:     2,
		requestQuantity: 3,
		expectedErr:     ErrProductMaxPurchaseExceeded,
	})
}

func TestBuildOrderResultRejectsProductMinPurchaseQuantityNotMet(t *testing.T) {
	assertBuildOrderResultRejectsPurchaseQuantity(t, orderPurchaseQuantityLimitFixture{
		dsnPrefix:       "order_service_purchase_min",
		categorySlug:    "test-category-min-limit",
		productSlug:     "test-product-min-limit",
		minQuantity:     3,
		requestQuantity: 2,
		expectedErr:     ErrProductMinPurchaseNotMet,
	})
}

func TestBuildOrderResultOriginalAmountBeforePromotion(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_promo_original_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductSKU{}, &models.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      "test-category-original",
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            "test-product-original",
		TitleJSON:       models.JSON{"zh-CN": "测试商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.RequireFromString("59.90")),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           models.DefaultSKUCode,
		PriceAmount:       models.NewMoneyFromDecimal(decimal.RequireFromString("59.90")),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	promotion := models.Promotion{
		Name:       "test-20-percent",
		ScopeType:  constants.ScopeTypeProduct,
		ScopeRefID: product.ID,
		Type:       constants.PromotionTypePercent,
		Value:      models.NewMoneyFromDecimal(decimal.NewFromInt(20)),
		MinAmount:  models.NewMoneyFromDecimal(decimal.Zero),
		IsActive:   true,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := db.Create(&promotion).Error; err != nil {
		t.Fatalf("create promotion failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductRepo:    repository.NewProductRepository(db),
		ProductSKURepo: repository.NewProductSKURepository(db),
		PromotionRepo:  repository.NewPromotionRepository(db),
		ExpireMinutes:  15,
	})

	result, err := svc.buildOrderResult(orderCreateParams{
		UserID: 1,
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  2,
			},
		},
	})
	if err != nil {
		t.Fatalf("buildOrderResult failed: %v", err)
	}

	expectedOriginal := decimal.RequireFromString("119.80")
	expectedPromotion := decimal.RequireFromString("23.96")
	expectedTotal := decimal.RequireFromString("95.84")

	if !result.OriginalAmount.Equal(expectedOriginal) {
		t.Fatalf("expected original amount %s, got: %s", expectedOriginal.String(), result.OriginalAmount.String())
	}
	if !result.PromotionDiscountAmount.Equal(expectedPromotion) {
		t.Fatalf("expected promotion discount amount %s, got: %s", expectedPromotion.String(), result.PromotionDiscountAmount.String())
	}
	if !result.DiscountAmount.Equal(decimal.Zero) {
		t.Fatalf("expected coupon discount amount 0, got: %s", result.DiscountAmount.String())
	}
	if !result.TotalAmount.Equal(expectedTotal) {
		t.Fatalf("expected total amount %s, got: %s", expectedTotal.String(), result.TotalAmount.String())
	}
}

func TestBuildOrderResultRejectsZeroTotalAmountAfterCoupon(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_coupon_zero_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.Product{}, &models.ProductSKU{}, &models.Coupon{}, &models.CouponUsage{}, &models.Promotion{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	category := models.Category{
		Slug:      "test-category-coupon",
		NameJSON:  models.JSON{"zh-CN": "测试分类"},
		SortOrder: 0,
		CreatedAt: now,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create category failed: %v", err)
	}

	product := models.Product{
		CategoryID:      category.ID,
		Slug:            "test-product-coupon",
		TitleJSON:       models.JSON{"zh-CN": "测试商品"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product failed: %v", err)
	}

	sku := models.ProductSKU{
		ProductID:         product.ID,
		SKUCode:           models.DefaultSKUCode,
		PriceAmount:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		IsActive:          true,
		ManualStockTotal:  constants.ManualStockUnlimited,
		ManualStockLocked: 0,
		ManualStockSold:   0,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(&sku).Error; err != nil {
		t.Fatalf("create sku failed: %v", err)
	}

	coupon := models.Coupon{
		Code:        "FREE10",
		Type:        constants.CouponTypeFixed,
		Value:       models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		MinAmount:   models.NewMoneyFromDecimal(decimal.Zero),
		MaxDiscount: models.NewMoneyFromDecimal(decimal.Zero),
		ScopeType:   constants.ScopeTypeProduct,
		ScopeRefIDs: fmt.Sprintf("[%d]", product.ID),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(&coupon).Error; err != nil {
		t.Fatalf("create coupon failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		ProductRepo:     repository.NewProductRepository(db),
		ProductSKURepo:  repository.NewProductSKURepository(db),
		CouponRepo:      repository.NewCouponRepository(db),
		CouponUsageRepo: repository.NewCouponUsageRepository(db),
		PromotionRepo:   repository.NewPromotionRepository(db),
		ExpireMinutes:   15,
	})

	_, err = svc.buildOrderResult(orderCreateParams{
		UserID:     1,
		CouponCode: "FREE10",
		Items: []CreateOrderItem{
			{
				ProductID: product.ID,
				SKUID:     sku.ID,
				Quantity:  1,
			},
		},
	})
	if !errors.Is(err, ErrInvalidOrderAmount) {
		t.Fatalf("expected invalid order amount, got: %v", err)
	}
}
