package upstreamhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	downstreamcallbackdomain "github.com/dujiao-next/internal/modules/downstreamcallback/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/money"
	upstreamadapter "github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

func TestMapCallbackStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "delivered keep delivered", input: "delivered", expect: "delivered"},
		{name: "completed map delivered", input: "completed", expect: "delivered"},
		{name: "fulfilled map delivered", input: "fulfilled", expect: "delivered"},
		{name: "canceled keep canceled", input: "canceled", expect: "canceled"},
		{name: "cancelled map canceled", input: "cancelled", expect: "canceled"},
		{name: "refunded keep refunded", input: "refunded", expect: "refunded"},
		{name: "partially refunded keep value", input: "partially_refunded", expect: "partially_refunded"},
		{name: "trim and lower", input: "  ReFuNdEd  ", expect: "refunded"},
		{name: "fallback normalized raw", input: "PROCESSING", expect: "processing"},
	}

	for _, tc := range tests {
		got := mapCallbackStatus(tc.input)
		if got != tc.expect {
			t.Fatalf("%s: want %q got %q", tc.name, tc.expect, got)
		}
	}
}

type stubConnections struct {
	conn *siteconnectiondomain.Connection
}

func (s stubConnections) GetByApiKey(string) (*siteconnectiondomain.Connection, error) {
	return s.conn, nil
}

type stubSecrets struct{}

func (stubSecrets) DecryptSecret(encrypted string) (string, error) { return encrypted, nil }

type stubProcurements struct {
	order   *procurementdomain.Order
	handled bool
}

func (s *stubProcurements) GetByLocalOrderNo(string) (*procurementdomain.Order, error) {
	return s.order, nil
}

func (s *stubProcurements) HandleUpstreamCallback(uint, string, *procurementcontract.Fulfillment) error {
	s.handled = true
	return nil
}

// TestHandleCallbackOwnership 保证回调只能由采购单所属连接、且上游订单号一致时才被受理。
func TestHandleCallbackOwnership(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name         string
		connectionID uint
		upstreamID   uint
		payloadID    uint
		wantHandled  bool
	}{
		{name: "同连接同上游单号放行", connectionID: 1, upstreamID: 88, payloadID: 88, wantHandled: true},
		{name: "上游单号未落库时只校验连接", connectionID: 1, upstreamID: 0, payloadID: 88, wantHandled: true},
		{name: "其它连接冒用本地订单号", connectionID: 2, upstreamID: 88, payloadID: 88, wantHandled: false},
		{name: "上游订单号不匹配", connectionID: 1, upstreamID: 88, payloadID: 99, wantHandled: false},
	}

	for _, tc := range cases {
		procurements := &stubProcurements{order: &procurementdomain.Order{
			ID:              7,
			ConnectionID:    tc.connectionID,
			LocalOrderNo:    "LOCAL-1",
			UpstreamOrderID: tc.upstreamID,
		}}
		handler := &Handler{Dependencies: Dependencies{
			Connections: stubConnections{conn: &siteconnectiondomain.Connection{
				ID: 1, ApiKey: "key-1", ApiSecret: "secret-1", Status: "active",
			}},
			ConnectionSecrets: stubSecrets{},
			Procurements:      procurements,
		}}

		timestamp := time.Now().Unix()
		body, _ := json.Marshal(callbackPayload{
			Event:             "order.fulfilled",
			OrderID:           tc.payloadID,
			DownstreamOrderNo: "LOCAL-1",
			Status:            "delivered",
			Timestamp:         timestamp,
		})

		request := httptest.NewRequest(http.MethodPost, "/api/v1/upstream/callback", bytes.NewReader(body))
		request.Header.Set(upstreamadapter.HeaderApiKey, "key-1")
		request.Header.Set(upstreamadapter.HeaderTimestamp, fmt.Sprintf("%d", timestamp))
		request.Header.Set(upstreamadapter.HeaderSignature,
			upstreamadapter.Sign("secret-1", http.MethodPost, "/api/v1/upstream/callback", timestamp, body))

		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = request
		handler.HandleCallback(c)

		if procurements.handled != tc.wantHandled {
			t.Fatalf("%s: handled=%v want %v (body %s)", tc.name, procurements.handled, tc.wantHandled, recorder.Body.String())
		}
	}
}

type formTestProducts struct{ product productdomain.Product }

func (s formTestProducts) GetByID(string) (*productdomain.Product, error) {
	p := s.product
	return &p, nil
}

type formTestSKUs struct{}

func (formTestSKUs) GetByID(uint) (*productdomain.ProductSKU, error) {
	return &productdomain.ProductSKU{ID: 19, ProductID: 18, IsActive: true}, nil
}

type formTestOrders struct {
	Orders
	received CreateOrderInput
}

