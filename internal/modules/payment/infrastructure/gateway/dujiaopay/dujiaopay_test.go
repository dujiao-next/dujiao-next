package dujiaopay

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignHeadersBuildsDujiaoPayCanonicalSignature(t *testing.T) {
	body := []byte(`{"chain":"tron","token_id":"tron-usdt","fiat_currency":"USD","fiat_amount":"20"}`)

	headers := SignHeaders("secret-1", "key-1", "POST", "/v1/orders", "b=2&a=1", body, 1750000000, "nonce-1")

	sum := sha256.Sum256(body)
	canonical := strings.Join([]string{
		"POST",
		"/v1/orders",
		"a=1&b=2",
		hex.EncodeToString(sum[:]),
		"1750000000",
		"nonce-1",
	}, "\n")
	mac := hmac.New(sha256.New, []byte("secret-1"))
	mac.Write([]byte(canonical))
	wantSig := hex.EncodeToString(mac.Sum(nil))

	if headers["DJP-Key-ID"] != "key-1" {
		t.Fatalf("DJP-Key-ID = %q, want key-1", headers["DJP-Key-ID"])
	}
	if headers["DJP-Timestamp"] != "1750000000" {
		t.Fatalf("DJP-Timestamp = %q, want 1750000000", headers["DJP-Timestamp"])
	}
	if headers["DJP-Nonce"] != "nonce-1" {
		t.Fatalf("DJP-Nonce = %q, want nonce-1", headers["DJP-Nonce"])
	}
	if headers["DJP-Signature"] != wantSig {
		t.Fatalf("DJP-Signature = %q, want %q", headers["DJP-Signature"], wantSig)
	}
}

