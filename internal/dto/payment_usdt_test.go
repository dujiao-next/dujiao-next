package dto

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

func TestExtractUSDTWalletInfo(t *testing.T) {
	tests := []struct {
		name            string
		providerType    string
		interactionMode string
		payload         models.JSON
		wantAddress     string
		wantChainAmount string
	}{
		{
			name:            "bepusdt qr with token in data",
			providerType:    constants.PaymentProviderBepusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload: models.JSON{
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
			payload: models.JSON{
				"data": map[string]any{"token": "TRX_ADDR_XYZ", "actual_amount": "13.45"},
			},
			wantAddress:     "",
			wantChainAmount: "",
		},
		{
			name:            "epusdt qr without receive_address yields empty",
			providerType:    constants.PaymentProviderEpusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload: models.JSON{
				"data": map[string]any{"trade_id": "T1", "actual_amount": "5.00"},
			},
			wantAddress:     "",
			wantChainAmount: "5.00",
		},
		{
			name:            "epusdt qr with receive_address",
			providerType:    constants.PaymentProviderEpusdt,
			interactionMode: constants.PaymentInteractionQR,
			payload: models.JSON{
				"data": map[string]any{"receive_address": "TGRC_ADDR", "actual_amount": "9.99"},
			},
			wantAddress:     "TGRC_ADDR",
			wantChainAmount: "9.99",
		},
		{
			name:            "non-usdt provider returns empty",
			providerType:    constants.PaymentProviderOfficial,
			interactionMode: constants.PaymentInteractionQR,
			payload:         models.JSON{"data": map[string]any{"token": "X"}},
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
			payload:         models.JSON{"data": "oops"},
			wantAddress:     "",
			wantChainAmount: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotAddr, gotAmount := ExtractUSDTWalletInfo(tc.providerType, tc.interactionMode, tc.payload)
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
	info := ExtractCryptoWalletInfo(constants.PaymentProviderDujiaoPay, constants.PaymentInteractionQR, models.JSON{
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
	info := ExtractCryptoWalletInfo(constants.PaymentProviderBepusdt, constants.PaymentInteractionQR, models.JSON{
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
	info := ExtractCryptoWalletInfo(constants.PaymentProviderDujiaoPay, constants.PaymentInteractionQR, models.JSON{
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
