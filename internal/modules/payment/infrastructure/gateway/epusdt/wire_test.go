package epusdt

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCrossRepositoryMerchantWire(t *testing.T) {
	dir := os.Getenv("PAYMENT_WIRE_LAB_DIR")
	if dir == "" {
		t.Skip("wire lab not selected")
	}
	defer func() { _ = os.WriteFile(filepath.Join(dir, "done"), nil, 0600) }()
	var endpoint []byte
	deadline := time.Now().Add(30 * time.Second)
	for {
		var err error
		endpoint, err = os.ReadFile(filepath.Join(dir, "endpoint"))
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("gateway relay did not start")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.HasPrefix(string(endpoint), "http://127.0.0.1:") {
		t.Fatal("wire test only accepts loopback")
	}
	for _, network := range []string{"tron", "binance"} {
		cfg := quoteConfig(network)
		cfg.GatewayURL = string(endpoint)
		cfg.PID = "test-token"
		cfg.SecretKey = strings.Repeat("m", 32)
		cfg.NotifyURL = "https://93.184.216.34/notify"
		if network == "binance" {
			cfg.QuotePolicy.Recipient = "0x2222222222222222222222222222222222222222"
		}
		// A business order with only one minute remaining must not be extended.
		now := time.Now()
		expires := now.Add(time.Minute)
		saved, err := PrepareIntent(cfg, CreateInput{OrderNo: "wire-" + network, Amount: "123.45", Name: "synthetic wire order"}, 1, 1, "CNY", &expires, nil, now)
		if err != nil {
			t.Fatal(err)
		}
		first, err := CreateFixedPayment(context.Background(), cfg, saved)
		if err != nil {
			t.Fatal(network, "first create", err)
		}
		again, err := CreateFixedPayment(context.Background(), cfg, saved)
		if err != nil {
			t.Fatal(network, "unbound retry", err)
		}
		if first.TradeID != again.TradeID || first.ExpiresAt.UnixMilli() != expires.UnixMilli() || first.PaymentURL != "" {
			t.Fatal("wire retry or deadline changed")
		}
		bound, err := decodeIntent(first.PaymentIntent)
		if err != nil || bound.Terms == nil {
			t.Fatal("missing validated terms")
		}
		want := "17283000000000000000"
		if network == "tron" {
			want = "17283000"
		}
		if bound.Terms.RawAmount != want {
			t.Fatal("wire precision mismatch")
		}
	}
}
