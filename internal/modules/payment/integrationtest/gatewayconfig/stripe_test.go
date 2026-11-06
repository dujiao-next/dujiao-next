package gatewayconfig_test

import (
	"testing"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestValidateChannelStripeOfficial(t *testing.T) {
	svc := buildMinimalPaymentServiceWithRegistry(t)
	channel := &paymentdomain.PaymentChannel{
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeStripe,
		InteractionMode: constants.PaymentInteractionRedirect,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{
			"secret_key":             "sk_test_123456",
			"webhook_secret":         "whsec_123456",
			"success_url":            "https://example.com/payment?stripe_return=1",
			"cancel_url":             "https://example.com/payment?stripe_cancel=1",
			"api_base_url":           "https://api.stripe.com",
			"payment_method_types":   []string{"card"},
			"publishable_key":        "pk_test_123456",
			"webhook_tolerance_secs": 300,
		},
	}
	if err := svc.ValidateChannel(channel); err != nil {
		t.Fatalf("validate stripe channel failed: %v", err)
	}
}

func TestValidateChannelStripeOfficialDynamicPaymentMethods(t *testing.T) {
	svc := buildMinimalPaymentServiceWithRegistry(t)
	channel := &paymentdomain.PaymentChannel{
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeStripe,
		InteractionMode: constants.PaymentInteractionRedirect,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{
			"secret_key":      "sk_test_123456",
			"webhook_secret":  "whsec_123456",
			"success_url":     "https://example.com/payment?stripe_return=1",
			"cancel_url":      "https://example.com/payment?stripe_cancel=1",
			"api_base_url":    "https://api.stripe.com",
			"publishable_key": "pk_test_123456",
			// 不传 payment_method_types，对应新版 API 使用 Dashboard 动态支付方式的场景
		},
	}
	if err := svc.ValidateChannel(channel); err != nil {
		t.Fatalf("validate stripe channel with dynamic payment methods failed: %v", err)
	}
}

func TestValidateChannelStripeInvalidInteractionMode(t *testing.T) {
	svc := buildMinimalPaymentServiceWithRegistry(t)
	channel := &paymentdomain.PaymentChannel{
		ProviderType:    constants.PaymentProviderOfficial,
		ChannelType:     constants.PaymentChannelTypeStripe,
		InteractionMode: constants.PaymentInteractionQR,
		FeeRate:         money.FromDecimal(decimal.Zero),
		ConfigJSON: jsonmap.JSON{
			"secret_key":           "sk_test_123456",
			"webhook_secret":       "whsec_123456",
			"success_url":          "https://example.com/payment?stripe_return=1",
			"cancel_url":           "https://example.com/payment?stripe_cancel=1",
			"api_base_url":         "https://api.stripe.com",
			"payment_method_types": []string{"card"},
		},
	}
	if err := svc.ValidateChannel(channel); err == nil {
		t.Fatalf("expected invalid interaction mode error")
	}
}
