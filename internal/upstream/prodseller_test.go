package upstream

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestNewProdSellerAdapterUsesSecretReferenceOnly(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	conn := &siteconnectiondomain.Connection{
		BaseURL:   "https://prodseller.example.test",
		SecretRef: "TEST_PRODSELLER_KEY",
		ApiSecret: "must-not-be-read",
		Protocol:  "prodseller",
	}

	adapter, err := NewProdSellerAdapter(conn, t.TempDir())
	if err != nil {
		t.Fatalf("NewProdSellerAdapter() error = %v", err)
	}
	if adapter.baseURL != "https://prodseller.example.test/v1" {
		t.Fatalf("base URL = %q, want /v1 suffix", adapter.baseURL)
	}
	if adapter.SecretRef() != "TEST_PRODSELLER_KEY" {
		t.Fatalf("secret ref = %q", adapter.SecretRef())
	}
	if conn.ApiSecret != "must-not-be-read" {
		t.Fatal("constructor must not mutate or replace the connection secret")
	}
}

func TestProdSellerAdapterListProductsPreservesExternalIDs(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/products" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Errorf("missing API key header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"id":"6a2ff705dbe65bbecc0d3d9d","name":"Email","price":0.02,"publicPrice":"0.03","imageUrl":"https://cdn.example.test/a.png","delivery":{"type":"instant"},"stock":12,"inStock":true,"requiresEmailActivation":false,"skus":[{"id":"sku-alpha","sku":"DEFAULT","price":"0.02","stock":12,"inStock":true}]}]`))
	}))
	defer server.Close()

	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.ListProducts(context.Background(), ListProductsOpts{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListProducts() error = %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	product := result.Items[0]
	if product.ID != 0 || product.ExternalID != "6a2ff705dbe65bbecc0d3d9d" {
		t.Fatalf("external product ID was not preserved: id=%d external=%q", product.ID, product.ExternalID)
	}
	if product.PriceAmount != "0.02" || product.PublicPriceAmount != "0.03" {
		t.Fatalf("price mapping = %q/%q", product.PriceAmount, product.PublicPriceAmount)
	}
	if product.FulfillmentType != "auto" || !product.InStock || product.RequiresEmailActivation {
		t.Fatalf("product flags = %#v", product)
	}
	if len(product.SKUs) != 1 || product.SKUs[0].ID != 0 || product.SKUs[0].ExternalID != "sku-alpha" {
		t.Fatalf("external SKU ID was not preserved: %#v", product.SKUs)
	}
}

func TestProdSellerAdapterKeepsSoldOutProductsAndSKUsListed(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":"sold-out-id","name":"Sold out","price":"1.00","publicPrice":"1.20","delivery":{"type":"instant"},"stock":0,"inStock":false},
			{"id":"unlimited-id","name":"Unlimited","price":"2.00","publicPrice":"2.40","delivery":{"type":"instant"},"stock":null,"inStock":true}
		]`))
	}))
	defer server.Close()

	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.ListProducts(context.Background(), ListProductsOpts{})
	if err != nil {
		t.Fatalf("ListProducts() error = %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("want 2 products, got %d", len(result.Items))
	}

	soldOut := result.Items[0]
	if !soldOut.IsActive || soldOut.InStock || soldOut.Stock != 0 {
		t.Fatalf("sold-out product flags = %#v", soldOut)
	}
	if len(soldOut.SKUs) != 1 || !soldOut.SKUs[0].IsActive || soldOut.SKUs[0].StockQuantity != 0 || soldOut.SKUs[0].StockStatus != "out_of_stock" {
		t.Fatalf("sold-out default SKU must remain listed with zero stock: %#v", soldOut.SKUs)
	}

	unlimited := result.Items[1]
	if !unlimited.IsActive || !unlimited.InStock || unlimited.Stock != -1 {
		t.Fatalf("unlimited product flags = %#v", unlimited)
	}
	if len(unlimited.SKUs) != 1 || !unlimited.SKUs[0].IsActive || unlimited.SKUs[0].StockQuantity != -1 || unlimited.SKUs[0].StockStatus != "unlimited" {
		t.Fatalf("null stock with inStock=true should be unlimited and listed: %#v", unlimited.SKUs)
	}
}

