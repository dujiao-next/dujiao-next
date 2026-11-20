package presenter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

func newMoney(s string) money.Amount {
	return money.FromDecimal(decimal.RequireFromString(s))
}

func TestWalletRechargePaymentPayloadIncludesRefundedAmount(t *testing.T) {
	for _, amount := range []string{"0.00", "40.00", "100.00"} {
		t.Run(amount, func(t *testing.T) {
			recharge := &walletdomain.RechargeOrder{Amount: newMoney("100.00"), RefundedAmount: newMoney(amount)}
			payload := NewWalletRechargePaymentPayload(recharge, nil, nil)
			data, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), `"refunded_amount":"`+amount+`"`) {
				t.Fatalf("missing refunded amount: %s", data)
			}
		})
	}
}

func TestWalletAccountRespOmitsSensitiveFields(t *testing.T) {
	account := &walletdomain.Account{
		ID:      1,
		UserID:  99,
		Balance: newMoney("500.00"),
	}

	resp := NewWalletAccountResp(account)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"user_id"`) {
		t.Error("user_id should not appear")
	}
	if strings.Contains(jsonStr, `"id"`) {
		t.Error("id should not appear")
	}
	if !strings.Contains(jsonStr, `"balance"`) {
		t.Error("balance should appear")
	}
}

func TestWalletTransactionRespOmitsSensitiveFields(t *testing.T) {
	txn := &walletdomain.Transaction{
		ID:            1,
		UserID:        99,
		Type:          "recharge",
		Direction:     "in",
		Amount:        newMoney("100.00"),
		BalanceBefore: newMoney("400.00"),
		BalanceAfter:  newMoney("500.00"),
		Currency:      "CNY",
		Reference:     "unique-ref-001",
		Remark:        "充值",
		CreatedAt:     time.Now(),
	}

	resp := NewWalletTransactionResp(txn)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	sensitiveFields := []string{"user_id", "balance_before", "reference", "currency", "updated_at"}
	for _, field := range sensitiveFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("sensitive field %q should not appear", field)
		}
	}

	if !strings.Contains(jsonStr, `"balance_after"`) {
		t.Error("balance_after should appear")
	}
	if !strings.Contains(jsonStr, `"remark"`) {
		t.Error("remark should appear")
	}
}

func TestWalletRechargePaymentPayloadOmitsSensitiveFields(t *testing.T) {
	expires := time.Now()
	recharge := &walletdomain.RechargeOrder{
		ID:         1,
		RechargeNo: "RCG-001",
		UserID:     99,
		PaymentID:  5,
		ChannelID:  3,
		FeeRate:    newMoney("2.50"),
		Amount:     newMoney("100.00"),
		Status:     "success",
	}
	payment := &paymentdomain.Payment{
		ID:              5,
		ProviderType:    "alipay",
		ChannelType:     "alipay",
		InteractionMode: "redirect",
		FeeRate:         newMoney("1.00"),
		FixedFee:        newMoney("0.50"),
		ProviderRef:     "ALI-REF-001",
		GatewayOrderNo:  "GW-001",
		ProviderPayload: jsonmap.JSON{"raw": "data"},
		PayURL:          "https://pay.example.com",
		ExpiredAt:       &expires,
		Status:          "paid",
	}
	account := &walletdomain.Account{Balance: newMoney("200.00")}

	resp := NewWalletRechargePaymentPayload(recharge, payment, account)
	data, _ := json.Marshal(resp)
	jsonStr := string(data)

	// Recharge 敏感字段（fee_rate 故意保留：用户充值页面需要展示费率给用户看）
	sensitiveFields := []string{"user_id", "channel_id"}
	for _, field := range sensitiveFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("sensitive field %q should not appear", field)
		}
	}

	// Payment 敏感字段
	paymentSensitive := []string{"provider_ref", "gateway_order_no", "provider_payload", "fixed_fee"}
	for _, field := range paymentSensitive {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("payment sensitive field %q should not appear", field)
		}
	}

	if !strings.Contains(jsonStr, `"pay_url"`) {
		t.Error("pay_url should appear")
	}
	if !strings.Contains(jsonStr, `"recharge_no"`) {
		t.Error("recharge_no should appear")
	}
}
