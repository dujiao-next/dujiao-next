package epusdt

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dujiao-next/internal/constants"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

func quoteConfig(network string) *Config {
	recipient := "0x1111111111111111111111111111111111111111"
	if network == "tron" {
		recipient = "TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV"
	}
	cfg := &Config{GatewayURL: "https://gateway.example", PID: "1000", SecretKey: "test-secret", Token: "usdt", Network: network,
		Currency: "cny", NotifyURL: "https://shop.example/notify", ReturnURL: "https://shop.example/return", FixedPaymentIntent: true,
		QuotePolicy: &QuotePolicy{Version: "test-v1", Rate: "0.14", Precision: 4, MaxTail: "0.0099", LifetimeSeconds: 900, Recipient: recipient}}
	cfg.Normalize()
	return cfg
}

func fixedResponse(network string, created int64) map[string]interface{} {
	contract, recipient, raw, decimals := "0x55d398326f99059ff775485246999027b3197955", "0x1111111111111111111111111111111111111111", "14000100000000000000", 18
	if network == "tron" {
		contract, recipient, raw, decimals = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", "TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdV", "14000100", 6
	}
	return map[string]interface{}{"status_code": 200, "data": map[string]interface{}{
		"trade_id": "fixed-trade-1", "order_id": "DJP-1", "status": 1, "amount": 100, "actual_amount": "14.0001", "currency": "CNY", "token": "USDT", "receive_address": recipient,
		"expiration_time": (created + 600000) / 1000, "payment_url": "https://attacker.example/checkout",
		"payment_terms": map[string]interface{}{"version": 1, "trade_id": "fixed-trade-1", "order_id": "DJP-1", "api_key_id": 7,
			"network": network, "token": "USDT", "contract": contract, "decimals": decimals, "recipient": recipient, "raw_amount": raw,
			"fiat_amount": "100", "request_amount": "100", "currency": "CNY", "rate": "0.14", "created_at_ms": created, "expires_at_ms": created + 600000}}}
}

func TestFixedIntentAuthorizesBothNetworksAndPreservesRetries(t *testing.T) {
	for _, network := range []string{"tron", "binance"} {
		t.Run(network, func(t *testing.T) {
			created := time.Now()
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var request map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
				}
				if request["fixed_payment_intent"] != true || request["network"] != network || request["signature"] != Sign(request, "test-secret") {
					t.Error("request lost signed fixed-mode binding")
				}
				_ = json.NewEncoder(w).Encode(fixedResponse(network, created.UnixMilli()))
			}))
			defer srv.Close()
			cfg := quoteConfig(network)
			cfg.GatewayURL = srv.URL
			input := CreateInput{OrderNo: "DJP-1", Amount: "100", Name: "order"}
			authorized, err := PrepareIntent(cfg, input, 1, 2, "CNY", nil, nil, created)
			if err != nil {
				t.Fatal(err)
			}
			result, err := CreateFixedPayment(context.Background(), cfg, authorized)
			if err != nil {
				t.Fatal(err)
			}
			if result.TradeID != "fixed-trade-1" || result.PaymentURL != "" || result.Raw != nil {
				t.Fatal("untrusted cashier data escaped")
			}
			bound, err := decodeIntent(result.PaymentIntent)
			if err != nil || bound.Terms == nil || bound.Terms.RawAmount == "" {
				t.Fatal("validated terms not saved")
			}
			cfg.QuotePolicy.Rate = "0.2"
			cfg.QuotePolicy.Version = "test-v2"
			cfg.QuotePolicy.Precision = 2
			cfg.QuotePolicy.MaxTail = "0.01"
			cfg.NotifyURL = "https://shop.example/new-notify"
			// A retry before a response was durably bound still uses the old authorization.
			retry, err := PrepareIntent(cfg, input, 1, 2, "CNY", nil, authorized, time.Now())
			if err != nil || !reflect.DeepEqual(retry, authorized) {
				t.Fatalf("reprice on retry: %v", err)
			}
			// A completed bind is recovered without another request or changed expiry.
			retry, err = PrepareIntent(cfg, input, 1, 2, "CNY", nil, result.PaymentIntent, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			again, err := CreateFixedPayment(context.Background(), cfg, retry)
			if err != nil || !reflect.DeepEqual(again.PaymentIntent, result.PaymentIntent) || calls != 1 {
				t.Fatalf("bound retry changed: calls=%d err=%v", calls, err)
			}
			input.Amount = "101"
			if _, err = PrepareIntent(cfg, input, 1, 2, "CNY", nil, authorized, time.Now()); err == nil {
				t.Fatal("conflicting amount accepted")
			}
			cfg.FixedPaymentIntent = false
			cfg.QuotePolicy = nil
			if _, err = PrepareIntent(cfg, input, 1, 2, "CNY", nil, authorized, time.Now()); err == nil {
				t.Fatal("fixed intent downgraded")
			}
		})
	}
}