func TestCreatePaymentPostsSignedDujiaoPayOrder(t *testing.T) {
	now := time.Unix(1750000000, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/orders" {
			t.Fatalf("path = %s, want /v1/orders", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if payload["merchant_order_id"] != "PAY-1001" {
			t.Fatalf("merchant_order_id = %v, want PAY-1001", payload["merchant_order_id"])
		}
		if payload["chain"] != "tron" || payload["token_id"] != "tron-usdt" {
			t.Fatalf("unexpected chain/token payload: %+v", payload)
		}
		if payload["fiat_currency"] != "USD" || payload["fiat_amount"] != "20.00" {
			t.Fatalf("unexpected fiat payload: %+v", payload)
		}
		if r.Header.Get("DJP-Key-ID") != "key-1" {
			t.Fatalf("DJP-Key-ID = %q", r.Header.Get("DJP-Key-ID"))
		}
		if r.Header.Get("DJP-Timestamp") != "1750000000" {
			t.Fatalf("DJP-Timestamp = %q", r.Header.Get("DJP-Timestamp"))
		}
		if r.Header.Get("DJP-Nonce") != "nonce-1" {
			t.Fatalf("DJP-Nonce = %q", r.Header.Get("DJP-Nonce"))
		}
		if r.Header.Get("Idempotency-Key") != "PAY-1001" {
			t.Fatalf("Idempotency-Key = %q", r.Header.Get("Idempotency-Key"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"order_id":"do_1001",
			"chain":"tron",
			"token_id":"tron-usdt",
			"pay_address":"TAddress",
			"payable_amount":"20.0001",
			"status":"pending",
			"expires_at":"2026-06-11T00:15:00Z",
			"checkout_token":"ct_once",
			"checkout_url":"https://pay.example.com/c/ct_once"
		}`))
	}))
	defer server.Close()

	result, err := CreatePayment(context.Background(), &Config{
		APIBaseURL:    server.URL,
		APIKeyID:      "key-1",
		APISecret:     "secret-1",
		WebhookSecret: "whsec-1",
		Chain:         "tron",
		TokenID:       "tron-usdt",
		FiatCurrency:  "USD",
	}, CreateInput{
		MerchantOrderID: "PAY-1001",
		FiatAmount:      "20.00",
		SuccessURL:      "https://shop.example.com/pay?status=success",
		CancelURL:       "https://shop.example.com/pay?status=cancel",
	}, WithNowFunc(func() time.Time { return now }), WithNonceFunc(func() string { return "nonce-1" }))
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if result.OrderID != "do_1001" {
		t.Fatalf("OrderID = %q, want do_1001", result.OrderID)
	}
	if result.CheckoutURL != "https://pay.example.com/c/ct_once" {
		t.Fatalf("CheckoutURL = %q", result.CheckoutURL)
	}
	if result.PayableAmount != "20.0001" || result.PayAddress != "TAddress" {
		t.Fatalf("unexpected payment details: %+v", result)
	}
}

func TestValidateConfigCashierMode(t *testing.T) {
	base := func() *Config {
		return &Config{
			APIBaseURL:    "https://api.example.com",
			APIKeyID:      "key-1",
			APISecret:     "secret-1",
			WebhookSecret: "whsec-1",
			FiatCurrency:  "USD",
		}
	}

	cashier := base()
	cashier.OrderMode = "cashier"
	if err := ValidateConfig(cashier); err != nil {
		t.Fatalf("cashier without token_id should pass: %v", err)
	}
	if cashier.TokenID != "" || cashier.Chain != "" {
		t.Fatalf("cashier should clear chain/token_id: %+v", cashier)
	}

	withMethods := base()
	withMethods.OrderMode = "cashier"
	withMethods.TokenID = "tron-usdt"
	withMethods.AllowedMethods = "Tron-USDT, base-usdc,tron-usdt"
	if err := ValidateConfig(withMethods); err != nil {
		t.Fatalf("cashier with allowed_methods should pass: %v", err)
	}
	methods := withMethods.AllowedMethodList()
	if len(methods) != 2 || methods[0] != "tron-usdt" || methods[1] != "base-usdc" {
		t.Fatalf("AllowedMethodList = %v, want [tron-usdt base-usdc]", methods)
	}

	// 未收录但符合 "<chain>-<token>" 约定的 token_id 允许配置，避免上游新增链时必须改代码。
	unlistedMethod := base()
	unlistedMethod.OrderMode = "cashier"
	unlistedMethod.AllowedMethods = "doge-usdt"
	if err := ValidateConfig(unlistedMethod); err != nil {
		t.Fatalf("unlisted but resolvable allowed method should pass: %v", err)
	}

	badMethod := base()
	badMethod.OrderMode = "cashier"
	badMethod.AllowedMethods = "dogeusdt"
	if err := ValidateConfig(badMethod); !errors.Is(err, ErrUnsupportedToken) {
		t.Fatalf("unresolvable allowed method should fail with ErrUnsupportedToken, got %v", err)
	}

	badMode := base()
	badMode.OrderMode = "invalid"
	if err := ValidateConfig(badMode); !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("invalid order_mode should fail, got %v", err)
	}

	transaction := base()
	if err := ValidateConfig(transaction); !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("transaction without token_id should fail, got %v", err)
	}
	if transaction.AllowedMethods != "" {
		t.Fatalf("transaction should clear allowed_methods")
	}
}

func TestResolveChainSupportsBSCUSDC(t *testing.T) {
	if got := ResolveChain("bsc-usdc"); got != "bsc" {
		t.Fatalf("ResolveChain(bsc-usdc) = %q, want bsc", got)
	}
}

// ResolveChain 对未收录的 token_id 按最后一个连字符切分，使 DujiaoPay 新增链或代币时
// 管理员直接填写即可；无法切分出 chain 的值必须返回空以便上层拒绝。
func TestResolveChainFallsBackToNamingConvention(t *testing.T) {
	cases := []struct {
		tokenID string
		want    string
	}{
		{"x-layer-usdt0", "x-layer"}, // 内置项，chain 自身含连字符
		{"doge-usdt", "doge"},        // 未收录但符合约定
		{"SUI-USDC", "sui"},          // 大小写与空白规范化
		{" sui-usdc ", "sui"},
		{"new-chain-usdt", "new-chain"},
		{"dogeusdt", ""}, // 缺少连字符
		{"-usdt", ""},    // chain 为空
		{"tron-", ""},    // token 为空
		{"", ""},
	}
	for _, tc := range cases {
		if got := ResolveChain(tc.tokenID); got != tc.want {
			t.Fatalf("ResolveChain(%q) = %q, want %q", tc.tokenID, got, tc.want)
		}
	}
}

func TestCreatePaymentCashierOmitsChainAndSendsAllowedMethods(t *testing.T) {
	now := time.Unix(1750000000, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := payload["chain"]; ok {
			t.Fatalf("cashier payload should omit chain: %+v", payload)
		}
		if _, ok := payload["token_id"]; ok {
			t.Fatalf("cashier payload should omit token_id: %+v", payload)
		}
		allowed, ok := payload["allowed_methods"].([]interface{})
		if !ok || len(allowed) != 2 {
			t.Fatalf("allowed_methods = %v, want 2 entries", payload["allowed_methods"])
		}
		first, _ := allowed[0].(map[string]interface{})
		if first["chain"] != "tron" || first["token_id"] != "tron-usdt" {
			t.Fatalf("allowed_methods[0] = %v", allowed[0])
		}
		second, _ := allowed[1].(map[string]interface{})
		if second["chain"] != "base" || second["token_id"] != "base-usdc" {
			t.Fatalf("allowed_methods[1] = %v", allowed[1])
		}
		w.Header().Set("Content-Type", "application/json")
		// 延迟分配订单：无 chain/token_id/pay_address/payable_amount，仅 selection_deadline。
		_, _ = w.Write([]byte(`{
			"order_id":"do_2001",
			"status":"awaiting_payment",
			"selection_deadline":"2026-06-11T00:15:00Z",
			"checkout_token":"ct_defer",
			"checkout_url":"https://pay.example.com/c/ct_defer"
		}`))
	}))
	defer server.Close()

	result, err := CreatePayment(context.Background(), &Config{
		APIBaseURL:     server.URL,
		APIKeyID:       "key-1",
		APISecret:      "secret-1",
		WebhookSecret:  "whsec-1",
		OrderMode:      "cashier",
		AllowedMethods: "tron-usdt,base-usdc",
		FiatCurrency:   "USD",
	}, CreateInput{
		MerchantOrderID: "PAY-2001",
		FiatAmount:      "20.00",
	}, WithNowFunc(func() time.Time { return now }), WithNonceFunc(func() string { return "nonce-1" }))
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if result.OrderID != "do_2001" {
		t.Fatalf("OrderID = %q, want do_2001", result.OrderID)
	}
	if result.CheckoutURL != "https://pay.example.com/c/ct_defer" {
		t.Fatalf("CheckoutURL = %q", result.CheckoutURL)
	}
	if result.Status != "awaiting_payment" {
		t.Fatalf("Status = %q, want awaiting_payment", result.Status)
	}
	if result.PayAddress != "" || result.PayableAmount != "" {
		t.Fatalf("deferred order should have no pay fields: %+v", result)
	}
}

func TestCreatePaymentCashierWithoutAllowedMethodsOmitsField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		if _, ok := payload["allowed_methods"]; ok {
			t.Fatalf("empty allowed_methods should be omitted: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order_id":"do_2002","status":"awaiting_payment","checkout_url":"https://pay.example.com/c/ct_2"}`))
	}))
	defer server.Close()

	if _, err := CreatePayment(context.Background(), &Config{
		APIBaseURL:    server.URL,
		APIKeyID:      "key-1",
		APISecret:     "secret-1",
		WebhookSecret: "whsec-1",
		OrderMode:     "cashier",
		FiatCurrency:  "USD",
	}, CreateInput{MerchantOrderID: "PAY-2002", FiatAmount: "5.00"}); err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
}

