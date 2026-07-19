package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/models"
)

func TestGenericWebhookCreateOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("unexpected authorization header %q", got)
		}
		if got := r.Header.Get("Idempotency-Key"); got != "ORDER-001" {
			t.Fatalf("unexpected idempotency key %q", got)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["event"] != "order.created" || payload["order_no"] != "ORDER-001" {
			t.Fatalf("unexpected payload: %#v", payload)
		}
		if payload["product_id"] != float64(10) || payload["sku_id"] != float64(11) {
			t.Fatalf("unexpected product identifiers: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"order_no":"REMOTE-001"}`))
	}))
	defer server.Close()

	adapter := NewGenericWebhookAdapter(&models.SiteConnection{BaseURL: server.URL, ApiKey: "test-token"})
	result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{
		DownstreamOrderNo: "ORDER-001",
		TraceID:           "trace-1",
		CallbackURL:       "https://shop.example.com/api/v1/upstream/generic-webhook/callback",
		LocalProductID:    10,
		LocalSKUID:        11,
		LocalSKUCode:      "DEFAULT",
		Quantity:          2,
		Amount:            "19.90",
		Currency:          "CNY",
		ManualFormData:    models.JSON{"account": "buyer"},
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if !result.OK || result.OrderNo != "REMOTE-001" || result.Status != "accepted" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGenericWebhookAcceptsEmptyAndMalformed2xxBodies(t *testing.T) {
	responses := []string{"", "not-json"}
	for _, responseBody := range responses {
		t.Run(responseBody, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte(responseBody))
			}))
			defer server.Close()
			adapter := NewGenericWebhookAdapter(&models.SiteConnection{BaseURL: server.URL, ApiKey: "token"})
			result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{DownstreamOrderNo: "ORDER-2"})
			if err != nil || result == nil || !result.OK {
				t.Fatalf("expected accepted response, result=%#v err=%v", result, err)
			}
		})
	}
}

func TestGenericWebhookErrorRetryability(t *testing.T) {
	for _, tc := range []struct {
		status    int
		retryable bool
	}{
		{status: http.StatusBadRequest, retryable: false},
		{status: http.StatusUnauthorized, retryable: false},
		{status: http.StatusRequestTimeout, retryable: true},
		{status: http.StatusTooManyRequests, retryable: true},
		{status: http.StatusBadGateway, retryable: true},
	} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "failed", tc.status)
			}))
			defer server.Close()
			adapter := NewGenericWebhookAdapter(&models.SiteConnection{BaseURL: server.URL, ApiKey: "token"})
			_, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{DownstreamOrderNo: "ORDER-3"})
			if err == nil {
				t.Fatal("expected error")
			}
			if got := IsRetryableRequestError(err); got != tc.retryable {
				t.Fatalf("retryable=%v want %v: %v", got, tc.retryable, err)
			}
		})
	}
}

func TestGenericWebhookExplicitRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":"invalid_request","error_message":"bad order"}`))
	}))
	defer server.Close()
	adapter := NewGenericWebhookAdapter(&models.SiteConnection{BaseURL: server.URL, ApiKey: "token"})
	result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{DownstreamOrderNo: "ORDER-4"})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if result.OK || result.ErrorCode != "invalid_request" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestGenericWebhookPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload["event"] != "connection.test" {
			t.Fatalf("unexpected ping payload: %#v", payload)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	adapter := NewGenericWebhookAdapter(&models.SiteConnection{BaseURL: server.URL, ApiKey: "token"})
	result, err := adapter.Ping(context.Background())
	if err != nil || result.ProtocolVersion != "1.0" {
		t.Fatalf("unexpected ping result=%#v err=%v", result, err)
	}
}

func TestGenericWebhookPingRejectsExplicitFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"message":"token rejected"}`))
	}))
	defer server.Close()
	adapter := NewGenericWebhookAdapter(&models.SiteConnection{BaseURL: server.URL, ApiKey: "token"})
	if _, err := adapter.Ping(context.Background()); err == nil || IsRetryableRequestError(err) {
		t.Fatalf("expected non-retryable ping rejection, got %v", err)
	}
}
