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
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
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
		})
	}
}

type canceledRefOrders struct{ formTestOrders }

func (s *canceledRefOrders) GetOrderByUser(uint, uint) (*orderdomain.Order, error) {
	return &orderdomain.Order{ID: 50, OrderNo: "OLD", Status: constants.OrderStatusCanceled}, nil
}

type existingRefs struct{ formTestRefs }

func (existingRefs) GetByCredentialAndDownstreamNo(uint, string) (*downstreamcallbackdomain.OrderRef, error) {
	return &downstreamcallbackdomain.OrderRef{OrderID: 50}, nil
}

// 余额不足导致旧单被取消后，下游用同一单号重试必须真正重新下单，而不是拿回已取消的旧单。
func TestCreateOrderRecreatesWhenSameDownstreamNoWasCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orders := &canceledRefOrders{}
	handler := &Handler{Dependencies: Dependencies{
		ProductRepository: formTestProducts{productdomain.Product{ID: 18, IsActive: true, FulfillmentType: constants.FulfillmentTypeAuto}},
		SKUs:              formTestSKUs{}, Orders: orders, Payments: formTestPayments{},
		DownstreamRefs: existingRefs{}, Settings: formTestSettings{},
	}}
	body, _ := json.Marshal(map[string]interface{}{"sku_id": 19, "quantity": 1, "downstream_order_no": "LOCAL-1"})
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
	if orders.received.Items == nil {
		t.Fatalf("expected a new order to be created, body=%s", recorder.Body.String())
	}
}

type catalogPriceLevels struct {
	MemberLevelService
	userPrice  decimal.Decimal
	levelPrice decimal.Decimal
}

func (l catalogPriceLevels) ResolveUserPrice(userID, _, _ uint, base decimal.Decimal) (decimal.Decimal, bool) {
	if userID == 7 && l.userPrice.LessThan(base) {
		return l.userPrice, true
	}
	return base, false
}

func (l catalogPriceLevels) ResolveMemberPrice(levelID, _, _ uint, base decimal.Decimal) (decimal.Decimal, decimal.Decimal) {
	if levelID == 3 && l.levelPrice.LessThan(base) {
		return l.levelPrice, base.Sub(l.levelPrice)
	}
	return base, decimal.Zero
}

func (l catalogPriceLevels) LoadUserPriceBook(userID uint) (memberlevelapp.UserPriceBook, error) {
	book := memberlevelapp.UserPriceBook{}
	if userID == 7 {
		book["18:0"] = l.userPrice
		book["18:19"] = l.userPrice
	}
	return book, nil
}

// ResolveCatalogPrice 复刻生产规则：专属价与等级价取更低者，专属价必须严格更低。
func (l catalogPriceLevels) ResolveCatalogPrice(book memberlevelapp.UserPriceBook, levelID, productID, skuID uint, base decimal.Decimal) (decimal.Decimal, bool, bool) {
	best, hit, exclusive := base, false, false
	if levelID == 3 && l.levelPrice.LessThan(best) {
		best, hit = l.levelPrice, true
	}
	if price, ok := book.Resolve(productID, skuID, base); ok && price.LessThan(best) {
		best, hit, exclusive = price, true, true
	}
	return best, hit, exclusive
}

// 上游目录价口径必须与下单一致：专属价与会员等级价取更低者。
// 专属价比会员价贵时必须让会员价胜出，否则用户会比没有专属价时付得更多。
func TestUpstreamCatalogPriceTakesLowerOfUserPriceAndLevelPrice(t *testing.T) {
	product := productdomain.Product{
		ID:          18,
		PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		SKUs: []productdomain.ProductSKU{{
			ID: 19, ProductID: 18, IsActive: true,
			PriceAmount: money.FromDecimal(decimal.NewFromInt(100)),
		}},
	}

	t.Run("user price below level price wins", func(t *testing.T) {
		handler := &Handler{Dependencies: Dependencies{MemberLevels: catalogPriceLevels{
			userPrice:  decimal.RequireFromString("85"),
			levelPrice: decimal.RequireFromString("90"),
		}}}
		book, _ := handler.MemberLevels.LoadUserPriceBook(7)
		hit := handler.toUpstreamProductWithMemberPrice(product, book, 3, nil)
		if hit.PriceAmount != "85.00" || hit.MemberPrice != "85.00" || hit.OriginalPrice != "100.00" {
			t.Fatalf("product price = %s/%s/%s, want 85.00/85.00/100.00", hit.PriceAmount, hit.MemberPrice, hit.OriginalPrice)
		}
		if got := hit.SKUs[0]; got.PriceAmount != "85.00" || got.MemberPrice != "85.00" || got.OriginalPrice != "100.00" {
			t.Fatalf("sku price = %s/%s/%s, want 85.00/85.00/100.00", got.PriceAmount, got.MemberPrice, got.OriginalPrice)
		}
	})

	t.Run("user price above level price loses to the level price", func(t *testing.T) {
		handler := &Handler{Dependencies: Dependencies{MemberLevels: catalogPriceLevels{
			userPrice:  decimal.RequireFromString("95"),
			levelPrice: decimal.RequireFromString("90"),
		}}}
		book, _ := handler.MemberLevels.LoadUserPriceBook(7)
		got := handler.toUpstreamProductWithMemberPrice(product, book, 3, nil)
		if got.PriceAmount != "90.00" || got.SKUs[0].PriceAmount != "90.00" {
			t.Fatalf("price = %s/%s, want the cheaper level price 90.00", got.PriceAmount, got.SKUs[0].PriceAmount)
		}
	})

	t.Run("user price alone applies when there is no level", func(t *testing.T) {
		handler := &Handler{Dependencies: Dependencies{MemberLevels: catalogPriceLevels{
			userPrice:  decimal.RequireFromString("95"),
			levelPrice: decimal.RequireFromString("90"),
		}}}
		book, _ := handler.MemberLevels.LoadUserPriceBook(7)
		got := handler.toUpstreamProductWithMemberPrice(product, book, 0, nil)
		if got.PriceAmount != "95.00" || got.SKUs[0].PriceAmount != "95.00" {
			t.Fatalf("price = %s/%s, want user price 95.00", got.PriceAmount, got.SKUs[0].PriceAmount)
		}
	})

	t.Run("level price applies to a user without a user price", func(t *testing.T) {
		handler := &Handler{Dependencies: Dependencies{MemberLevels: catalogPriceLevels{
			userPrice:  decimal.RequireFromString("95"),
			levelPrice: decimal.RequireFromString("90"),
		}}}
		book, _ := handler.MemberLevels.LoadUserPriceBook(8)
		got := handler.toUpstreamProductWithMemberPrice(product, book, 3, nil)
		if got.PriceAmount != "90.00" || got.SKUs[0].PriceAmount != "90.00" {
			t.Fatalf("level fallback price = %s/%s, want 90.00", got.PriceAmount, got.SKUs[0].PriceAmount)
		}
	})

	t.Run("no discount at all", func(t *testing.T) {
		handler := &Handler{Dependencies: Dependencies{MemberLevels: catalogPriceLevels{
			userPrice:  decimal.RequireFromString("95"),
			levelPrice: decimal.RequireFromString("90"),
		}}}
		book, _ := handler.MemberLevels.LoadUserPriceBook(8)
		none := handler.toUpstreamProductWithMemberPrice(product, book, 0, nil)
		if none.PriceAmount != "100.00" || none.MemberPrice != "" || none.SKUs[0].PriceAmount != "100.00" {
			t.Fatalf("no discount price = %s member=%q, want 100.00 and empty", none.PriceAmount, none.MemberPrice)
		}
	})
}