func TestProdSellerAdapterGetBalanceAndPing(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/balance" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"telegramId":12345,"username":"seller","balance":"12.34","membership":{"level":"pro"}}`))
	}))
	defer server.Close()

	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	balance, err := adapter.GetBalance(context.Background())
	if err != nil {
		t.Fatalf("GetBalance() error = %v", err)
	}
	if balance.Balance != "12.34" || balance.TelegramID != "12345" || balance.Currency != "USDT" {
		t.Fatalf("balance = %#v", balance)
	}
	ping, err := adapter.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if ping.Balance != "12.34" || ping.SiteName != "seller" {
		t.Fatalf("ping = %#v", ping)
	}
}

func TestDecodeBalanceAcceptsStringMembershipAndNumericBalance(t *testing.T) {
	balance, err := decodeBalance(json.RawMessage(`{"telegramId":1943397168,"username":"seller","balance":0.54,"membership":"bronze"}`))
	if err != nil {
		t.Fatalf("decodeBalance() error = %v", err)
	}
	if balance.Balance != "0.54" || balance.TelegramID != "1943397168" {
		t.Fatalf("balance = %#v", balance)
	}
	if level, ok := balance.Membership["level"].(string); !ok || level != "bronze" {
		t.Fatalf("membership = %#v", balance.Membership)
	}
}

func TestProdSellerAdapterNormalizesHTTPErrors(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	cases := []struct {
		name string
		code int
		want error
	}{
		{name: "auth", code: http.StatusUnauthorized, want: ErrProdSellerAuthentication},
		{name: "balance", code: http.StatusPaymentRequired, want: ErrProdSellerInsufficientBalance},
		{name: "conflict", code: http.StatusConflict, want: ErrProdSellerConflict},
		{name: "retryable", code: http.StatusInternalServerError, want: ErrProdSellerRetryable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write([]byte(`{"code":"safe_error"}`))
			}))
			defer server.Close()
			adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			_, err = adapter.GetBalance(context.Background())
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, errors.Is(_, %v) is false", err, tc.want)
			}
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.Header().Set("X-RateLimit-Limit", "300")
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("X-RateLimit-Reset", "1234567890")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.GetBalance(context.Background())
	if !errors.Is(err, ErrProdSellerRateLimited) {
		t.Fatalf("429 error = %v", err)
	}
	var apiErr *ProdSellerAPIError
	if !errors.As(err, &apiErr) || apiErr.RetryAfter != 7*time.Second || apiErr.RateRemaining != "0" {
		t.Fatalf("rate limit metadata = %#v", apiErr)
	}
}

func TestProdSellerAdapterTimeoutIsRetryable(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte(`{"balance":"1"}`))
	}))
	defer server.Close()
	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adapter.client.Timeout = 1 * time.Millisecond
	_, err = adapter.GetBalance(context.Background())
	if !errors.Is(err, ErrProdSellerRetryable) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestProdSellerAdapterDoesNotCreateOrders(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		t.Error("a write request must not be sent in the read-only phase")
	}))
	defer server.Close()
	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{Quantity: 1})
	if !errors.Is(err, ErrProdSellerWriteDisabled) {
		t.Fatalf("CreateOrder() error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("unexpected HTTP requests: %d", requests.Load())
	}
}

func TestProdSellerAdapterRequiresEnvironmentVariable(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "")
	_, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if !errors.Is(err, ErrProdSellerAPIKeyMissing) {
		t.Fatalf("missing key error = %v", err)
	}
	if _, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{SecretRef: "not-a-variable"}, t.TempDir()); !errors.Is(err, ErrProdSellerAPIKeyRefInvalid) {
		t.Fatalf("invalid ref error = %v", err)
	}
}

func TestProdSellerAdapterErrorDoesNotExposeResponseBody(t *testing.T) {
	t.Setenv("TEST_PRODSELLER_KEY", "test-key")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"do-not-leak-this-response"}`))
	}))
	defer server.Close()
	adapter, err := NewProdSellerAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, SecretRef: "TEST_PRODSELLER_KEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	_, err = adapter.GetBalance(context.Background())
	if strings.Contains(err.Error(), "do-not-leak-this-response") {
		t.Fatalf("error leaked response body: %v", err)
	}
}
