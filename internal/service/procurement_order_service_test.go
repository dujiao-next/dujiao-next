package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/upstream"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// ── test helpers ──

func setupProcurementTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:procurement_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Order{},
		&models.OrderItem{},
		&models.OrderRefundRecord{},
		&models.Fulfillment{},
		&models.ProcurementOrder{},
		&models.SiteConnection{},
		&models.ProductMapping{},
		&models.SKUMapping{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	models.DB = db
	return db
}

// createProcTestOrder 创建一个测试订单
func createProcTestOrder(t *testing.T, db *gorm.DB, orderNo, status, fulfillmentType string) *models.Order {
	t.Helper()
	order := &models.Order{
		OrderNo:        orderNo,
		UserID:         1,
		Status:         status,
		Currency:       "CNY",
		OriginalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := &models.OrderItem{
		OrderID:         order.ID,
		ProductID:       1,
		SKUID:           1,
		Quantity:        1,
		FulfillmentType: fulfillmentType,
		TitleJSON:       models.JSON{"zh-CN": "Test Product"},
		UnitPrice:       models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		TotalPrice:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	// 重新加载以包含 items
	var loaded models.Order
	if err := db.Preload("Items").First(&loaded, order.ID).Error; err != nil {
		t.Fatalf("reload order failed: %v", err)
	}
	return &loaded
}

// createTestProcurementOrder 创建一个测试采购单
func createTestProcurementOrder(t *testing.T, db *gorm.DB, connID, localOrderID uint, localOrderNo, status string) *models.ProcurementOrder {
	t.Helper()
	proc := &models.ProcurementOrder{
		ConnectionID:    connID,
		LocalOrderID:    localOrderID,
		LocalOrderNo:    localOrderNo,
		Status:          status,
		LocalSellAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		Currency:        "CNY",
		TraceID:         "test-trace-id",
	}
	if err := db.Create(proc).Error; err != nil {
		t.Fatalf("create procurement order failed: %v", err)
	}
	return proc
}

func newTestProcurementService(
	db *gorm.DB,
	connSvc *SiteConnectionService,
) *ProcurementOrderService {
	svc := NewProcurementOrderService(
		repository.NewProcurementOrderRepository(db),
		repository.NewOrderRepository(db),
		repository.NewFulfillmentRepository(db),
		repository.NewProductMappingRepository(db),
		repository.NewSKUMappingRepository(db),
		connSvc,
		nil, // queueClient
		nil, // settingService
		config.EmailConfig{},
		nil, // fulfillmentService
	)
	return svc
}

type procurementCallbackStatusFixture struct {
	orderNo                   string
	initialOrderStatus        string
	initialProcurementStatus  string
	callbackStatus            string
	expectedProcurementStatus string
	expectedOrderStatus       string
}

func assertProcurementCallbackStatus(t *testing.T, fixture procurementCallbackStatusFixture) {
	t.Helper()
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, fixture.orderNo, fixture.initialOrderStatus, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, fixture.initialProcurementStatus)

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.HandleUpstreamCallback(proc.ID, fixture.callbackStatus, nil); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	var updatedProc models.ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != fixture.expectedProcurementStatus {
		t.Errorf("expected procurement status %q, got %q", fixture.expectedProcurementStatus, updatedProc.Status)
	}

	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != fixture.expectedOrderStatus {
		t.Errorf("expected order status %q, got %q", fixture.expectedOrderStatus, updatedOrder.Status)
	}
}

// ── Phase 1 tests: order rollback on procurement failure ──

func TestRejectProcurement_RollsBackOrderStatus(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-REJECT-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "pending")

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	svc.rejectProcurement(proc, "connection not found")

	// 验证采购单状态 = rejected
	var updatedProc models.ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != "rejected" {
		t.Errorf("expected procurement status 'rejected', got %q", updatedProc.Status)
	}

	// 验证本地订单状态从 fulfilling 回退到 paid
	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestHandleUpstreamCallback_Canceled_RollsBackOrder(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-CANCEL-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "accepted")

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.HandleUpstreamCallback(proc.ID, "canceled", nil); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	// 验证采购单状态 = canceled
	var updatedProc models.ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != "canceled" {
		t.Errorf("expected procurement status 'canceled', got %q", updatedProc.Status)
	}

	// 验证本地订单状态从 fulfilling 回退到 paid
	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestHandleUpstreamCallback_Delivered_CreatesFulfillment(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-DELIVER-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, "accepted")

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	now := time.Now()
	fulfillment := &upstream.UpstreamFulfillment{
		Type:        constants.FulfillmentTypeUpstream,
		Status:      constants.FulfillmentStatusDelivered,
		Payload:     "CDK-001\nCDK-002",
		DeliveredAt: &now,
	}

	if err := svc.HandleUpstreamCallback(proc.ID, "delivered", fulfillment); err != nil {
		t.Fatalf("HandleUpstreamCallback: %v", err)
	}

	// 验证采购单状态保持 delivered
	var updatedProc models.ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != constants.ProcurementStatusDelivered {
		t.Errorf("expected procurement status %q, got %q", constants.ProcurementStatusDelivered, updatedProc.Status)
	}

	// 验证本地订单状态 = delivered
	var updatedOrder models.Order
	if err := db.First(&updatedOrder, order.ID).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}
	if updatedOrder.Status != constants.OrderStatusDelivered {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusDelivered, updatedOrder.Status)
	}

	// 验证 Fulfillment 记录已创建
	var ff models.Fulfillment
	if err := db.Where("order_id = ?", order.ID).First(&ff).Error; err != nil {
		t.Fatalf("expected fulfillment record to exist: %v", err)
	}
	if ff.Payload != "CDK-001\nCDK-002" {
		t.Errorf("unexpected fulfillment payload: %q", ff.Payload)
	}
	if ff.Type != constants.FulfillmentTypeUpstream {
		t.Errorf("expected fulfillment type %q, got %q", constants.FulfillmentTypeUpstream, ff.Type)
	}
}

