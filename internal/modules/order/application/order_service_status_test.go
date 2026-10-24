package application

import (
	"fmt"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestCalcParentStatus(t *testing.T) {
	children := []orderdomain.Order{
		{Status: constants.OrderStatusDelivered},
		{Status: constants.OrderStatusPaid},
	}
	status := CalcParentStatus(children, constants.OrderStatusPaid)
	if status != constants.OrderStatusPartiallyDelivered {
		t.Fatalf("expected partially_delivered, got %s", status)
	}

	children = []orderdomain.Order{
		{Status: constants.OrderStatusDelivered},
		{Status: constants.OrderStatusDelivered},
	}
	status = CalcParentStatus(children, constants.OrderStatusPaid)
	if status != constants.OrderStatusDelivered {
		t.Fatalf("expected delivered, got %s", status)
	}
}

func TestCalcParentStatusAllRefunded(t *testing.T) {
	children := []orderdomain.Order{
		{Status: constants.OrderStatusRefunded},
		{Status: constants.OrderStatusRefunded},
	}
	status := CalcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusRefunded {
		t.Fatalf("expected refunded, got %s", status)
	}
}

func TestCalcParentStatusPartiallyRefunded(t *testing.T) {
	children := []orderdomain.Order{
		{Status: constants.OrderStatusRefunded},
		{Status: constants.OrderStatusDelivered},
	}
	status := CalcParentStatus(children, constants.OrderStatusDelivered)
	if status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected partially_refunded, got %s", status)
	}
}

func TestExpectedRefundStatus(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name   string
		order  orderdomain.Order
		expect string
	}{
		{
			name: "partial refund",
			order: orderdomain.Order{
				Status:         constants.OrderStatusCompleted,
				PaidAt:         &now,
				TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: money.FromDecimal(decimal.NewFromInt(30)),
			},
			expect: constants.OrderStatusPartiallyRefunded,
		},
		{
			name: "full refund",
			order: orderdomain.Order{
				Status:         constants.OrderStatusCompleted,
				PaidAt:         &now,
				TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: money.FromDecimal(decimal.NewFromInt(100)),
			},
			expect: constants.OrderStatusRefunded,
		},
		{
			name: "canceled should keep",
			order: orderdomain.Order{
				Status:         constants.OrderStatusCanceled,
				PaidAt:         &now,
				TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
				RefundedAmount: money.FromDecimal(decimal.NewFromInt(100)),
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
	order := &orderdomain.Order{
		Status:         constants.OrderStatusCompleted,
		PaidAt:         &now,
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(40)),
		RefundedAmount: money.FromDecimal(decimal.NewFromInt(10)),
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusCompleted},
			{Status: constants.OrderStatusCompleted},
		},
	}
	if got := resolvedParentStatus(order); got != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected partially_refunded, got %s", got)
	}
}

func TestUpdateOrderStatusRejectsManualPaidTransition(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_reject_manual_paid_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	now := time.Now()
	order := &orderdomain.Order{
		OrderNo:        "MANUAL-PAID-MUST-FAIL",
		Status:         constants.OrderStatusPendingPayment,
		Currency:       "CNY",
		TotalAmount:    money.FromDecimal(decimal.NewFromInt(100)),
		OriginalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	svc := NewOrderService(OrderServiceOptions{OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")})

	if _, err := svc.UpdateOrderStatus(order.ID, constants.OrderStatusPaid); err != ErrOrderStatusInvalid {
		t.Fatalf("manual paid transition error = %v, want %v", err, ErrOrderStatusInvalid)
	}
	var stored orderdomain.Order
	if err := db.First(&stored, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	if stored.Status != constants.OrderStatusPendingPayment || stored.PaidAt != nil {
		t.Fatalf("manual paid transition mutated order: %+v", stored)
	}
}

func TestIsTransitionAllowedRefunded(t *testing.T) {
	if !IsTransitionAllowed(constants.OrderStatusDelivered, constants.OrderStatusPartiallyRefunded) {
		t.Fatalf("expected delivered to partially_refunded transition to be allowed")
	}
	if !IsTransitionAllowed(constants.OrderStatusPartiallyRefunded, constants.OrderStatusRefunded) {
		t.Fatalf("expected partially_refunded to refunded transition to be allowed")
	}
	if !IsTransitionAllowed(constants.OrderStatusDelivered, constants.OrderStatusRefunded) {
		t.Fatalf("expected delivered to refunded transition to be allowed")
	}
	if !IsTransitionAllowed(constants.OrderStatusCompleted, constants.OrderStatusRefunded) {
		t.Fatalf("expected completed to refunded transition to be allowed")
	}
	if IsTransitionAllowed(constants.OrderStatusCanceled, constants.OrderStatusRefunded) {
		t.Fatalf("expected canceled to refunded transition to be rejected")
	}
}

func TestUpdateOrderStatusParentToPartiallyRefundedSyncsChildren(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_parent_partial_refund_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	now := time.Now()
	paidAt := now
	parent := &orderdomain.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001",
		UserID:           0,
		Status:           constants.OrderStatusDelivered,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		RefundedAmount:   money.FromDecimal(decimal.NewFromInt(30)),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}

	childA := &orderdomain.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001-A",
		ParentID:         &parent.ID,
		UserID:           0,
		Status:           constants.OrderStatusDelivered,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(60)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(60)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(60)),
		RefundedAmount:   money.FromDecimal(decimal.Zero),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(childA).Error; err != nil {
		t.Fatalf("create childA order failed: %v", err)
	}

	childB := &orderdomain.Order{
		OrderNo:          "PARENT-PARTIAL-REFUND-001-B",
		ParentID:         &parent.ID,
		UserID:           0,
		Status:           constants.OrderStatusCompleted,
		Currency:         "CNY",
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(40)),
		DiscountAmount:   money.FromDecimal(decimal.Zero),
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(40)),
		WalletPaidAmount: money.FromDecimal(decimal.Zero),
		OnlinePaidAmount: money.FromDecimal(decimal.NewFromInt(40)),
		RefundedAmount:   money.FromDecimal(decimal.Zero),
		PaidAt:           &paidAt,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(childB).Error; err != nil {
		t.Fatalf("create childB order failed: %v", err)
	}

	svc := NewOrderService(OrderServiceOptions{
		OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes"),
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

	var reloadedA orderdomain.Order
	if err := db.First(&reloadedA, childA.ID).Error; err != nil {
		t.Fatalf("reload childA failed: %v", err)
	}
	if reloadedA.Status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected childA partially_refunded, got: %s", reloadedA.Status)
	}
	var reloadedB orderdomain.Order
	if err := db.First(&reloadedB, childB.ID).Error; err != nil {
		t.Fatalf("reload childB failed: %v", err)
	}
	if reloadedB.Status != constants.OrderStatusPartiallyRefunded {
		t.Fatalf("expected childB partially_refunded, got: %s", reloadedB.Status)
	}
}

func TestCanCompleteParentOrder(t *testing.T) {
	order := &orderdomain.Order{
		Status: constants.OrderStatusDelivered,
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusDelivered},
			{Status: constants.OrderStatusCompleted},
		},
	}
	if !canCompleteParentOrder(order) {
		t.Fatalf("expected delivered parent order to be completable")
	}
}

