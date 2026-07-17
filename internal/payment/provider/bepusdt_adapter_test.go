package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/payment/bepusdt"
	"github.com/shopspring/decimal"
)

func TestBepusdtAdapter_Type(t *testing.T) {
	a := NewBepusdtAdapter()
	want := constants.PaymentProviderBepusdt + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestBepusdtAdapter_ValidateConfig_UnsupportedChannel(t *testing.T) {
	a := NewBepusdtAdapter()
	err := a.ValidateConfig(models.JSON{}, "no-such-channel-type")
	if err == nil {
		t.Fatalf("expected error for unsupported channel")
	}
	if !errors.Is(err, ErrUnsupportedChannel) {
		t.Fatalf("expected wrapped ErrUnsupportedChannel, got %v", err)
	}
}

func TestBepusdtAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewBepusdtAdapter()
	// 用 bepusdt 真实支持的 channelType（usdt-trc20 / usdc-trc20 / trx）
	_, err := a.CreatePayment(context.Background(), models.JSON{}, CreateInput{
		OrderNo:     "ORDER_1",
		Currency:    "USDT",
		ChannelType: "usdt-trc20",
	})
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("expected wrapped ErrConfigInvalid, got %v", err)
	}
}

func TestBepusdtAdapter_CreatePayment_QRModeUsesWalletAddress(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()

	result, err := a.CreatePayment(context.Background(), validBepusdtConfig(server.URL), CreateInput{
		OrderNo:     "ORDER-QR-1",
		Subject:     "测试商品",
		Amount:      models.NewMoneyFromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentChannelTypeUsdtTrc20,
		Extra:       models.JSON{"interaction_mode": constants.PaymentInteractionQR},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	if result.RedirectURL != "" {
		t.Fatalf("RedirectURL = %q, want empty in qr mode", result.RedirectURL)
	}
	if result.QRCodeURL != "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t" {
		t.Fatalf("QRCodeURL = %q, want wallet address", result.QRCodeURL)
	}
	data := result.Payload["data"].(map[string]interface{})
	if data["actual_amount"] != "4.25" {
		t.Fatalf("actual_amount = %v, want 4.25", data["actual_amount"])
	}
	if data["trade_type"] != "usdt.trc20" {
		t.Fatalf("trade_type = %v, want usdt.trc20", data["trade_type"])
	}
	if data["chain"] != "tron" || data["token_id"] != "tron-usdt" {
		t.Fatalf("unexpected chain labels: chain=%v token_id=%v", data["chain"], data["token_id"])
	}
}

func TestBepusdtAdapter_CreatePayment_RedirectModeKeepsCashierURL(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()

	result, err := a.CreatePayment(context.Background(), validBepusdtConfig(server.URL), CreateInput{
		OrderNo:     "ORDER-REDIRECT-1",
		Subject:     "测试商品",
		Amount:      models.NewMoneyFromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentChannelTypeUsdtTrc20,
		Extra:       models.JSON{"interaction_mode": constants.PaymentInteractionRedirect},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	wantURL := "https://bepusdt.example/pay/checkout-counter/BEP-1"
	if result.RedirectURL != wantURL {
		t.Fatalf("RedirectURL = %q, want %q", result.RedirectURL, wantURL)
	}
	if result.QRCodeURL != wantURL {
		t.Fatalf("QRCodeURL = %q, want %q", result.QRCodeURL, wantURL)
	}
}

func TestBepusdtAdapter_MapBepusdtError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", bepusdt.ErrConfigInvalid, ErrConfigInvalid},
		{"trade_type→unsupported", bepusdt.ErrTradeTypeNotSupport, ErrUnsupportedChannel},
		{"request", bepusdt.ErrRequestFailed, ErrRequestFailed},
		{"response", bepusdt.ErrResponseInvalid, ErrResponseInvalid},
		{"signature", bepusdt.ErrSignatureInvalid, ErrSignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapBepusdtError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapBepusdtError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}

func validBepusdtConfig(gatewayURL string) models.JSON {
	return models.JSON{
		"gateway_url": gatewayURL,
		"auth_token":  "token-001",
		"trade_type":  "usdt.trc20",
		"fiat":        "CNY",
		"notify_url":  "https://api.example.com/api/v1/payments/callback",
		"return_url":  "https://example.com/pay",
	}
}

func newBepusdtCreatePaymentServer(t *testing.T, wantTradeType string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/order/create-transaction" {
			t.Fatalf("path = %s, want /api/v1/order/create-transaction", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		if payload["trade_type"] != wantTradeType {
			t.Fatalf("trade_type = %v, want %s", payload["trade_type"], wantTradeType)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 200,
			"message": "success",
			"data": {
				"fiat": "CNY",
				"trade_id": "BEP-1",
				"order_id": "ORDER-1",
				"amount": "28.88",
				"actual_amount": "4.25",
				"token": "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
				"expiration_time": 1200,
				"status": 1,
				"payment_url": "https://bepusdt.example/pay/checkout-counter/BEP-1"
			}
		}`))
	}))
}
