package upstream

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/provider"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestHandleGenericWebhookCallbackDeliversOrder(t *testing.T) {
	handler, db, conn, order := setupGenericWebhookCallbackTest(t, "callback-token")
	router := gin.New()
	router.POST(constants.DefaultWebhookCallbackPath, handler.HandleGenericWebhookCallback)

	body, _ := json.Marshal(map[string]interface{}{
		"event":    "order.fulfilled",
		"order_no": order.OrderNo,
		"status":   "completed",
		"fulfillment": map[string]interface{}{
			"type":    "upstream",
			"payload": "CARD----SECRET",
		},
	})
	req := httptest.NewRequest(http.MethodPost, constants.DefaultWebhookCallbackPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer callback-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", resp.Code, resp.Body.String())
	}

	var refreshed models.Order
	if err := db.First(&refreshed, order.ID).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if refreshed.Status != constants.OrderStatusCompleted {
		t.Fatalf("expected completed order, got %s", refreshed.Status)
	}
	var procurement models.ProcurementOrder
	if err := db.Where("local_order_id = ?", order.ID).First(&procurement).Error; err != nil {
		t.Fatalf("load procurement: %v", err)
	}
	if procurement.Status != constants.ProcurementStatusCompleted {
		t.Fatalf("expected completed procurement, got %s", procurement.Status)
	}
	var fulfillment models.Fulfillment
	if err := db.Where("order_id = ?", order.ID).First(&fulfillment).Error; err != nil {
		t.Fatalf("load fulfillment: %v", err)
	}
	if fulfillment.Payload != "CARD----SECRET" {
		t.Fatalf("unexpected fulfillment: %#v", fulfillment)
	}

	// Repeated delivery callbacks are idempotent.
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, constants.DefaultWebhookCallbackPath, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer callback-token")
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("unexpected repeated callback status %d: %s", resp.Code, resp.Body.String())
	}
	var count int64
	db.Model(&models.Fulfillment{}).Where("order_id = ?", order.ID).Count(&count)
	if count != 1 {
		t.Fatalf("expected one fulfillment, got %d", count)
	}
	_ = conn
}

func TestHandleGenericWebhookCallbackAuthenticationAndConnectionIsolation(t *testing.T) {
	handler, _, _, order := setupGenericWebhookCallbackTest(t, "token-one")
	router := gin.New()
	router.POST("/callback", handler.HandleGenericWebhookCallback)
	body := []byte(fmt.Sprintf(`{"event":"order.fulfilled","order_no":%q,"status":"delivered","fulfillment":{"payload":"x"}}`, order.OrderNo))

	req := httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer wrong-token")
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.Code)
	}

	other, err := handler.SiteConnectionService.Create(service.CreateConnectionInput{
		Name:        "other-webhook",
		BaseURL:     "https://other.example.com/orders",
		ApiKey:      "token-two",
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	})
	if err != nil {
		t.Fatalf("create second connection: %v", err)
	}
	if err := handler.SiteConnectionService.SetStatus(other.ID, constants.ConnectionStatusActive); err != nil {
		t.Fatalf("activate second connection: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "/callback", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer token-two")
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected connection-isolated not found, got %d: %s", resp.Code, resp.Body.String())
	}
}

func setupGenericWebhookCallbackTest(t *testing.T, token string) (*Handler, *gorm.DB, *models.SiteConnection, *models.Order) {
	t.Helper()
	dsn := fmt.Sprintf("file:generic_webhook_callback_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(
		&models.SiteConnection{}, &models.Order{}, &models.OrderItem{},
		&models.Fulfillment{}, &models.ProcurementOrder{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	models.DB = db
	connRepo := repository.NewSiteConnectionRepository(db)
	connSvc := service.NewSiteConnectionService(connRepo, "test-app-secret", t.TempDir())
	conn, err := connSvc.Create(service.CreateConnectionInput{
		Name:        "webhook",
		BaseURL:     "https://provider.example.com/orders",
		ApiKey:      token,
		Protocol:    constants.ConnectionProtocolGenericWebhook,
		CallbackURL: "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
	})
	if err != nil {
		t.Fatalf("create connection: %v", err)
	}
	if err := connSvc.SetStatus(conn.ID, constants.ConnectionStatusActive); err != nil {
		t.Fatalf("activate connection: %v", err)
	}
	conn.Status = constants.ConnectionStatusActive

	order := &models.Order{
		OrderNo: "CALLBACK-ORDER-001", UserID: 1, Status: constants.OrderStatusFulfilling, Currency: "CNY",
		OriginalAmount: models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		TotalAmount:    models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
	}
	if err := db.Create(order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}
	if err := db.Create(&models.OrderItem{
		OrderID: order.ID, ProductID: 1, SKUID: 1, Quantity: 1,
		FulfillmentType: constants.FulfillmentTypeUpstream,
		TitleJSON:       models.JSON{"zh-CN": "Test"},
		UnitPrice:       order.TotalAmount, TotalPrice: order.TotalAmount,
	}).Error; err != nil {
		t.Fatalf("create item: %v", err)
	}
	if err := db.Create(&models.ProcurementOrder{
		ConnectionID: conn.ID, LocalOrderID: order.ID, LocalOrderNo: order.OrderNo,
		Status: constants.ProcurementStatusAccepted, LocalSellAmount: order.TotalAmount, Currency: "CNY",
	}).Error; err != nil {
		t.Fatalf("create procurement: %v", err)
	}
	procSvc := service.NewProcurementOrderService(
		repository.NewProcurementOrderRepository(db), repository.NewOrderRepository(db),
		repository.NewFulfillmentRepository(db), repository.NewProductMappingRepository(db),
		repository.NewSKUMappingRepository(db), connSvc, nil, nil, config.EmailConfig{}, nil,
	)
	container := &provider.Container{SiteConnectionService: connSvc, ProcurementOrderService: procSvc}
	return New(container, nil), db, conn, order
}