func TestParseWebhookVerifiesSignatureAndMapsPaidEvent(t *testing.T) {
	body := []byte(`{"event_id":"evt_1","event_type":"order.paid","event_version":"v1","created_at":"2026-06-06T12:00:00Z","data":{"order_id":"do_1001","merchant_order_id":"PAY-1001","fiat_currency":"usd","fiat_amount":"20.00","payable_amount":"20.145","tx_id":"0xabc"}}`)
	mac := hmac.New(sha256.New, []byte("whsec-1"))
	mac.Write([]byte("1750000000."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	event, err := ParseWebhook(&Config{WebhookSecret: "whsec-1"}, map[string]string{
		"DJP-Webhook-ID":        "evt_1",
		"DJP-Webhook-Timestamp": "1750000000",
		"DJP-Webhook-Signature": signature,
	}, body, time.Unix(1750000010, 0))
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if event.EventType != "order.paid" {
		t.Fatalf("EventType = %q", event.EventType)
	}
	if event.Status != "success" {
		t.Fatalf("Status = %q, want success", event.Status)
	}
	if event.OrderID != "do_1001" || event.MerchantOrderID != "PAY-1001" {
		t.Fatalf("unexpected ids: %+v", event)
	}
	if event.FiatCurrency != "USD" || event.FiatAmount != "20.00" {
		t.Fatalf("unexpected fiat facts: currency=%q amount=%q", event.FiatCurrency, event.FiatAmount)
	}
	if event.TransactionID != "0xabc" {
		t.Fatalf("TransactionID = %q, want 0xabc", event.TransactionID)
	}
}

func TestParseWebhookAcceptsLegacyTxHash(t *testing.T) {
	body := []byte(`{"event_id":"evt_legacy","event_type":"order.paid","event_version":"v1","created_at":"2026-06-06T12:00:00Z","data":{"order_id":"do_legacy","merchant_order_id":"PAY-LEGACY","fiat_currency":"CNY","fiat_amount":"88.00","tx_hash":"0xlegacy"}}`)
	mac := hmac.New(sha256.New, []byte("whsec-1"))
	mac.Write([]byte("1750000000."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	event, err := ParseWebhook(&Config{WebhookSecret: "whsec-1"}, map[string]string{
		"DJP-Webhook-Timestamp": "1750000000",
		"DJP-Webhook-Signature": signature,
	}, body, time.Unix(1750000010, 0))
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if event.TransactionID != "0xlegacy" {
		t.Fatalf("TransactionID = %q, want legacy tx_hash", event.TransactionID)
	}
}

func TestParseWebhookMethodSelectedEventMapsToEmptyStatus(t *testing.T) {
	body := []byte(`{"event_id":"evt_2","event_type":"order.method_selected","event_version":"v1","created_at":"2026-06-06T12:00:00Z","data":{"order_id":"do_2001","merchant_order_id":"PAY-2001","chain":"tron","token_id":"tron-usdt","payable_amount":"20.0001"}}`)
	mac := hmac.New(sha256.New, []byte("whsec-1"))
	mac.Write([]byte("1750000000."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	event, err := ParseWebhook(&Config{WebhookSecret: "whsec-1"}, map[string]string{
		"DJP-Webhook-ID":        "evt_2",
		"DJP-Webhook-Timestamp": "1750000000",
		"DJP-Webhook-Signature": signature,
	}, body, time.Unix(1750000010, 0))
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if event.EventType != "order.method_selected" {
		t.Fatalf("EventType = %q", event.EventType)
	}
	// 延迟分配订单选定链/币事件不改变支付状态，service 层按空 status 忽略。
	if event.Status != "" {
		t.Fatalf("Status = %q, want empty", event.Status)
	}
}

func TestParseWebhookRejectsProcessableEventWithoutBoundIdentifiers(t *testing.T) {
	body := []byte(`{"event_id":"evt_missing","event_type":"order.paid","event_version":"v1","created_at":"2026-06-06T12:00:00Z","data":{"merchant_order_id":"PAY-1001","tx_hash":"0xabc"}}`)
	mac := hmac.New(sha256.New, []byte("whsec-1"))
	mac.Write([]byte("1750000000."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	_, err := ParseWebhook(&Config{WebhookSecret: "whsec-1"}, map[string]string{
		"DJP-Webhook-Timestamp": "1750000000",
		"DJP-Webhook-Signature": signature,
	}, body, time.Unix(1750000010, 0))
	if !errors.Is(err, ErrResponseInvalid) {
		t.Fatalf("processable event without order binding must fail with ErrResponseInvalid, got %v", err)
	}
}
