package service

import (
	"testing"

	"github.com/dujiao-next/internal/models"
)

func TestPaymentMethodAllowed(t *testing.T) {
	payload := models.JSON{
		"data": map[string]any{
			"payment_methods": []map[string]any{
				{"currency": "USDT", "network": "tron"},
				{"currency": "USDC", "network": "base"},
			},
		},
	}

	if !paymentMethodAllowed(payload, "USDC", "base") {
		t.Fatal("expected listed payment method to be allowed")
	}
	if paymentMethodAllowed(payload, "USDT", "base") {
		t.Fatal("expected unlisted currency/network pair to be rejected")
	}
	if paymentMethodAllowed(models.JSON{}, "USDT", "tron") {
		t.Fatal("expected missing payment method list to be rejected")
	}
}

func TestHasProviderResultAcceptsBepusdtMethodSelection(t *testing.T) {
	payment := &models.Payment{
		ProviderType:    "bepusdt",
		InteractionMode: "qr",
		ProviderRef:     "BEP-1",
	}
	if !hasProviderResult(payment) {
		t.Fatal("expected BEpusdt selection state to be reusable")
	}
	payment.ProviderType = "official"
	if hasProviderResult(payment) {
		t.Fatal("expected a generic provider reference without destination to be unusable")
	}
}