func TestCanCompleteParentOrderRejectInvalidStatus(t *testing.T) {
	order := &orderdomain.Order{
		Status: constants.OrderStatusPartiallyDelivered,
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusDelivered},
		},
	}
	if canCompleteParentOrder(order) {
		t.Fatalf("expected partially_delivered parent order to be rejected")
	}
}

func TestCanCompleteParentOrderRejectInvalidChild(t *testing.T) {
	order := &orderdomain.Order{
		Status: constants.OrderStatusDelivered,
		Children: []orderdomain.Order{
			{Status: constants.OrderStatusPaid},
		},
	}
	if canCompleteParentOrder(order) {
		t.Fatalf("expected parent order with paid child to be rejected")
	}
}

func TestUpdateOrderStatusRejectsCancelingPaidParent(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_reject_cancel_paid_parent_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	now := time.Now()
	parent := &orderdomain.Order{
		OrderNo:          "PAID-PARENT-NO-CANCEL",
		Status:           constants.OrderStatusDelivered,
		Currency:         "CNY",
		TotalAmount:      money.FromDecimal(decimal.NewFromInt(100)),
		OriginalAmount:   money.FromDecimal(decimal.NewFromInt(100)),
		WalletPaidAmount: money.FromDecimal(decimal.NewFromInt(100)),
		PaidAt:           &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent failed: %v", err)
	}
	child := &orderdomain.Order{
		OrderNo:     "PAID-PARENT-NO-CANCEL-A",
		ParentID:    &parent.ID,
		Status:      constants.OrderStatusDelivered,
		Currency:    "CNY",
		TotalAmount: money.FromDecimal(decimal.NewFromInt(100)),
		PaidAt:      &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.Create(child).Error; err != nil {
		t.Fatalf("create child failed: %v", err)
	}
	svc := NewOrderService(OrderServiceOptions{OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")})

	if _, err := svc.UpdateOrderStatus(parent.ID, constants.OrderStatusCanceled); err != ErrOrderStatusInvalid {
		t.Fatalf("cancel paid parent error = %v, want %v", err, ErrOrderStatusInvalid)
	}
	var stored orderdomain.Order
	if err := db.First(&stored, parent.ID).Error; err != nil {
		t.Fatalf("reload parent failed: %v", err)
	}
	if stored.Status != constants.OrderStatusDelivered {
		t.Fatalf("paid parent status mutated to %s", stored.Status)
	}
}

func TestUpdateOrderStatusRejectsCancelingChildOrder(t *testing.T) {
	dsn := fmt.Sprintf("file:order_service_reject_cancel_child_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &fulfillmentdomain.Fulfillment{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	now := time.Now()
	parent := &orderdomain.Order{OrderNo: "CHILD-CANCEL-PARENT", Status: constants.OrderStatusPendingPayment, Currency: "CNY", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent failed: %v", err)
	}
	child := &orderdomain.Order{OrderNo: "CHILD-CANCEL-PARENT-A", ParentID: &parent.ID, Status: constants.OrderStatusPendingPayment, Currency: "CNY", CreatedAt: now, UpdatedAt: now}
	if err := db.Create(child).Error; err != nil {
		t.Fatalf("create child failed: %v", err)
	}
	svc := NewOrderService(OrderServiceOptions{OrderStore: ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")})
	if _, err := svc.UpdateOrderStatus(child.ID, constants.OrderStatusCanceled); err != ErrOrderStatusInvalid {
		t.Fatalf("cancel child error = %v, want %v", err, ErrOrderStatusInvalid)
	}
}