func TestHandleUpstreamCallback_PartiallyRefunded_AfterFulfilledUpdatesProcurementStatus(t *testing.T) {
	assertProcurementCallbackStatus(t, procurementCallbackStatusFixture{
		orderNo:                   "PROC-REFUND-KEEP-001",
		initialOrderStatus:        constants.OrderStatusDelivered,
		initialProcurementStatus:  constants.ProcurementStatusFulfilled,
		callbackStatus:            "partially_refunded",
		expectedProcurementStatus: constants.ProcurementStatusPartiallyRefunded,
		expectedOrderStatus:       constants.OrderStatusDelivered,
	})
}

func TestHandleUpstreamCallback_PartiallyRefunded_WhileFulfillingKeepsOrderStatus(t *testing.T) {
	assertProcurementCallbackStatus(t, procurementCallbackStatusFixture{
		orderNo:                   "PROC-REFUND-FULFILLING-001",
		initialOrderStatus:        constants.OrderStatusFulfilling,
		initialProcurementStatus:  constants.ProcurementStatusAccepted,
		callbackStatus:            "partially_refunded",
		expectedProcurementStatus: constants.ProcurementStatusPartiallyRefunded,
		expectedOrderStatus:       constants.OrderStatusFulfilling,
	})
}

func TestHandleUpstreamCallback_Refunded_AfterCompletedKeepsOrderStatus(t *testing.T) {
	assertProcurementCallbackStatus(t, procurementCallbackStatusFixture{
		orderNo:                   "PROC-REFUND-COMPLETED-001",
		initialOrderStatus:        constants.OrderStatusCompleted,
		initialProcurementStatus:  constants.ProcurementStatusFulfilled,
		callbackStatus:            "refunded",
		expectedProcurementStatus: constants.ProcurementStatusRefunded,
		expectedOrderStatus:       constants.OrderStatusCompleted,
	})
}

func TestProcurement_GetByID_DoesNotIncludeLocalRefundRecords(t *testing.T) {
	db := setupProcurementTestDB(t)

	parent := createProcTestOrder(t, db, "PROC-PARENT-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	child := createProcTestOrder(t, db, "PROC-CHILD-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	if err := db.Model(&child).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatalf("set child parent: %v", err)
	}

	proc := createTestProcurementOrder(t, db, 1, child.ID, child.OrderNo, constants.ProcurementStatusAccepted)

	localRecord := &models.OrderRefundRecord{
		OrderID:    child.ID,
		Type:       constants.OrderRefundTypeManual,
		Amount:     models.NewMoneyFromDecimal(decimal.NewFromFloat(10.5)),
		Currency:   "CNY",
		Remark:     "local refund",
		GuestEmail: "guest-local@example.com",
	}
	if err := db.Create(localRecord).Error; err != nil {
		t.Fatalf("create local refund record: %v", err)
	}

	parentRecord := &models.OrderRefundRecord{
		OrderID:    parent.ID,
		Type:       constants.OrderRefundTypeWallet,
		Amount:     models.NewMoneyFromDecimal(decimal.NewFromFloat(7.25)),
		Currency:   "CNY",
		Remark:     "parent refund",
		GuestEmail: "guest-parent@example.com",
	}
	if err := db.Create(parentRecord).Error; err != nil {
		t.Fatalf("create parent refund record: %v", err)
	}

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	got, err := svc.GetByID(proc.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected procurement order")
	}
	if got.UpstreamRefundedAmount != "" || len(got.UpstreamRefundRecords) != 0 {
		t.Fatalf("expected no upstream refund fields, got refunded_amount=%q records=%d", got.UpstreamRefundedAmount, len(got.UpstreamRefundRecords))
	}
}