func (s *formTestOrders) CreateOrder(input CreateOrderInput) (*orderdomain.Order, error) {
	s.received = input
	return &orderdomain.Order{ID: 100, OrderNo: "FORM-TEST", Status: constants.OrderStatusPendingPayment}, nil
}

type formTestPayments struct{}

func (formTestPayments) CreatePayment(CreatePaymentInput) (*CreatePaymentResult, error) {
	return &CreatePaymentResult{OrderPaid: true}, nil
}

type formTestRefs struct{}

func (formTestRefs) Create(*downstreamcallbackdomain.OrderRef) error { return nil }
func (formTestRefs) GetByCredentialAndDownstreamNo(uint, string) (*downstreamcallbackdomain.OrderRef, error) {
	return nil, nil
}

type formTestSettings struct{ Settings }

func (formTestSettings) GetSiteCurrency(string) (string, error) { return "CNY", nil }

// 接收端不得按交付类型丢弃 manual_form_data：对接商品（upstream）同样可能带必填表单。
func TestCreateOrderPassesManualFormDataForAllFulfillmentTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, kind := range []string{constants.FulfillmentTypeManual, constants.FulfillmentTypeAuto, constants.FulfillmentTypeUpstream} {
		t.Run(kind, func(t *testing.T) {
			orders := &formTestOrders{}
			handler := &Handler{Dependencies: Dependencies{
				ProductRepository: formTestProducts{productdomain.Product{ID: 18, IsActive: true, FulfillmentType: kind}},
				SKUs:              formTestSKUs{}, Orders: orders, Payments: formTestPayments{},
				DownstreamRefs: formTestRefs{}, Settings: formTestSettings{},
			}}
			body, _ := json.Marshal(map[string]interface{}{
				"sku_id": 19, "quantity": 1, "manual_form_data": map[string]interface{}{"x_handle": "fixture_user"},
			})
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/upstream/orders", bytes.NewReader(body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Set(upstreamUserIDKey, uint(1))
			ctx.Set(upstreamCredentialIDKey, uint(1))
			handler.CreateOrder(ctx)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
			}
			if got := orders.received.ManualFormData["18"]["x_handle"]; got != "fixture_user" {
				t.Fatalf("manual form data dropped for %s: %#v", kind, orders.received.ManualFormData)
			}
			if !orders.received.IsApiOrder {
				t.Fatalf("expected IsApiOrder to be true for upstream order")
			}
		})
	}
}

func TestToUpstreamProductWithAgencyPrice(t *testing.T) {
	handler := &Handler{Dependencies: Dependencies{}}
	product := productdomain.Product{
		ID:                1,
		Slug:              "test-product",
		PriceAmount:       productdomain.Product{}.PriceAmount,
		AgencyPriceAmount: productdomain.Product{}.AgencyPriceAmount,
		FulfillmentType:   constants.FulfillmentTypeManual,
		SKUs: []productdomain.ProductSKU{
			{
				ID:                10,
				SKUCode:           "SKU-1",
				PriceAmount:       productdomain.Product{}.PriceAmount,
				AgencyPriceAmount: productdomain.Product{}.AgencyPriceAmount,
				IsActive:          true,
			},
		},
	}
	product.PriceAmount = money.FromDecimal(decimal.RequireFromString("100.00"))
	product.AgencyPriceAmount = money.FromDecimal(decimal.RequireFromString("60.00"))
	product.SKUs[0].PriceAmount = money.FromDecimal(decimal.RequireFromString("100.00"))
	product.SKUs[0].AgencyPriceAmount = money.FromDecimal(decimal.RequireFromString("60.00"))

	res := handler.toUpstreamProductWithMemberPrice(product, 0, nil)
	if res.PriceAmount != "60.00" {
		t.Fatalf("expected product price_amount to be 60.00, got %s", res.PriceAmount)
	}
	if res.OriginalPrice != "100.00" {
		t.Fatalf("expected product original_price to be 100.00, got %s", res.OriginalPrice)
	}
	if res.AgencyPrice != "60.00" {
		t.Fatalf("expected product agency_price to be 60.00, got %s", res.AgencyPrice)
	}
	if len(res.SKUs) != 1 {
		t.Fatalf("expected 1 sku, got %d", len(res.SKUs))
	}
	if res.SKUs[0].PriceAmount != "60.00" {
		t.Fatalf("expected sku price_amount to be 60.00, got %s", res.SKUs[0].PriceAmount)
	}
	if res.SKUs[0].OriginalPrice != "100.00" {
		t.Fatalf("expected sku original_price to be 100.00, got %s", res.SKUs[0].OriginalPrice)
	}
	if res.SKUs[0].AgencyPrice != "60.00" {
		t.Fatalf("expected sku agency_price to be 60.00, got %s", res.SKUs[0].AgencyPrice)
	}
}
