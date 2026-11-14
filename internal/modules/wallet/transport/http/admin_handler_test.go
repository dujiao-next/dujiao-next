package wallethttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

func TestAdjustUserWalletRequiresRemarkWithDedicatedMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("admin_id", uint(1))
	ctx.Params = gin.Params{{Key: "id", Value: "10"}}
	ctx.Request = httptest.NewRequest(
		http.MethodPost,
		"/api/v1/admin/users/10/wallet/adjust",
		strings.NewReader(`{"operation":"add","amount":"10.00","remark":"   "}`),
	)
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Accept-Language", "en-US")

	(&AdminHandler{}).AdjustUserWallet(ctx)

	if !strings.Contains(recorder.Body.String(), "A remark is required for wallet balance adjustments") {
		t.Fatalf("expected dedicated remark validation message, got %s", recorder.Body.String())
	}
}

type rechargeRefundStub struct {
	AdminWalletService
	called int
	input  RechargeRefundInput
	err    error
}

func (s *rechargeRefundStub) RefundRecharge(input RechargeRefundInput) error {
	s.called++
	s.input = input
	return s.err
}

func TestRefundRechargeValidatesPrecisionAndReportsInsufficientBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		amount      string
		wantCalls   int
		wantMessage string
		err         error
	}{
		{"0.001", 0, "Bad request", ErrInsufficientBalance}, {"-1", 0, "Bad request", ErrInsufficientBalance}, {"20.00", 1, "Insufficient wallet balance to refund", ErrInsufficientBalance},
		{"20.00", 1, "Order exceeded the maximum refundable period", ErrRechargeRefundExpired},
	} {
		t.Run(tc.amount, func(t *testing.T) {
			stub := &rechargeRefundStub{err: tc.err}
			handler := &AdminHandler{wallets: stub}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("admin_id", uint(7))
			ctx.Params = gin.Params{{Key: "id", Value: "10"}}
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/wallet/recharges/10/manual-refund", strings.NewReader(`{"amount":"`+tc.amount+`","payment_fee_refunded":true,"request_id":"f4d13d00-0000-4000-8000-000000000001"}`))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("Accept-Language", "en-US")
			handler.RefundRecharge(ctx)
			if stub.called != tc.wantCalls {
				t.Fatalf("calls: %d body: %s", stub.called, recorder.Body.String())
			}
			if tc.wantCalls > 0 && !strings.Contains(recorder.Body.String(), tc.wantMessage) {
				t.Fatalf("message: %s", recorder.Body.String())
			}
			if stub.called > 0 && (stub.input.AdminID != 7 || stub.input.RechargeID != 10 || !stub.input.PaymentFeeRefunded) {
				t.Fatalf("wrong identity: %+v", stub.input)
			}
		})
	}
}

type rechargeDetailWalletStub struct {
	AdminWalletService
	status, refunded, balance string
	expired                   bool
}

func (s *rechargeDetailWalletStub) GetRechargeDetail(id uint) (*walletdomain.RechargeOrder, *paymentdomain.Payment, error) {
	return &walletdomain.RechargeOrder{ID: id, UserID: 7, ChannelID: 2, Status: s.status, Amount: money.FromDecimal(decimal.NewFromInt(100)), RefundedAmount: money.FromDecimal(decimal.RequireFromString(s.refunded))}, nil, nil
}
func (s *rechargeDetailWalletStub) ListRechargeRefunds(id uint) ([]walletdomain.Transaction, error) {
	return nil, nil
}

// IsRechargeRefundWindowExpired 返回测试指定的退款期限状态。
func (s *rechargeDetailWalletStub) IsRechargeRefundWindowExpired(recharge *walletdomain.RechargeOrder) (bool, error) {
	return s.expired, nil
}
func (s *rechargeDetailWalletStub) GetAccount(id uint) (*walletdomain.Account, error) {
	return &walletdomain.Account{UserID: id, Balance: money.FromDecimal(decimal.RequireFromString(s.balance))}, nil
}

type rechargeDetailUsersStub struct{ AdminUserReader }

func (rechargeDetailUsersStub) ListByIDs(ids []uint) ([]userdomain.User, error) { return nil, nil }

type rechargeDetailChannelsStub struct{ PaymentChannelReader }

func (rechargeDetailChannelsStub) ListByIDs(ids []uint) ([]paymentdomain.PaymentChannel, error) {
	return nil, nil
}

func TestGetRechargeDetailLimitsRefundableAmountByOrderAndWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, status, refunded, balance, want string
		expired                               bool
	}{
		{"wallet_balance", "success", "0", "20", "20", false},
		{"remaining_recharge", "partially_refunded", "60", "100", "40", false},
		{"partial_with_low_balance", "partially_refunded", "60", "15", "15", false},
		{"original_recharge", "success", "0", "200", "100", false},
		{"empty_wallet", "success", "0", "0", "0", false},
		{"fully_refunded", "refunded", "100", "200", "0", false},
		{"expired", "success", "0", "200", "0", true},
		{"partial_expired", "partially_refunded", "60", "200", "0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := &AdminHandler{wallets: &rechargeDetailWalletStub{status: tc.status, refunded: tc.refunded, balance: tc.balance, expired: tc.expired}, users: rechargeDetailUsersStub{}, channels: rechargeDetailChannelsStub{}}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Params = gin.Params{{Key: "id", Value: "10"}}
			ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/wallet/recharges/10", nil)
			handler.GetRechargeDetail(ctx)
			if recorder.Code != http.StatusOK {
				t.Fatalf("response: %s", recorder.Body.String())
			}
			var result struct {
				Data struct {
					RefundableAmount    money.Amount `json:"refundable_amount"`
					RefundWindowExpired bool         `json:"refund_window_expired"`
				} `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if !result.Data.RefundableAmount.Decimal.Equal(decimal.RequireFromString(tc.want)) {
				t.Fatalf("refundable amount: %s", result.Data.RefundableAmount)
			}
			if result.Data.RefundWindowExpired != tc.expired {
				t.Fatalf("refund window expired: %v", result.Data.RefundWindowExpired)
			}
		})
	}
}

type rechargeConfirmStub struct {
	AdminWalletService
	called int
	input  ConfirmRechargeInput
}

func (s *rechargeConfirmStub) ConfirmRecharge(input ConfirmRechargeInput) error {
	s.called++
	s.input = input
	return nil
}

func TestConfirmRechargeRequiresPaymentAndPassesProviderRef(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		calls      int
	}{
		{"missing_payment", `{"remark":"verified"}`, 0},
		{"selected_payment", `{"payment_id":20,"provider_ref":"TX-HASH-001","remark":"verified"}`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &rechargeConfirmStub{}
			handler := &AdminHandler{wallets: stub}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("admin_id", uint(7))
			ctx.Params = gin.Params{{Key: "id", Value: "10"}}
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/wallet/recharges/10/manual-confirm-payment", strings.NewReader(tc.body))
			ctx.Request.Header.Set("Content-Type", "application/json")
			handler.ConfirmRecharge(ctx)
			if stub.called != tc.calls {
				t.Fatalf("calls: %d, response: %s", stub.called, recorder.Body.String())
			}
			if tc.calls != 0 && (stub.input.RechargeID != 10 || stub.input.PaymentID != 20 || stub.input.AdminID != 7 || stub.input.ProviderRef != "TX-HASH-001" || stub.input.Remark != "verified") {
				t.Fatalf("input: %+v", stub.input)
			}
		})
	}
}