func TestProcurement_FillParentOrderNo_BackfillsLocalRefundedAmountFromParent(t *testing.T) {
	db := setupProcurementTestDB(t)

	parent := createProcTestOrder(t, db, "PROC-PARENT-REFUND-001", constants.OrderStatusPartiallyRefunded, constants.FulfillmentTypeUpstream)
	if err := db.Model(&models.Order{}).Where("id = ?", parent.ID).Updates(map[string]interface{}{
		"refunded_amount": models.NewMoneyFromDecimal(decimal.NewFromFloat(12.34)),
	}).Error; err != nil {
		t.Fatalf("set parent refunded_amount: %v", err)
	}

	child := createProcTestOrder(t, db, "PROC-CHILD-REFUND-001", constants.OrderStatusPartiallyRefunded, constants.FulfillmentTypeUpstream)
	if err := db.Model(&models.Order{}).Where("id = ?", child.ID).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatalf("set child parent: %v", err)
	}

	proc := createTestProcurementOrder(t, db, 1, child.ID, child.OrderNo, constants.ProcurementStatusAccepted)
	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	got, err := svc.GetByID(proc.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil || got.LocalOrder == nil {
		t.Fatalf("expected procurement with local_order, got %+v", got)
	}

	svc.FillParentOrderNo(got)

	if got.ParentOrderNo != parent.OrderNo {
		t.Fatalf("expected parent_order_no %q, got %q", parent.OrderNo, got.ParentOrderNo)
	}
	if got.LocalOrder.RefundedAmount.String() != "12.34" {
		t.Fatalf("expected local_order.refunded_amount 12.34, got %s", got.LocalOrder.RefundedAmount.String())
	}
}

