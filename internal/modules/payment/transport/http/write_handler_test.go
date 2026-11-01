package paymenthttp

import (
	"context"
	"encoding/json"
	"errors"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type cashierOrderLookup struct {
	UserOrderLookup
	calls int
}

func (s *cashierOrderLookup) GetOrderByUserOrderNoForTenant(tenant reseller.TenantContext, no string, uid uint) (*orderdomain.Order, error) {
	s.calls++
	if !tenant.IsReseller() || *tenant.ResellerID != 3 || no != "owned" || uid != 7 {
		return nil, ErrOrderNotFound
	}
	return &orderdomain.Order{ID: 11, UserID: 7, OrderNo: no, ResellerID: tenant.ResellerID}, nil
}

type cashierReaderStub struct {
	calls int
	err   error
}

func (s *cashierReaderStub) Read(_ context.Context, p, o, u uint) (paymentdomain.CashierView, error) {
	s.calls++
	if p != 5 || o != 11 || u != 7 {
		return paymentdomain.CashierView{}, errors.New("foreign payment")
	}
	return paymentdomain.NewCashierView(p, "preparing"), s.err
}

func TestCashierHTTPAuthorizesTenantAndOwnerBeforeReading(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, name := range []string{"owned", "unauthenticated", "other_user", "main_tenant", "other_tenant", "other_order", "invalid_id", "foreign_payment", "storage_error"} {
		t.Run(name, func(t *testing.T) {
			lookup, reader := &cashierOrderLookup{}, &cashierReaderStub{}
			h := &LatestHandler{userOrders: lookup, cashier: reader}
			c, w := newResponseTestContext()
			uid := uint(7)
			tenant := reseller.ResellerTenantContext("synthetic.example", 3, 8, "")
			query := "/payments/cashier?order_no=owned&payment_id=5"
			switch name {
			case "other_user":
				uid = 8
			case "main_tenant":
				tenant = reseller.MainTenantContext("synthetic.example")
			case "other_tenant":
				tenant = reseller.ResellerTenantContext("synthetic.example", 4, 8, "")
			case "other_order":
				query = "/payments/cashier?order_no=foreign&payment_id=5"
			case "invalid_id":
				query = "/payments/cashier?order_no=owned&payment_id=-1"
			case "foreign_payment":
				query = "/payments/cashier?order_no=owned&payment_id=6"
			case "storage_error":
				reader.err = errors.New("synthetic-secret-database-error")
			}
			c.Request = httptest.NewRequest(http.MethodGet, query, nil).WithContext(reseller.WithTenantContext(context.Background(), tenant))
			if name != "unauthenticated" {
				c.Set("user_id", uid)
			}
			h.GetCashier(c)
			var body response.Response
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if (body.StatusCode == 0) != (name == "owned") {
				t.Fatalf("unexpected result %s", w.Body.String())
			}
			if name != "owned" && name != "foreign_payment" && name != "storage_error" && reader.calls != 0 {
				t.Fatal("unauthorized read")
			}
			if strings.Contains(w.Body.String(), "synthetic-secret") {
				t.Fatal("error details exposed")
			}
			if w.Header().Get("Cache-Control") != "no-store, private, max-age=0" || w.Header().Get("Referrer-Policy") != "no-referrer" {
				t.Fatal("missing privacy headers")
			}
		})
	}
	engine := gin.New()
	h := &LatestHandler{}
	RegisterGuestLatestRoute(engine.Group("/guest"), h)
	RegisterUserLatestRoute(engine.Group("/user"), h)
	for _, route := range engine.Routes() {
		if route.Path == "/guest/payments/cashier" {
			t.Fatal("guest cashier registered")
		}
	}
}

func TestRespondPaymentCreateError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "gateway response invalid",
			err:  ErrPaymentGatewayResponseInvalid,
			code: response.CodeBadRequest,
			msg:  "支付网关响应异常",
		},
		{
			name: "recharge channel not allowed",
			err:  ErrPaymentChannelNotAllowedForRecharge,
			code: response.CodeBadRequest,
			msg:  "钱包充值不支持此支付渠道",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "创建支付失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newResponseTestContext()

			respondPaymentCreateError(c, tt.err)

			assertErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func TestRespondPaymentCaptureError(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name string
		err  error
		code int
		msg  string
	}{
		{
			name: "amount mismatch",
			err:  ErrPaymentAmountMismatch,
			code: response.CodeBadRequest,
			msg:  "支付金额不匹配",
		},
		{
			name: "unknown error",
			err:  errors.New("boom"),
			code: response.CodeInternal,
			msg:  "支付回调处理失败",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, recorder := newResponseTestContext()

			respondPaymentCaptureError(c, tt.err)

			assertErrorResponse(t, recorder, tt.code, tt.msg)
		})
	}
}

func newResponseTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c, recorder
}

func assertErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder, wantCode int, wantMsg string) {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("HTTP status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.StatusCode != wantCode {
		t.Fatalf("status_code = %d, want %d; body=%s", body.StatusCode, wantCode, recorder.Body.String())
	}
	if body.Msg != wantMsg {
		t.Fatalf("msg = %q, want %q; body=%s", body.Msg, wantMsg, recorder.Body.String())
	}
}
