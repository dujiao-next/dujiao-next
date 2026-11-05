package paypal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		BaseURL:      "https://api-m.sandbox.paypal.com",
		ReturnURL:    "https://example.com/payment?order_id=1",
		CancelURL:    "https://example.com/payment?order_id=1",
		WebhookID:    "WH-123456",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig should pass, got: %v", err)
	}
}

func TestValidateConfigAllowsMissingWebhookID(t *testing.T) {
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		BaseURL:      "https://api-m.sandbox.paypal.com",
		ReturnURL:    "https://example.com/payment?order_id=1",
		CancelURL:    "https://example.com/payment?order_id=1",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig should allow missing webhook_id, got: %v", err)
	}
}

func TestVerifyWebhookSignatureRequiresWebhookID(t *testing.T) {
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		BaseURL:      "https://api-m.sandbox.paypal.com",
		ReturnURL:    "https://example.com/payment?order_id=1",
		CancelURL:    "https://example.com/payment?order_id=1",
	}

	err := VerifyWebhookSignature(context.Background(), cfg, http.Header{}, []byte(`{}`))
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("VerifyWebhookSignature should require webhook_id, got: %v", err)
	}
}

func TestVerifyWebhookSignaturePreservesRawEvent(t *testing.T) {
	raw := []byte("{\n  \"id\": \"WH-1\",\n  \"event_type\": \"PAYMENT.CAPTURE.COMPLETED\",\n  \"summary\": \"A&B <x>\"\n}\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/v1/notifications/verify-webhook-signature":
			body, _ := io.ReadAll(r.Body)
			wantSuffix := append([]byte(`"webhook_event":`), raw...)
			wantSuffix = append(wantSuffix, '}')
			if !bytes.HasSuffix(body, wantSuffix) {
				t.Errorf("webhook_event changed:\nrequest: %s\n  event: %s", body, raw)
			}
			_, _ = w.Write([]byte(`{"verification_status":"SUCCESS"}`))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := &Config{ClientID: "client-id", ClientSecret: "client-secret", BaseURL: server.URL, WebhookID: "WH-ID"}
	headers := http.Header{}
	headers.Set("Paypal-Transmission-Id", "transmission-id")
	headers.Set("Paypal-Transmission-Time", "2026-07-29T12:00:00Z")
	headers.Set("Paypal-Cert-Url", "https://api.paypal.com/cert?foo=1&bar=2")
	headers.Set("Paypal-Auth-Algo", "SHA256withRSA")
	headers.Set("Paypal-Transmission-Sig", "signature")

	if err := VerifyWebhookSignature(context.Background(), cfg, headers, raw); err != nil {
		t.Fatalf("VerifyWebhookSignature failed: %v", err)
	}
}

func newCaptureTestConfig(baseURL string) *Config {
	return &Config{
		ClientID: "client-id", ClientSecret: "client-secret", BaseURL: baseURL,
		ReturnURL: "https://example.com/pay", CancelURL: "https://example.com/cancel",
	}
}

const capturedOrderBody = `{"id":"ORDER-1","status":"COMPLETED","purchase_units":[{"payments":{"captures":[{"id":"CAP-1","status":"COMPLETED","amount":{"currency_code":"USD","value":"1.00"},"create_time":"2026-10-16T07:24:21Z"}]}}]}`

// 订单已被捕获时再次 capture，PayPal 返回 422 ORDER_ALREADY_CAPTURED，
// 应回退到 GET 订单并返回已完成的捕获结果，而不是报错。
func TestCaptureOrderAlreadyCapturedFallsBackToGetOrder(t *testing.T) {
	getCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v1/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v2/checkout/orders/ORDER-1/capture":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_ALREADY_CAPTURED","description":"Order already captured."}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v2/checkout/orders/ORDER-1":
			getCalls++
			_, _ = w.Write([]byte(capturedOrderBody))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	result, err := CaptureOrder(context.Background(), newCaptureTestConfig(server.URL), "ORDER-1")
	if err != nil {
		t.Fatalf("CaptureOrder should recover from ORDER_ALREADY_CAPTURED, got: %v", err)
	}
	if getCalls != 1 {
		t.Fatalf("expected 1 GET order call, got %d", getCalls)
	}
	if result.Status != "COMPLETED" || result.CaptureID != "CAP-1" || result.Amount != "1.00" || result.Currency != "USD" || result.PaidAt == nil {
		t.Fatalf("unexpected capture result: %+v", result)
	}
}