func TestProcurement_List_BackfillsChildLocalRefundedAmountFromParent(t *testing.T) {
	db := setupProcurementTestDB(t)

	parent := createProcTestOrder(t, db, "PROC-LIST-PARENT-REFUND-001", constants.OrderStatusPartiallyRefunded, constants.FulfillmentTypeUpstream)
	if err := db.Model(&models.Order{}).Where("id = ?", parent.ID).Updates(map[string]interface{}{
		"refunded_amount": models.NewMoneyFromDecimal(decimal.NewFromFloat(8.88)),
	}).Error; err != nil {
		t.Fatalf("set parent refunded_amount: %v", err)
	}

	child := createProcTestOrder(t, db, "PROC-LIST-CHILD-REFUND-001", constants.OrderStatusPartiallyRefunded, constants.FulfillmentTypeUpstream)
	if err := db.Model(&models.Order{}).Where("id = ?", child.ID).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatalf("set child parent: %v", err)
	}

	proc := createTestProcurementOrder(t, db, 1, child.ID, child.OrderNo, constants.ProcurementStatusAccepted)
	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	orders, total, err := svc.List(repository.ProcurementOrderListFilter{
		LocalOrderNo: child.OrderNo,
		Pagination: repository.Pagination{
			Page:     1,
			PageSize: 20,
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(orders) != 1 || orders[0].ID != proc.ID {
		t.Fatalf("unexpected procurement list result: total=%d len=%d orders=%+v", total, len(orders), orders)
	}
	if orders[0].ParentOrderNo != parent.OrderNo {
		t.Fatalf("expected parent_order_no %q, got %q", parent.OrderNo, orders[0].ParentOrderNo)
	}
	if orders[0].LocalOrder == nil {
		t.Fatalf("expected local_order in list result")
	}
	if orders[0].LocalOrder.RefundedAmount.String() != "8.88" {
		t.Fatalf("expected local_order.refunded_amount 8.88, got %s", orders[0].LocalOrder.RefundedAmount.String())
	}
}

func TestProcurement_List_DoesNotIncludeLocalRefundRecords(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-LIST-REFUND-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	proc := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, constants.ProcurementStatusAccepted)

	record := &models.OrderRefundRecord{
		OrderID:  order.ID,
		Type:     constants.OrderRefundTypeManual,
		Amount:   models.NewMoneyFromDecimal(decimal.NewFromInt(12)),
		Currency: "CNY",
		Remark:   "list refund",
		UserID:   1,
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("create refund record: %v", err)
	}

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	orders, total, err := svc.List(repository.ProcurementOrderListFilter{
		LocalOrderNo: order.OrderNo,
		Pagination: repository.Pagination{
			Page:     1,
			PageSize: 20,
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(orders) != 1 || orders[0].ID != proc.ID {
		t.Fatalf("unexpected procurement list result: %+v", orders)
	}
	if orders[0].UpstreamRefundedAmount != "" || len(orders[0].UpstreamRefundRecords) != 0 {
		t.Fatalf("expected no upstream refund fields in list result, got refunded_amount=%q records=%d", orders[0].UpstreamRefundedAmount, len(orders[0].UpstreamRefundRecords))
	}
}

func TestProcurement_GetByID_SyncsUpstreamRefundStatusAndRecords(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "PROC-UPSTREAM-REFUND-001", constants.OrderStatusDelivered, constants.FulfillmentTypeUpstream)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order_id":        999,
			"order_no":        "UP-999",
			"status":          "partially_refunded",
			"amount":          "50.00",
			"refunded_amount": "10.00",
			"currency":        "CNY",
			"refund_records": []map[string]any{
				{
					"id":         101,
					"type":       "manual",
					"amount":     "10.00",
					"currency":   "CNY",
					"remark":     "upstream partial refund",
					"created_at": time.Now().Format(time.RFC3339),
				},
			},
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:      "upstream-refund",
		BaseURL:   server.URL,
		ApiKey:    "key",
		ApiSecret: "secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, constants.ProcurementStatusFulfilled)
	if err := db.Model(&models.ProcurementOrder{}).Where("id = ?", proc.ID).Updates(map[string]interface{}{
		"upstream_order_id": uint(999),
		"upstream_order_no": "UP-999",
	}).Error; err != nil {
		t.Fatalf("set upstream order info: %v", err)
	}

	svc := newTestProcurementService(db, connSvc)
	got, err := svc.GetByID(proc.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected procurement order")
	}
	if got.Status != constants.ProcurementStatusPartiallyRefunded {
		t.Fatalf("expected status %s, got %s", constants.ProcurementStatusPartiallyRefunded, got.Status)
	}
	if len(got.UpstreamRefundRecords) != 1 {
		t.Fatalf("expected 1 upstream_refund_records, got %d", len(got.UpstreamRefundRecords))
	}
	if id, ok := got.UpstreamRefundRecords[0]["id"].(int); !ok || id != 1 {
		t.Fatalf("expected upstream_refund_records[0].id = 1, got %#v", got.UpstreamRefundRecords[0]["id"])
	}
	if got.UpstreamRefundedAmount != "10.00" {
		t.Fatalf("expected upstream_refunded_amount 10.00, got %q", got.UpstreamRefundedAmount)
	}

	var refreshed models.ProcurementOrder
	if err := db.First(&refreshed, proc.ID).Error; err != nil {
		t.Fatalf("reload procurement order: %v", err)
	}
	if refreshed.Status != constants.ProcurementStatusPartiallyRefunded {
		t.Fatalf("expected persisted status %s, got %s", constants.ProcurementStatusPartiallyRefunded, refreshed.Status)
	}
}

func TestProcurement_List_SyncsUpstreamRefundStatusAndRecords(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "PROC-UPSTREAM-REFUND-LIST-001", constants.OrderStatusDelivered, constants.FulfillmentTypeUpstream)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order_id":        888,
			"order_no":        "UP-888",
			"status":          "partially_refunded",
			"amount":          "80.00",
			"refunded_amount": "8.00",
			"currency":        "CNY",
			"refund_records": []map[string]any{
				{"id": 201, "type": "wallet", "amount": "8.00", "currency": "CNY", "remark": "list upstream refund"},
			},
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:      "upstream-refund-list",
		BaseURL:   server.URL,
		ApiKey:    "key",
		ApiSecret: "secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, constants.ProcurementStatusFulfilled)
	if err := db.Model(&models.ProcurementOrder{}).Where("id = ?", proc.ID).Updates(map[string]interface{}{
		"upstream_order_id": uint(888),
		"upstream_order_no": "UP-888",
	}).Error; err != nil {
		t.Fatalf("set upstream order info: %v", err)
	}

	svc := newTestProcurementService(db, connSvc)
	orders, total, err := svc.List(repository.ProcurementOrderListFilter{
		LocalOrderNo: order.OrderNo,
		Pagination: repository.Pagination{
			Page:     1,
			PageSize: 20,
		},
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 1 || len(orders) != 1 {
		t.Fatalf("unexpected list result: total=%d len=%d", total, len(orders))
	}
	if orders[0].Status != constants.ProcurementStatusPartiallyRefunded {
		t.Fatalf("expected list status %s, got %s", constants.ProcurementStatusPartiallyRefunded, orders[0].Status)
	}
	if len(orders[0].UpstreamRefundRecords) != 1 {
		t.Fatalf("expected 1 upstream_refund_records, got %d", len(orders[0].UpstreamRefundRecords))
	}
	if id, ok := orders[0].UpstreamRefundRecords[0]["id"].(int); !ok || id != 1 {
		t.Fatalf("expected upstream_refund_records[0].id = 1, got %#v", orders[0].UpstreamRefundRecords[0]["id"])
	}
	if orders[0].UpstreamRefundedAmount != "8.00" {
		t.Fatalf("expected upstream_refunded_amount 8.00, got %q", orders[0].UpstreamRefundedAmount)
	}
}

func TestProcurement_GetByID_WithoutUpstreamRefundOmitsRefundFields(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "PROC-UPSTREAM-NO-REFUND-001", constants.OrderStatusDelivered, constants.FulfillmentTypeUpstream)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order_id":        777,
			"order_no":        "UP-777",
			"status":          "fulfilled",
			"amount":          "66.00",
			"refunded_amount": "0.00",
			"currency":        "CNY",
			"refund_records":  []map[string]any{},
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:      "upstream-no-refund",
		BaseURL:   server.URL,
		ApiKey:    "key",
		ApiSecret: "secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, constants.ProcurementStatusFulfilled)
	if err := db.Model(&models.ProcurementOrder{}).Where("id = ?", proc.ID).Updates(map[string]interface{}{
		"upstream_order_id": uint(777),
		"upstream_order_no": "UP-777",
	}).Error; err != nil {
		t.Fatalf("set upstream order info: %v", err)
	}

	svc := newTestProcurementService(db, connSvc)
	got, err := svc.GetByID(proc.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("expected procurement order")
	}
	if got.Status != constants.ProcurementStatusFulfilled {
		t.Fatalf("expected status %s, got %s", constants.ProcurementStatusFulfilled, got.Status)
	}
	if got.UpstreamRefundedAmount != "" {
		t.Fatalf("expected empty upstream_refunded_amount, got %q", got.UpstreamRefundedAmount)
	}
	if len(got.UpstreamRefundRecords) != 0 {
		t.Fatalf("expected empty upstream_refund_records, got %d", len(got.UpstreamRefundRecords))
	}

	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal procurement order failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal procurement order payload failed: %v", err)
	}
	if _, ok := decoded["upstream_refunded_amount"]; ok {
		t.Fatalf("expected upstream_refunded_amount to be omitted when no upstream refund, payload=%s", string(payload))
	}
	if _, ok := decoded["upstream_refund_records"]; ok {
		t.Fatalf("expected upstream_refund_records to be omitted when no upstream refund, payload=%s", string(payload))
	}
	if _, ok := decoded["upstream_order_id"]; ok {
		t.Fatalf("expected upstream_order_id to be omitted from procurement payload, payload=%s", string(payload))
	}
}

func TestBuildUpstreamRefundRecords_SortsByCreatedAtAscAndRenumbersID(t *testing.T) {
	records := []models.JSON{
		{
			"id":         99,
			"type":       "wallet",
			"amount":     "20.00",
			"created_at": "2026-04-12T10:00:00Z",
		},
		{
			"id":         100,
			"type":       "wallet",
			"amount":     "10.00",
			"created_at": "2026-04-12T09:00:00Z",
		},
	}

	got := buildUpstreamRefundRecords(records)
	if len(got) != 2 {
		t.Fatalf("expected 2 records, got %d", len(got))
	}
	if amount, _ := got[0]["amount"].(string); amount != "10.00" {
		t.Fatalf("expected first amount 10.00, got %#v", got[0]["amount"])
	}
	if amount, _ := got[1]["amount"].(string); amount != "20.00" {
		t.Fatalf("expected second amount 20.00, got %#v", got[1]["amount"])
	}
	if id, ok := got[0]["id"].(int); !ok || id != 1 {
		t.Fatalf("expected first id 1, got %#v", got[0]["id"])
	}
	if id, ok := got[1]["id"].(int); !ok || id != 2 {
		t.Fatalf("expected second id 2, got %#v", got[1]["id"])
	}
}

// ── SubmitToUpstream tests ──

func TestSubmitToUpstream_GenericWebhook(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "PROC-WEBHOOK-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	if err := db.Model(&models.OrderItem{}).Where("order_id = ?", order.ID).Update("sku_snapshot_json", models.JSON{"sku_code": "DEFAULT"}).Error; err != nil {
		t.Fatalf("update sku snapshot: %v", err)
	}
	pm := &models.ProductMapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 1, IsActive: true}
	if err := db.Create(pm).Error; err != nil {
		t.Fatalf("create product mapping: %v", err)
	}
	if err := db.Create(&models.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 1, UpstreamStock: -1, UpstreamIsActive: true}).Error; err != nil {
		t.Fatalf("create sku mapping: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer webhook-token" || r.Header.Get("Idempotency-Key") != order.OrderNo {
			t.Fatalf("unexpected webhook headers: %#v", r.Header)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if payload["product_id"] != float64(1) || payload["sku_id"] != float64(1) || payload["amount"] != "100.00" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if payload["callback_url"] != "https://shop.example.com/api/custom/generic-hook" {
			t.Fatalf("unexpected configured callback URL: %#v", payload["callback_url"])
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "order_no": "REMOTE-WEBHOOK-1"})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:        "generic-webhook",
		BaseURL:     server.URL,
		ApiKey:      "webhook-token",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/custom/generic-hook",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := db.Model(pm).Update("connection_id", conn.ID).Error; err != nil {
		t.Fatalf("update mapping connection: %v", err)
	}
	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, constants.ProcurementStatusPending)
	svc := newTestProcurementService(db, connSvc)
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("SubmitToUpstream: %v", err)
	}
	var updated models.ProcurementOrder
	if err := db.First(&updated, proc.ID).Error; err != nil {
		t.Fatalf("reload procurement: %v", err)
	}
	if updated.Status != constants.ProcurementStatusAccepted || updated.UpstreamOrderID != 0 || updated.UpstreamOrderNo != "REMOTE-WEBHOOK-1" {
		t.Fatalf("unexpected procurement: %#v", updated)
	}
}