func TestFixedIntentRejectsUntrustedTerms(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(map[string]interface{}, map[string]interface{})
	}{
		{"missing snapshot", func(d, terms map[string]interface{}) { delete(d, "payment_terms") }},
		{"old protocol", func(d, terms map[string]interface{}) { terms["version"] = 2 }},
		{"other order", func(d, terms map[string]interface{}) { terms["order_id"] = "OTHER" }},
		{"other trade", func(d, terms map[string]interface{}) { terms["trade_id"] = "OTHER" }},
		{"path in trade", func(d, terms map[string]interface{}) { terms["trade_id"] = "../../other" }},
		{"missing identity", func(d, terms map[string]interface{}) { terms["api_key_id"] = 0 }},
		{"wrong chain", func(d, terms map[string]interface{}) { terms["network"] = "ethereum" }},
		{"wrong token", func(d, terms map[string]interface{}) { terms["token"] = "USDC" }},
		{"fake token", func(d, terms map[string]interface{}) {
			terms["contract"] = "0x2222222222222222222222222222222222222222"
		}},
		{"other wallet", func(d, terms map[string]interface{}) {
			terms["recipient"] = "0x2222222222222222222222222222222222222222"
		}},
		{"wrong decimals", func(d, terms map[string]interface{}) { terms["decimals"] = 6 }},
		{"low price", func(d, terms map[string]interface{}) { terms["raw_amount"] = "13999999999999999999" }},
		{"excess tail", func(d, terms map[string]interface{}) { terms["raw_amount"] = "14010000000000000000" }},
		{"fraction of slot", func(d, terms map[string]interface{}) { terms["raw_amount"] = "14000100000000000001" }},
		{"noncanonical raw", func(d, terms map[string]interface{}) { terms["raw_amount"] = "014000100000000000000" }},
		{"changed fiat", func(d, terms map[string]interface{}) { terms["fiat_amount"] = "1" }},
		{"changed requested amount", func(d, terms map[string]interface{}) { terms["request_amount"] = "1" }},
		{"currency relabel", func(d, terms map[string]interface{}) { terms["currency"] = "USD" }},
		{"gateway rate", func(d, terms map[string]interface{}) { terms["rate"] = "0.01" }},
		{"extended expiry", func(d, terms map[string]interface{}) { terms["expires_at_ms"] = time.Now().Add(time.Hour).UnixMilli() }},
		{"stale creation", func(d, terms map[string]interface{}) {
			terms["created_at_ms"] = time.Now().Add(-time.Minute).UnixMilli()
		}},
		{"future creation", func(d, terms map[string]interface{}) {
			terms["created_at_ms"] = time.Now().Add(time.Minute).UnixMilli()
		}},
		{"expired", func(d, terms map[string]interface{}) {
			terms["expires_at_ms"] = time.Now().Add(-time.Second).UnixMilli()
		}},
		{"seconds as milliseconds", func(d, terms map[string]interface{}) { terms["expires_at_ms"] = time.Now().Add(time.Hour).Unix() }},
		{"outer wrong amount", func(d, terms map[string]interface{}) { d["actual_amount"] = "14.000100000000000001" }},
		{"outer wallet", func(d, terms map[string]interface{}) {
			d["receive_address"] = "0x2222222222222222222222222222222222222222"
		}},
		{"success is not evidence", func(d, terms map[string]interface{}) { d["status"] = 2 }},
		{"outer expiry", func(d, terms map[string]interface{}) { d["expiration_time"] = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created := time.Now()
			response := fixedResponse("binance", created.UnixMilli())
			d := response["data"].(map[string]interface{})
			tc.edit(d, d["payment_terms"].(map[string]interface{}))
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _ = json.NewEncoder(w).Encode(response) }))
			defer srv.Close()
			cfg := quoteConfig("binance")
			cfg.GatewayURL = srv.URL
			authorized, err := PrepareIntent(cfg, CreateInput{OrderNo: "DJP-1", Amount: "100"}, 1, 2, "CNY", nil, nil, created)
			if err != nil {
				t.Fatal(err)
			}
			if result, err := CreateFixedPayment(context.Background(), cfg, authorized); !errors.Is(err, ErrResponseInvalid) || result != nil {
				t.Fatalf("hostile response accepted: %v", err)
			}
		})
	}
}

