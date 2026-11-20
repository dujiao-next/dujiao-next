package container

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestNewPaymentProviderRegistryRegistersEveryAdapter(t *testing.T) {
	registry := newPaymentProviderRegistry()
	tests := []struct {
		name         string
		providerType string
		channelType  string
		wantType     string
	}{
		{name: "stripe", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeStripe, wantType: "official:stripe"},
		{name: "paypal", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypePaypal, wantType: "official:paypal"},
		{name: "wechat", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeWechat, wantType: "official:wechat"},
		{name: "alipay", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeAlipay, wantType: "official:alipay"},
		{name: "nano", providerType: constants.PaymentProviderOfficial, channelType: constants.PaymentChannelTypeNano, wantType: "official:nano"},
		{name: "epay", providerType: constants.PaymentProviderEpay, channelType: "fallback-channel", wantType: "epay:"},
		{name: "epusdt", providerType: constants.PaymentProviderEpusdt, channelType: "fallback-channel", wantType: "epusdt:"},
		{name: "bepusdt", providerType: constants.PaymentProviderBepusdt, channelType: "fallback-channel", wantType: "bepusdt:"},
		{name: "dujiaopay", providerType: constants.PaymentProviderDujiaoPay, channelType: "fallback-channel", wantType: "dujiaopay:"},
		{name: "tokenpay", providerType: constants.PaymentProviderTokenpay, channelType: "fallback-channel", wantType: "tokenpay:"},
		{name: "okpay", providerType: constants.PaymentProviderOkpay, channelType: "fallback-channel", wantType: "okpay:"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			adapter, ok := registry.Lookup(test.providerType, test.channelType)
			if !ok {
				t.Fatalf("provider %s channel %s is not registered", test.providerType, test.channelType)
			}
			if got := adapter.Type(); got != test.wantType {
				t.Fatalf("adapter type want %q got %q", test.wantType, got)
			}
		})
	}

	if _, ok := registry.Lookup(constants.PaymentProviderOfficial, "unknown"); ok {
		t.Fatal("official provider must not fall back across channel types")
	}
}