func TestSubmitToUpstream_GenericWebhookImmediateCallbackPreservesDeliveredStatus(t *testing.T) {
	db := setupProcurementTestDB(t)
	parent := &models.Order{
		OrderNo:        "PROC-WEBHOOK-IMMEDIATE",
		UserID:         1,
		Status:         constants.OrderStatusFulfilling,
		Currency:       "CNY",
		OriginalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
	}
	if err := db.Create(parent).Error; err != nil {
		t.Fatalf("create parent order: %v", err)
	}
	child := createProcTestOrder(t, db, parent.OrderNo+"-01", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	if err := db.Model(child).Update("parent_id", parent.ID).Error; err != nil {
		t.Fatalf("attach child order: %v", err)
	}
	if err := db.Model(&models.OrderItem{}).Where("order_id = ?", child.ID).Update("sku_snapshot_json", models.JSON{"sku_code": "DEFAULT"}).Error; err != nil {
		t.Fatalf("update sku snapshot: %v", err)
	}

	pm := &models.ProductMapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 1, IsActive: true}
	if err := db.Create(pm).Error; err != nil {
		t.Fatalf("create product mapping: %v", err)
	}
	if err := db.Create(&models.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 1, UpstreamStock: -1, UpstreamIsActive: true}).Error; err != nil {
		t.Fatalf("create sku mapping: %v", err)
	}

	var svc *ProcurementOrderService
	var proc *models.ProcurementOrder
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}
		if payload["order_no"] != child.OrderNo {
			t.Errorf("unexpected order_no: %#v", payload["order_no"])
		}
		if err := svc.HandleUpstreamCallback(proc.ID, constants.OrderStatusDelivered, &upstream.UpstreamFulfillment{
			Type:    constants.FulfillmentTypeUpstream,
			Payload: "CARD----SECRET",
		}); err != nil {
			t.Errorf("immediate callback: %v", err)
			http.Error(w, "callback failed", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true, "order_no": "REMOTE-WEBHOOK-IMMEDIATE"})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:        "generic-webhook-immediate",
		BaseURL:     server.URL,
		ApiKey:      "webhook-immediate-token",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := db.Model(pm).Update("connection_id", conn.ID).Error; err != nil {
		t.Fatalf("update mapping connection: %v", err)
	}
	proc = createTestProcurementOrder(t, db, conn.ID, child.ID, child.OrderNo, constants.ProcurementStatusPending)
	svc = newTestProcurementService(db, connSvc)

	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("SubmitToUpstream: %v", err)
	}

	var updatedProc models.ProcurementOrder
	if err := db.First(&updatedProc, proc.ID).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if updatedProc.Status != constants.ProcurementStatusDelivered {
		t.Fatalf("procurement status = %q, want %q", updatedProc.Status, constants.ProcurementStatusDelivered)
	}
	if updatedProc.UpstreamOrderNo != "REMOTE-WEBHOOK-IMMEDIATE" {
		t.Fatalf("upstream order no = %q", updatedProc.UpstreamOrderNo)
	}

	for _, expected := range []struct {
		id   uint
		name string
	}{
		{id: parent.ID, name: "parent"},
		{id: child.ID, name: "child"},
	} {
		var order models.Order
		if err := db.First(&order, expected.id).Error; err != nil {
			t.Fatalf("load %s order: %v", expected.name, err)
		}
		if order.Status != constants.OrderStatusDelivered {
			t.Fatalf("%s order status = %q, want %q", expected.name, order.Status, constants.OrderStatusDelivered)
		}
	}

	var fulfillment models.Fulfillment
	if err := db.Where("order_id = ?", child.ID).First(&fulfillment).Error; err != nil {
		t.Fatalf("load fulfillment: %v", err)
	}
	if fulfillment.Payload != "CARD----SECRET" {
		t.Fatalf("fulfillment payload = %q", fulfillment.Payload)
	}
}

