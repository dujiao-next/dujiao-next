package wallethttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

type walletAdjustmentStub struct {
	AdminWalletService
	called int
	input  AdjustBalanceInput
}

// AdminAdjustBalance 记录后台余额调整请求。
func (s *walletAdjustmentStub) AdminAdjustBalance(input AdjustBalanceInput) (*walletdomain.Account, *walletdomain.Transaction, error) {
	s.called++
	s.input = input
	return &walletdomain.Account{}, &walletdomain.Transaction{}, nil
}

type walletSiteCurrencyStub struct {
	currency string
	err      error
	called   int
}

// GetSiteCurrency 返回测试指定的站点币种或读取错误。
func (s *walletSiteCurrencyStub) GetSiteCurrency(defaultValue string) (string, error) {
	s.called++
	if s.currency == "" {
		return defaultValue, s.err
	}
	return s.currency, s.err
}

func TestAdjustUserWalletUsesSiteCurrency(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name         string
		operation    string
		currency     string
		siteCurrency string
		settingsErr  error
		noSettings   bool
		wantStatus   int
		wantMessage  string
	}{
		{name: "默认使用人民币", operation: "add", wantStatus: http.StatusOK},
		{name: "允许相同币种", operation: "add", currency: "CNY", wantStatus: http.StatusOK},
		{name: "归一化相同币种", operation: "subtract", currency: " cny ", wantStatus: http.StatusOK},
		{name: "拒绝美元加款", operation: "add", currency: "USD", wantStatus: http.StatusBadRequest, wantMessage: "Payment currency mismatch"},
		{name: "拒绝日元扣款", operation: "subtract", currency: "JPY", wantStatus: http.StatusBadRequest, wantMessage: "Payment currency mismatch"},
		{name: "拒绝伪造币种", operation: "add", currency: "ZZZ", wantStatus: http.StatusBadRequest, wantMessage: "Payment currency mismatch"},
		{name: "使用配置的美元币种", operation: "add", siteCurrency: "USD", wantStatus: http.StatusOK},
		{name: "允许配置的美元币种", operation: "subtract", currency: "usd", siteCurrency: "USD", wantStatus: http.StatusOK},
		{name: "美元站点拒绝人民币", operation: "add", currency: "CNY", siteCurrency: "USD", wantStatus: http.StatusBadRequest, wantMessage: "Payment currency mismatch"},
		{name: "币种读取失败时拒绝调整", operation: "add", currency: "CNY", settingsErr: errors.New("settings unavailable"), wantStatus: http.StatusInternalServerError},
		{name: "无设置依赖时默认人民币", operation: "add", noSettings: true, wantStatus: http.StatusOK},
		{name: "无设置依赖时仍拒绝异币", operation: "subtract", currency: "USD", noSettings: true, wantStatus: http.StatusBadRequest, wantMessage: "Payment currency mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wallets := &walletAdjustmentStub{}
			settings := &walletSiteCurrencyStub{currency: tc.siteCurrency, err: tc.settingsErr}
			handler := &AdminHandler{wallets: wallets, settings: settings}
			if tc.noSettings {
				handler.settings = nil
			}
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Set("admin_id", uint(7))
			ctx.Params = gin.Params{{Key: "id", Value: "10"}}
			body, err := json.Marshal(AdminAdjustUserWalletRequest{Operation: tc.operation, Amount: "10.00", Currency: tc.currency, Remark: "reconciliation"})
			if err != nil {
				t.Fatal(err)
			}
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/10/wallet/adjust", strings.NewReader(string(body)))
			ctx.Request.Header.Set("Content-Type", "application/json")
			ctx.Request.Header.Set("Accept-Language", "en-US")
			handler.AdjustUserWallet(ctx)
			var result response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			wantCode := tc.wantStatus
			if tc.wantStatus == http.StatusOK {
				wantCode = 0
			}
			if recorder.Code != http.StatusOK || result.StatusCode != wantCode || !strings.Contains(result.Msg, tc.wantMessage) {
				t.Fatalf("status=%d body=%s, want business code=%d message=%q", recorder.Code, recorder.Body.String(), wantCode, tc.wantMessage)
			}
			if !tc.noSettings && settings.called != 1 {
				t.Fatalf("site currency reads=%d, want 1", settings.called)
			}
			if tc.wantStatus != http.StatusOK {
				if wallets.called != 0 {
					t.Fatalf("rejected request reached balance adjustment: %+v", wallets.input)
				}
				return
			}
			wantCurrency := tc.siteCurrency
			if wantCurrency == "" {
				wantCurrency = constants.SiteCurrencyDefault
			}
			wantDelta := decimal.NewFromInt(10)
			if tc.operation == "subtract" {
				wantDelta = wantDelta.Neg()
			}
			if wallets.called != 1 || wallets.input.Currency != wantCurrency || !wallets.input.Delta.Decimal.Equal(wantDelta) || wallets.input.UserID != 10 || wallets.input.OperatorAdminID != 7 {
				t.Fatalf("unexpected balance adjustment: calls=%d input=%+v", wallets.called, wallets.input)
			}
		})
	}
}

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
	err    error
}

func (s *rechargeConfirmStub) ConfirmRecharge(input ConfirmRechargeInput) error {
	s.called++
	s.input = input
	return s.err
}

func TestConfirmRechargeReportsCurrencyMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		locale      string
		wantMessage string
	}{
		{locale: "zh-CN", wantMessage: "支付币种不匹配"},
		{locale: "zh-TW", wantMessage: "支付幣種不匹配"},
		{locale: "en-US", wantMessage: "Payment currency mismatch"},
	} {
		for _, serviceErr := range []error{ErrPaymentCurrencyMismatch, fmt.Errorf("confirm recharge: %w", ErrPaymentCurrencyMismatch)} {
			t.Run(tc.locale+"/"+serviceErr.Error(), func(t *testing.T) {
				stub := &rechargeConfirmStub{err: serviceErr}
				handler := &AdminHandler{wallets: stub}
				recorder := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(recorder)
				ctx.Set("admin_id", uint(7))
				ctx.Params = gin.Params{{Key: "id", Value: "10"}}
				ctx.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/wallet/recharges/10/manual-confirm-payment", strings.NewReader(`{"payment_id":20}`))
				ctx.Request.Header.Set("Content-Type", "application/json")
				ctx.Request.Header.Set("Accept-Language", tc.locale)
				handler.ConfirmRecharge(ctx)
				var result response.Response
				if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if recorder.Code != http.StatusOK || result.StatusCode != response.CodeBadRequest || result.Msg != tc.wantMessage || stub.called != 1 {
					t.Fatalf("unexpected confirmation response: calls=%d status=%d body=%s", stub.called, recorder.Code, recorder.Body.String())
				}
			})
		}
	}
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