func TestFixedIntentRejectsInvalidPolicyAndLossyAmounts(t *testing.T) {
	for _, edit := range []func(*Config){
		func(c *Config) { c.Network = "ethereum" }, func(c *Config) { c.Token = "trx" }, func(c *Config) { c.OrderMode = "cashier" },
		func(c *Config) { c.QuotePolicy.Rate = "NaN" }, func(c *Config) { c.QuotePolicy.Rate = "1e1000000" },
		func(c *Config) { c.QuotePolicy.Rate = "0" }, func(c *Config) { c.QuotePolicy.MaxTail = "0.01" },
		func(c *Config) { c.QuotePolicy.MaxTail = "0.00001" }, func(c *Config) { c.QuotePolicy.Recipient = "0x0000000000000000000000000000000000000000" },
		func(c *Config) { c.PaymentType = "Epay" }, func(c *Config) { c.FixedPaymentIntent = false },
	} {
		cfg := quoteConfig("binance")
		edit(cfg)
		if ValidateConfig(cfg) == nil {
			t.Fatalf("invalid policy accepted: %+v", cfg.QuotePolicy)
		}
	}
	cfg := quoteConfig("tron")
	cfg.QuotePolicy.Recipient = "TBXSw8fM4jpQkGc6zZjsVABFpVN7UvXPdVA"
	if ValidateConfig(cfg) == nil {
		t.Fatal("bad TRON checksum accepted")
	}
	cfg = quoteConfig("binance")
	for _, amount := range []string{"0.01", "1.001", "9007199254740993.01", "NaN", "1e1000000"} {
		if _, err := PrepareIntent(cfg, CreateInput{OrderNo: "DJP-1", Amount: amount}, 1, 2, "CNY", nil, nil, time.Now()); err == nil {
			t.Fatalf("amount accepted: %s", amount)
		}
	}
	deadline := time.Now().Add(2 * time.Minute)
	authorized, err := PrepareIntent(cfg, CreateInput{OrderNo: "DJP-1", Amount: "100"}, 1, 2, "CNY", &deadline, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	i, _ := decodeIntent(authorized)
	if i.ExpiresAtMs != deadline.UnixMilli() {
		t.Fatal("business deadline extended")
	}
	if _, err = PrepareIntent(cfg, CreateInput{OrderNo: "DJP-1", Amount: "100"}, 1, 2, "USD", nil, nil, time.Now()); err == nil {
		t.Fatal("currency silently relabeled")
	}
	if _, err = CreatePayment(context.Background(), cfg, CreateInput{OrderNo: "DJP-1", Amount: "100"}); err == nil {
		t.Fatal("legacy API bypassed required persistence")
	}
}

func TestEpusdtTransportDoesNotForwardSignedRequestsOrReadUnboundedBodies(t *testing.T) {
	calls := 0
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer sink.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	if _, err := postJSON(context.Background(), srv.URL, map[string]interface{}{"signature": "test"}); err == nil || calls != 0 {
		t.Fatal("redirect forwarded signed request")
	}
	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 65537))) }))
	defer large.Close()
	if _, err := postJSON(context.Background(), large.URL, map[string]interface{}{}); err == nil {
		t.Fatal("oversized body accepted")
	}
}

func TestReadChainExpectationRevalidatesExpiredAuthorization(t *testing.T) {
	for _, network := range []string{"binance", "tron"} {
		t.Run(network, func(t *testing.T) {
			cfg := quoteConfig(network)
			created := time.Now().Add(-time.Hour)
			saved, err := PrepareIntent(cfg, CreateInput{OrderNo: "DJP-1", Amount: "100", Name: "test"}, 1, 2, "CNY", nil, nil, created)
			if err != nil {
				t.Fatal(err)
			}
			i, err := decodeIntent(saved)
			if err != nil {
				t.Fatal(err)
			}
			response := fixedResponse(network, created.UnixMilli())
			data := response["data"].(map[string]interface{})
			b, _ := json.Marshal(data["payment_terms"])
			if err := json.Unmarshal(b, &i.Terms); err != nil {
				t.Fatal(err)
			}
			bound, _ := intentMap(i)
			p := paymentdomain.Payment{ID: 1, OrderID: 2, GatewayOrderNo: "DJP-1", ProviderRef: "fixed-trade-1", ProviderType: constants.PaymentProviderEpusdt, RequiresChainVerification: true,
				Currency: "CNY", Amount: money.FromDecimal(decimal.NewFromInt(100)), PaymentIntent: bound}
			e, err := ReadChainExpectation(p)
			if err != nil || e.RawAmount != i.Terms.RawAmount || e.ExpiresAtMs >= time.Now().UnixMilli() {
				t.Fatalf("expired but valid authorization rejected: %v", err)
			}
			p.PaymentIntent["base_raw"] = "1"
			if _, err := ReadChainExpectation(p); err == nil {
				t.Fatal("corrupted authorization accepted")
			}
			p.PaymentIntent, _ = intentMap(i)
			p.ProviderRef = "other-trade"
			if _, err := ReadChainExpectation(p); err == nil {
				t.Fatal("wrong trade accepted")
			}
		})
	}
}