func TestGenericWebhookExplicitRejectionIsPermanent(t *testing.T) {
	if isRetryableSubmissionError(constants.ConnectionProtocolGenericWebhook, "temporary_overload") {
		t.Fatal("generic webhook ok:false response must not be retried")
	}
	if !isRetryableSubmissionError(constants.ConnectionProtocolDujiaoNext, "temporary_overload") {
		t.Fatal("dujiao-next unknown error codes should preserve existing retry behavior")
	}
}

func TestSubmitToUpstream_Success(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-SUBMIT-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	// 创建 product mapping 和 sku mapping
	pm := &models.ProductMapping{
		ConnectionID:      1,
		LocalProductID:    1,
		UpstreamProductID: 101,
		IsActive:          true,
	}
	db.Create(pm)
	sm := &models.SKUMapping{
		ProductMappingID: pm.ID,
		LocalSKUID:       1,
		UpstreamSKUID:    201,
		UpstreamIsActive: true,
	}
	db.Create(sm)

	// mock upstream server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"order_id": 999,
			"order_no": "UP-999",
			"status":   "accepted",
			"amount":   "50.00",
			"currency": "CNY",
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, err := connSvc.Create(CreateConnectionInput{
		Name:      "test-upstream",
		BaseURL:   server.URL,
		ApiKey:    "key",
		ApiSecret: "secret",
		Protocol:  constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "pending")

	svc := newTestProcurementService(db, connSvc)

	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("SubmitToUpstream: %v", err)
	}

	// 验证采购单状态 = accepted
	var updatedProc models.ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "accepted" {
		t.Errorf("expected procurement status 'accepted', got %q", updatedProc.Status)
	}
	if updatedProc.UpstreamOrderID != 999 {
		t.Errorf("expected upstream_order_id=999, got %d", updatedProc.UpstreamOrderID)
	}

	// 验证本地订单状态 = fulfilling
	var updatedOrder models.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusFulfilling {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusFulfilling, updatedOrder.Status)
	}
}

