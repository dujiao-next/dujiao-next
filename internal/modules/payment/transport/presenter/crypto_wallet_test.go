package presenter

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestFixedPaymentPresentationNeverExposesGatewayInstructions(t *testing.T) {
	p := &domain.Payment{ID: 7, RequiresChainVerification: true, ProviderType: constants.PaymentProviderEpusdt,
		InteractionMode: constants.PaymentInteractionQR, PayURL: "https://attacker.example/pay", QRCode: "https://attacker.example/qr",
		ProviderPayload: jsonmap.JSON{"data": map[string]any{"receive_address": "attacker", "actual_amount": "0.01", "chain": "attacker", "token_id": "attacker"}}}
	views := []any{NewCreatePaymentResp(&CreatePaymentResultView{Payment: p}), NewLatestPaymentResp(p, "synthetic")}
	for _, view := range views {
		b, err := json.Marshal(view)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(b, &fields); err != nil {
			t.Fatal(err)
		}
		if fields["fixed_payment"] != true || strings.Contains(string(b), "attacker") {
			t.Fatalf("unsafe response: %s", b)
		}
		for _, field := range []string{"pay_url", "qr_code", "wallet_address", "chain_amount", "chain", "token_id"} {
			if value, ok := fields[field]; ok && value != "" {
				t.Fatalf("legacy instructions exposed: %s", field)
			}
		}
	}
	// Normal payment presentation is unchanged.
	p.RequiresChainVerification = false
	if got := NewCreatePaymentResp(&CreatePaymentResultView{Payment: p}); got.PayURL != p.PayURL || got.WalletAddress != "attacker" {
		t.Fatal("legacy payment unexpectedly changed")
	}
}

func TestExtractUSDTWalletInfo(t *testing.T) {
	tests := []struct {
		name            string
		providerType    string
		interactionMode string
		payload         jsonmap.JSON
		wantAddress     string
		wantChainAmount string
	}{
		{
			name:            "bepusdt qr with token in data",
			providerType:    constants.PaymentProviderBepusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload: jsonmap.JSON{
				"data": map[string]any{
					"token":         "TRX_ADDR_XYZ",
					"actual_amount": "13.45",
				},
			},
			wantAddress:     "TRX_ADDR_XYZ",
			wantChainAmount: "13.45",
		},
		{
			name:            "bepusdt redirect mode returns empty",
			providerType:    constants.PaymentProviderBepusdt,
			interactionMode: constants.PaymentInteractionRedirect,
			payload: jsonmap.JSON{
				"data": map[string]any{"token": "TRX_ADDR_XYZ", "actual_amount": "13.45"},
			},
			wantAddress:     "",
			wantChainAmount: "",
		},
		{
			name:            "epusdt qr without receive_address yields empty",
			providerType:    constants.PaymentProviderEpusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload: jsonmap.JSON{
				"data": map[string]any{"trade_id": "T1", "actual_amount": "5.00"},
			},
			wantAddress:     "",
			wantChainAmount: "5.00",
		},
		{
			name:            "epusdt qr with receive_address",
			providerType:    constants.PaymentProviderEpusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload: jsonmap.JSON{
				"data": map[string]any{"receive_address": "TGRC_ADDR", "actual_amount": "9.99"},
			},
			wantAddress:     "TGRC_ADDR",
			wantChainAmount: "9.99",
		},
		{
			name:            "non-usdt provider returns empty",
			providerType:    constants.PaymentProviderOfficial,
			interactionMode: constants.PaymentInteractionQR,
			payload:         jsonmap.JSON{"data": map[string]any{"token": "X"}},
			wantAddress:     "",
			wantChainAmount: "",
		},
		{
			name:            "nil payload safe",
			providerType:    constants.PaymentProviderBepusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload:         nil,
			wantAddress:     "",
			wantChainAmount: "",
		},
		{
			name:            "bepusdt qr with data scalar instead of map",
			providerType:    constants.PaymentProviderBepusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload:         jsonmap.JSON{"data": "oops"},
			wantAddress:     "",
			wantChainAmount: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			info := ExtractCryptoWalletInfo(tc.providerType, tc.interactionMode, tc.payload)
			gotAddr, gotAmount := info.Address, info.ChainAmount
			if gotAddr != tc.wantAddress {
				t.Errorf("address: got %q want %q", gotAddr, tc.wantAddress)
			}
			if gotAmount != tc.wantChainAmount {
				t.Errorf("chain amount: got %q want %q", gotAmount, tc.wantChainAmount)
			}
		})
	}
}

func TestExtractCryptoWalletInfo_DujiaoPayQR(t *testing.T) {
	info := ExtractCryptoWalletInfo(constants.PaymentProviderDujiaoPay, constants.PaymentInteractionQR, jsonmap.JSON{
		"chain":          "tron",
		"token_id":       "tron-usdt",
		"pay_address":    "TAddr",
		"payable_amount": "10.0001",
	})

	if info.Address != "TAddr" {
		t.Fatalf("Address = %q", info.Address)
	}
	if info.ChainAmount != "10.0001" {
		t.Fatalf("ChainAmount = %q", info.ChainAmount)
	}
	if info.Chain != "tron" {
		t.Fatalf("Chain = %q", info.Chain)
	}
	if info.TokenID != "tron-usdt" {
		t.Fatalf("TokenID = %q", info.TokenID)
	}
}

func TestExtractCryptoWalletInfo_BepusdtQRIncludesChainLabels(t *testing.T) {
	info := ExtractCryptoWalletInfo(constants.PaymentProviderBepusdt, constants.PaymentInteractionQR, jsonmap.JSON{
		"data": map[string]any{
			"token":         "TAddr",
			"actual_amount": "4.25",
			"chain":         "tron",
			"token_id":      "tron-usdt",
		},
	})

	if info.Address != "TAddr" || info.ChainAmount != "4.25" || info.Chain != "tron" || info.TokenID != "tron-usdt" {
		t.Fatalf("unexpected info: %+v", info)
	}
}

func TestExtractCryptoWalletInfo_DujiaoPayWrappedPayload(t *testing.T) {
	info := ExtractCryptoWalletInfo(constants.PaymentProviderDujiaoPay, constants.PaymentInteractionQR, jsonmap.JSON{
		"data": map[string]any{
			"chain":          "base",
			"token_id":       "base-usdc",
			"pay_address":    "0xAddr",
			"payable_amount": "8.50",
		},
	})

	if info.Address != "0xAddr" || info.ChainAmount != "8.50" || info.Chain != "base" || info.TokenID != "base-usdc" {
		t.Fatalf("unexpected info: %+v", info)
	}
}
