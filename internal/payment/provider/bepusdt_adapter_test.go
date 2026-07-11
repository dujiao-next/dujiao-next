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

func TestBepusdtAdapter_CreatePayment_QRModeReturnsPaymentMethods(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()
	config := validBepusdtConfig(server.URL)
	config["currencies"] = "USDT,USDC"

	result, err := a.CreatePayment(context.Background(), config, CreateInput{
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
	if result.QRCodeURL != "" {
		t.Fatalf("QRCodeURL = %q, want empty before selecting a payment method", result.QRCodeURL)
	}
	data := result.Payload["data"].(map[string]interface{})
	methods, ok := data["payment_methods"].([]bepusdt.PaymentMethod)
	if !ok || len(methods) != 2 {
		t.Fatalf("payment_methods = %#v, want two methods", data["payment_methods"])
	}
	if data["selection_required"] != true {
		t.Fatalf("selection_required = %v, want true", data["selection_required"])
	}
	if methods[0].Currency != "USDT" || methods[0].Network != "tron" {
		t.Fatalf("unexpected first payment method: %#v", methods[0])
	}
}

func TestBepusdtAdapter_SelectPaymentMethodReturnsWalletAddress(t *testing.T) {
	adapter := NewBepusdtAdapter()
	selector, ok := adapter.(PaymentMethodSelector)
	if !ok {
		t.Fatal("bepusdt adapter does not implement PaymentMethodSelector")
	}
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()

	result, err := selector.SelectPaymentMethod(context.Background(), validBepusdtConfig(server.URL), SelectPaymentMethodInput{
		ProviderRef: "BEP-SELECT-1",
		Currency:    "USDC",
		Network:     "base",
	})
	if err != nil {
		t.Fatalf("SelectPaymentMethod() failed: %v", err)
	}
	if result.QRCodeURL != "0xBaseWalletAddress" {
		t.Fatalf("QRCodeURL = %q, want selected wallet address", result.QRCodeURL)
	}
	data := result.Payload["data"].(map[string]interface{})
	if data["selection_required"] != false || data["selected_currency"] != "USDC" || data["selected_network"] != "base" {
		t.Fatalf("unexpected selection payload: %#v", data)
	}
	if data["chain"] != "base" || data["token_id"] != "base-usdc" {
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
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/order/create-order":
			if payload["reselect"] != true {
				t.Fatalf("reselect = %v, want true", payload["reselect"])
			}
			if payload["currencies"] != "USDT,USDC" {
				t.Fatalf("currencies = %v, want USDT,USDC", payload["currencies"])
			}
			_, _ = w.Write([]byte(`{
				"status_code": 200,
				"message": "success",
				"data": {
					"fiat": "CNY",
					"trade_id": "BEP-SELECT-1",
					"order_id": "ORDER-QR-1",
					"amount": "28.88",
					"expiration_time": 1200,
					"payment_url": "https://bepusdt.example/pay/cashier/BEP-SELECT-1",
					"network": [
						{"amount":"28.88","actual_amount":"4.25","fiat":"CNY","exchange_rate":"6.79","currency":"USDT","network":"tron","token_net_name":"TRC20","is_popular":true},
						{"amount":"28.88","actual_amount":"4.01","fiat":"CNY","exchange_rate":"7.20","currency":"USDC","network":"base","token_net_name":"Base","is_popular":false}
					]
				}
			}`))
		case "/api/v1/pay/update-order":
			if payload["trade_id"] != "BEP-SELECT-1" || payload["currency"] != "USDC" || payload["network"] != "base" {
				t.Fatalf("unexpected update-order payload: %#v", payload)
			}
			_, _ = w.Write([]byte(`{
				"status_code": 200,
				"message": "success",
				"data": {
					"fiat": "CNY",
					"trade_type": "usdc.base",
					"trade_id": "BEP-SELECT-1",
					"order_id": "ORDER-QR-1",
					"amount": "28.88",
					"actual_amount": "4.01",
					"token": "0xBaseWalletAddress",
					"expiration_time": 1200,
					"payment_url": "https://bepusdt.example/pay/checkout-counter/BEP-SELECT-1"
				}
			}`))
		case "/api/v1/order/create-transaction":
			if payload["trade_type"] != wantTradeType {
				t.Fatalf("trade_type = %v, want %s", payload["trade_type"], wantTradeType)
			}
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
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
}
