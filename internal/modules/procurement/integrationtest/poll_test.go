package procurement_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	ordergormstore "github.com/dujiao-next/internal/modules/order/infrastructure/gormstore"
	procurementapp "github.com/dujiao-next/internal/modules/procurement/application"
	procurementgormstore "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	procurementorder "github.com/dujiao-next/internal/modules/procurement/infrastructure/orderreader"
	procurementupstream "github.com/dujiao-next/internal/modules/procurement/infrastructure/upstreamgateway"
	siteconnectionapp "github.com/dujiao-next/internal/modules/siteconnection/application"
)

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

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, _ := connSvc.Create(siteconnectionapp.CreateInput{
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

	// 验证采购单状态 = fulfilled
	var updatedProc ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "fulfilled" {
		t.Errorf("expected procurement status 'fulfilled', got %q", updatedProc.Status)
	}

	// 验证本地订单状态 = delivered
	var updatedOrder orderdomain.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusDelivered {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusDelivered, updatedOrder.Status)
	}
}

func TestPollUpstreamStatus_FulfilledMappedToDelivered(t *testing.T) {
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

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, _ := connSvc.Create(siteconnectionapp.CreateInput{
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

	var updatedProc ProcurementOrder
	db.First(&updatedProc, proc.ID)
	if updatedProc.Status != "fulfilled" {
		t.Errorf("expected procurement status 'fulfilled', got %q", updatedProc.Status)
	}

	var updatedOrder orderdomain.Order
	db.First(&updatedOrder, order.ID)
	if updatedOrder.Status != constants.OrderStatusDelivered {
		t.Errorf("expected order status %q, got %q", constants.OrderStatusDelivered, updatedOrder.Status)
	}
}

// 巡检必须覆盖全部 accepted 采购单，而不是只看最新的一页。
func TestSyncAcceptedOrders_WalksAllPages(t *testing.T) {
	db := setupProcurementTestDB(t)
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "status": "processing"})
	}))
	defer server.Close()

	connSvc := newTestSiteConnectionService(db, "test-key", t.TempDir())
	conn, err := connSvc.Create(siteconnectionapp.CreateInput{
		Name: "test-upstream", BaseURL: server.URL, ApiKey: "key", ApiSecret: "secret",
		Protocol: constants.ConnectionProtocolDujiaoNext,
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	const total = 250
	for i := 0; i < total; i++ {
		proc := createTestProcurementOrder(t, db, conn.ID, uint(i+1), fmt.Sprintf("PROC-SWEEP-%03d", i), constants.ProcurementStatusAccepted)
		db.Model(proc).Update("upstream_order_id", 1000+i)
	}

	newTestProcurementService(db, connSvc).SyncAcceptedOrders()
	if got := calls.Load(); got != total {
		t.Fatalf("expected %d upstream checks, got %d", total, got)
	}
}

type recordingEnqueuer struct{ submitted []uint }

func (r *recordingEnqueuer) EnqueueSubmit(id uint, _ time.Duration) error {
	r.submitted = append(r.submitted, id)
	return nil
}
func (r *recordingEnqueuer) EnqueuePoll(uint, time.Duration) error { return nil }

// 入队失败（如 Redis 重启）留下的 pending 采购单必须被巡检补投，否则订单永远不发货。
func TestSyncAcceptedOrders_RequeuesStalledSubmissions(t *testing.T) {
	db := setupProcurementTestDB(t)
	order := createProcTestOrder(t, db, "PROC-STALLED-001", constants.OrderStatusPaid, constants.FulfillmentTypeUpstream)
	stalled := createTestProcurementOrder(t, db, 1, order.ID, order.OrderNo, constants.ProcurementStatusPending)
	db.Model(stalled).UpdateColumn("updated_at", time.Now().Add(-time.Hour))
	fresh := createTestProcurementOrder(t, db, 1, order.ID+1, "PROC-STALLED-002", constants.ProcurementStatusPending)

	queue := &recordingEnqueuer{}
	svc := procurementapp.NewService(procurementapp.Options{
		Repository:  procurementgormstore.New(db),
		Orders:      procurementorder.New(ordergormstore.New(db, "test-guest-credential-secret-with-32-bytes")),
		Connections: procurementupstream.New(newTestSiteConnectionService(db, "test-key", t.TempDir())),
		Queue:       queue,
	})
	svc.SyncAcceptedOrders()
	if len(queue.submitted) != 1 || queue.submitted[0] != stalled.ID {
		t.Fatalf("expected only stalled procurement %d requeued (fresh %d skipped), got %v", stalled.ID, fresh.ID, queue.submitted)
	}
}