func TestSubmitToUpstream_NonRetryableError_Rejects(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-NONRETRY-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	pm := &models.ProductMapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(pm)
	sm := &models.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 201, UpstreamIsActive: true}
	db.Create(sm)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":            false,
			"error_code":    "product_out_of_stock",
			"error_message": "product out of stock",
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, _ := connSvc.Create(CreateConnectionInput{
		Name: "test-upstream", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
	})

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "pending")
	svc := newTestProcurementService(db, connSvc)

	// 不可重试错误应返回 error
	_ = svc.SubmitToUpstream(proc.ID)

	// 验证采购单状态 = rejected
	var updatedProc models.ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "rejected" {
		t.Errorf("expected procurement status 'rejected', got %q", updatedProc.Status)
	}

	// 验证本地订单状态回退到 paid
	var updatedOrder models.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q after rejection, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

func TestSubmitToUpstream_RetryableError_Retries(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-RETRY-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)
	pm := &models.ProductMapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(pm)
	sm := &models.SKUMapping{ProductMappingID: pm.ID, LocalSKUID: 1, UpstreamSKUID: 201, UpstreamIsActive: true}
	db.Create(sm)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"ok":            false,
			"error_code":    "server_error",
			"error_message": "temporary failure",
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, _ := connSvc.Create(CreateConnectionInput{
		Name: "test-upstream", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
		RetryMax: 3,
	})

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "pending")
	svc := newTestProcurementService(db, connSvc)

	// 可重试错误不应返回 error（已入队重试）
	if err := svc.SubmitToUpstream(proc.ID); err != nil {
		t.Fatalf("expected no error for retryable failure, got: %v", err)
	}

	// 验证采购单状态 = failed（而非 rejected）
	var updatedProc models.ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "failed" {
		t.Errorf("expected procurement status 'failed', got %q", updatedProc.Status)
	}
	if updatedProc.RetryCount != 1 {
		t.Errorf("expected retry_count=1, got %d", updatedProc.RetryCount)
	}
}

func TestHandleSubmitFailure_MaxRetriesExhausted(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-MAXRETRY-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn := &models.SiteConnection{
		RetryMax:       2,
		RetryIntervals: "[30,60]",
	}
	db.Create(conn)

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "failed")
	// 设置 retry_count 已达上限
	db.Model(proc).Update("retry_count", 2)

	svc := newTestProcurementService(db, connSvc)

	// 模拟调用 handleSubmitFailure（可重试但已达上限）
	_ = svc.handleSubmitFailure(proc, conn, "timeout after retries", true)

	// 验证采购单状态 = rejected
	var updatedProc models.ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "rejected" {
		t.Errorf("expected procurement status 'rejected', got %q", updatedProc.Status)
	}

	// 验证本地订单回退到 paid
	var updatedOrder models.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusPaid {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusPaid, updatedOrder.Status)
	}
}

// ── CreateForOrder tests ──

func TestCreateForOrder_SkipsNonUpstreamItems(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-SKIP-001", constants.OrderStatusPaid, constants.FulfillmentTypeAuto)

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	if err := svc.CreateForOrder(order.ID); err != nil {
		t.Fatalf("CreateForOrder: %v", err)
	}

	// 验证没有创建采购单
	var count int64
	db.Model(&models.ProcurementOrder{}).Count(&count)
	if count != 0 {
		t.Errorf("expected no procurement orders for auto fulfillment, got %d", count)
	}
}

func TestCreateForOrder_IdempotentSkipsDuplicate(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-DUP-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	pm := &models.ProductMapping{ConnectionID: 1, LocalProductID: 1, UpstreamProductID: 101, IsActive: true}
	db.Create(pm)

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	svc := newTestProcurementService(db, connSvc)

	// 第一次创建成功
	if err := svc.CreateForOrder(order.ID); err != nil {
		t.Fatalf("first CreateForOrder: %v", err)
	}

	// 第二次应该返回 ErrProcurementExists
	err := svc.CreateForOrder(order.ID)
	if err != ErrProcurementExists {
		t.Errorf("expected ErrProcurementExists on duplicate, got: %v", err)
	}
}

// ── PollUpstreamStatus test ──