// 其它 capture 失败（如买家尚未批准）仍然报错，且错误里要带上 PayPal 返回的 issue/description。
func TestCaptureOrderErrorIncludesPayPalDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/v2/checkout/orders/ORDER-2/capture":
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"name":"UNPROCESSABLE_ENTITY","details":[{"issue":"ORDER_NOT_APPROVED","description":"Payer has not yet approved the Order."}]}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	_, err := CaptureOrder(context.Background(), newCaptureTestConfig(server.URL), "ORDER-2")
	if !errors.Is(err, ErrResponseInvalid) {
		t.Fatalf("expected ErrResponseInvalid, got: %v", err)
	}
	for _, want := range []string{"422", "ORDER_NOT_APPROVED", "Payer has not yet approved"} {
		if !bytes.Contains([]byte(err.Error()), []byte(want)) {
			t.Fatalf("error %q should contain %q", err.Error(), want)
		}
	}
}

func TestParseConfigAndNormalize(t *testing.T) {
	raw := map[string]interface{}{
		"client_id":     " cid ",
		"client_secret": " secret ",
		"base_url":      "https://api-m.sandbox.paypal.com/",
		"return_url":    "https://example.com/return",
		"cancel_url":    "https://example.com/cancel",
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}
	if cfg.ClientID != "cid" {
		t.Fatalf("client id not normalized, got: %s", cfg.ClientID)
	}
	if cfg.BaseURL != "https://api-m.sandbox.paypal.com" {
		t.Fatalf("base url not normalized, got: %s", cfg.BaseURL)
	}
	if cfg.UserAction == "" {
		t.Fatalf("user action should have default value")
	}
}

func TestToPaymentStatus(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		resourceStatus string
		expectStatus   string
		expectOK       bool
	}{
		{name: "EventCompleted", eventType: paypalEventCaptureCompleted, resourceStatus: "", expectStatus: constants.PaymentStatusSuccess, expectOK: true},
		{name: "EventPending", eventType: paypalEventCapturePending, resourceStatus: "", expectStatus: constants.PaymentStatusPending, expectOK: true},
		{name: "ResourceDeclined", eventType: "", resourceStatus: paypalResourceStatusDeclined, expectStatus: constants.PaymentStatusFailed, expectOK: true},
		{name: "ResourceCreated", eventType: "", resourceStatus: paypalResourceStatusCreated, expectStatus: constants.PaymentStatusPending, expectOK: true},
		{name: "Unknown", eventType: "UNKNOWN", resourceStatus: "UNKNOWN", expectStatus: "", expectOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, ok := ToPaymentStatus(tc.eventType, tc.resourceStatus)
			if ok != tc.expectOK {
				t.Fatalf("unexpected ok: got %v, want %v", ok, tc.expectOK)
			}
			if status != tc.expectStatus {
				t.Fatalf("unexpected status: got %s, want %s", status, tc.expectStatus)
			}
		})
	}
}

func TestWebhookEventHelpers(t *testing.T) {
	event := &WebhookEvent{
		EventType: "PAYMENT.CAPTURE.COMPLETED",
		Resource: map[string]interface{}{
			"supplementary_data": map[string]interface{}{
				"related_ids": map[string]interface{}{
					"order_id": "ORDER-123",
				},
			},
			"amount": map[string]interface{}{
				"value":         "10.00",
				"currency_code": "USD",
			},
			"create_time": "2026-02-09T12:00:00Z",
			"status":      "COMPLETED",
		},
	}
	if got := event.RelatedOrderID(); got != "ORDER-123" {
		t.Fatalf("unexpected order id: %s", got)
	}
	value, currency := event.CaptureAmount()
	if value != "10.00" || currency != "USD" {
		t.Fatalf("unexpected amount info: %s %s", value, currency)
	}
	if event.PaidAt() == nil {
		t.Fatalf("PaidAt should parse time")
	}
	if status := event.ResourceStatus(); status != "COMPLETED" {
		t.Fatalf("unexpected resource status: %s", status)
	}
}

func TestWebhookEventHelpersCaptureAmountFallback(t *testing.T) {
	event := &WebhookEvent{
		EventType: "CHECKOUT.ORDER.COMPLETED",
		Resource: map[string]interface{}{
			"purchase_units": []interface{}{
				map[string]interface{}{
					"amount": map[string]interface{}{
						"value":         "88.66",
						"currency_code": "USD",
					},
				},
			},
		},
	}

	value, currency := event.CaptureAmount()
	if value != "88.66" || currency != "USD" {
		t.Fatalf("unexpected fallback amount info: %s %s", value, currency)
	}
}