func TestPollUpstreamStatus_Delivered(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-POLL-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		now := time.Now()
		json.NewEncoder(w).Encode(map[string]any{
			"order_id": 999,
			"order_no": "UP-999",
			"status":   "delivered",
			"amount":   "50.00",
			"currency": "CNY",
			"fulfillment": map[string]any{
				"type":         "auto",
				"status":       "delivered",
				"payload":      "KEY-001\nKEY-002",
				"delivered_at": now.Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, _ := connSvc.Create(CreateConnectionInput{
		Name: "poll-upstream", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
	})

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "accepted")
	db.Model(proc).Updates(map[string]interface{}{
		"upstream_order_id": uint(999),
		"upstream_order_no": "UP-999",
	})

	svc := newTestProcurementService(db, connSvc)

	if err := svc.PollUpstreamStatus(proc.ID); err != nil {
		t.Fatalf("PollUpstreamStatus: %v", err)
	}

	// 验证采购单状态保持 delivered
	var updatedProc models.ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != constants.ProcurementStatusDelivered {
		t.Errorf("expected procurement status %q, got %q", constants.ProcurementStatusDelivered, updatedProc.Status)
	}

	// 验证本地订单状态 = delivered
	var updatedOrder models.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusDelivered {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusDelivered, updatedOrder.Status)
	}
}

func TestPollUpstreamStatus_FulfilledPreserved(t *testing.T) {
	db := setupProcurementTestDB(t)

	order := createProcTestOrder(t, db, "PROC-POLL-FULLFILLED-001", constants.OrderStatusFulfilling, constants.FulfillmentTypeUpstream)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		now := time.Now()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order_id": 1001,
			"order_no": "UP-1001",
			"status":   "fulfilled",
			"amount":   "50.00",
			"currency": "CNY",
			"fulfillment": map[string]any{
				"type":         "auto",
				"status":       "delivered",
				"payload":      "KEY-003\nKEY-004",
				"delivered_at": now.Format(time.RFC3339),
			},
		})
	}))
	defer server.Close()

	connSvc := NewSiteConnectionService(repository.NewSiteConnectionRepository(db), "test-key", t.TempDir())
	conn, _ := connSvc.Create(CreateConnectionInput{
		Name: "poll-upstream-fulfilled", BaseURL: server.URL,
		ApiKey: "key", ApiSecret: "secret", Protocol: constants.ConnectionProtocolDujiaoNext,
	})

	proc := createTestProcurementOrder(t, db, conn.ID, order.ID, order.OrderNo, "accepted")
	db.Model(proc).Updates(map[string]interface{}{
		"upstream_order_id": uint(1001),
		"upstream_order_no": "UP-1001",
	})

	svc := newTestProcurementService(db, connSvc)
	if err := svc.PollUpstreamStatus(proc.ID); err != nil {
		t.Fatalf("PollUpstreamStatus: %v", err)
	}

	var updatedProc models.ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "fulfilled" {
		t.Errorf("expected procurement status 'fulfilled', got %q", updatedProc.Status)
	}

	var updatedOrder models.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusFulfilled {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusFulfilled, updatedOrder.Status)
	}
}

// ── Unit tests for pure functions ──

func TestParseRetryIntervals(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []time.Duration
	}{
		{
			name:     "empty string returns defaults",
			input:    "",
			expected: []time.Duration{30 * time.Second, 60 * time.Second, 300 * time.Second},
		},
		{
			name:     "valid array",
			input:    "[10,20,30]",
			expected: []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second},
		},
		{
			name:     "with spaces",
			input:    "[ 10 , 20 , 30 ]",
			expected: []time.Duration{10 * time.Second, 20 * time.Second, 30 * time.Second},
		},
		{
			name:     "invalid entries skipped",
			input:    "[10,abc,30]",
			expected: []time.Duration{10 * time.Second, 30 * time.Second},
		},
		{
			name:     "all invalid returns defaults",
			input:    "[abc,def]",
			expected: []time.Duration{30 * time.Second, 60 * time.Second, 300 * time.Second},
		},
		{
			name:     "negative values skipped",
			input:    "[10,-5,30]",
			expected: []time.Duration{10 * time.Second, 30 * time.Second},
		},
		{
			name:     "zero values skipped",
			input:    "[0,10]",
			expected: []time.Duration{10 * time.Second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRetryIntervals(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d intervals, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, d := range result {
				if d != tt.expected[i] {
					t.Errorf("interval[%d]: expected %v, got %v", i, tt.expected[i], d)
				}
			}
		})
	}
}

func TestIsRetryableErrorCode(t *testing.T) {
	nonRetryable := []string{
		"insufficient_balance",
		"payment_failed",
		"product_unavailable",
		"sku_unavailable",
		"invalid_request",
		"unauthorized",
		"forbidden",
		"duplicate_order",
		"product_out_of_stock",
	}
	for _, code := range nonRetryable {
		if isRetryableErrorCode(code) {
			t.Errorf("expected %q to be non-retryable", code)
		}
	}

	retryable := []string{
		"server_error",
		"timeout",
		"network_error",
		"unknown_error",
		"",
	}
	for _, code := range retryable {
		if !isRetryableErrorCode(code) {
			t.Errorf("expected %q to be retryable", code)
		}
	}

	// 测试带空格的情况
	if isRetryableErrorCode("  unauthorized  ") {
		t.Error("expected trimmed 'unauthorized' to be non-retryable")
	}
}
