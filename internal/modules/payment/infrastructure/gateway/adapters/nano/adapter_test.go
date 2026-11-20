package nanoadapter

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/nano"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

const (
	shopAccount = "nano_3t6k35gi95xu6tergt6p69ck76ogmitsa8mnijtpxm9fkcm736xtoncuohr3"
	payer       = "nano_1111111111111111111111111111111111111111111111111111hifc8npp"
	blockHash   = "991CF190094C00F0B68E2E5F75F6BEE95A2E0BD93CEAA4A6734DB9F19B728948"
)

func testConfig(t *testing.T, nodeURL string) jsonmap.JSON {
	t.Helper()
	return jsonmap.JSON{
		"account":       shopAccount,
		"node_url":      nodeURL,
		"claim_dir":     t.TempDir(),
		"exchange_rate": "0.25",
	}
}

func createInput(paymentID uint, amount string) paymentcontract.GatewayCreateInput {
	return paymentcontract.GatewayCreateInput{
		PaymentID: paymentID,
		OrderNo:   "DJ20261120001",
		Amount:    money.Amount{Decimal: decimal.RequireFromString(amount)},
		Currency:  "CNY",
	}
}

// fakeNode 回答 receivable / block_info；block 为 nil 时表示还没有付款。
type fakeNode struct {
	block     map[string]interface{}
	confirmed string
}

func (f *fakeNode) server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		for k := range req {
			if strings.Contains(k, "key") || strings.Contains(k, "seed") || k == "wallet" {
				t.Errorf("node request carries %q", k)
			}
		}
		switch req["action"] {
		case "receivable":
			blocks := map[string]interface{}{}
			if f.block != nil {
				blocks[blockHash] = map[string]string{"amount": f.block["amount"].(string), "source": payer}
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"blocks": blocks})
		case "account_history":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"history": []interface{}{}})
		case "block_info":
			if f.block == nil {
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Block not found"})
				return
			}
			b := map[string]interface{}{}
			for k, v := range f.block {
				b[k] = v
			}
			b["confirmed"] = f.confirmed
			_ = json.NewEncoder(w).Encode(b)
		default:
			t.Errorf("unexpected node action %q", req["action"])
		}
	}))
}

func sendOf(to, amount string) map[string]interface{} {
	return map[string]interface{}{
		"block_account": payer,
		"amount":        amount,
		"subtype":       "send",
		"contents":      map[string]string{"type": "state", "link_as_account": to},
	}
}

func TestNanoAdapter_TypeAndRegistryKey(t *testing.T) {
	if got := NewNanoAdapter().Type(); got != "official:nano" {
		t.Fatalf("Type() = %q", got)
	}
}

func TestNanoAdapter_ValidateConfig(t *testing.T) {
	a := NewNanoAdapter()
	good := testConfig(t, "https://node.example")
	if err := a.ValidateConfig(good, constants.PaymentInteractionQR); err != nil {
		t.Fatalf("good config refused: %v", err)
	}
	bad := map[string]func(jsonmap.JSON){
		"empty account":   func(c jsonmap.JSON) { c["account"] = "" },
		"bad checksum":    func(c jsonmap.JSON) { c["account"] = shopAccount[:len(shopAccount)-1] + "4" },
		"no node":         func(c jsonmap.JSON) { delete(c, "node_url") },
		"node not http":   func(c jsonmap.JSON) { c["node_url"] = "file:///etc/passwd" },
		"no claim dir":    func(c jsonmap.JSON) { delete(c, "claim_dir") },
		"relative claims": func(c jsonmap.JSON) { c["claim_dir"] = "claims" },
		"zero rate":       func(c jsonmap.JSON) { c["exchange_rate"] = "0" },
	}
	for name, mutate := range bad {
		c := jsonmap.JSON{}
		for k, v := range good {
			c[k] = v
		}
		mutate(c)
		if err := a.ValidateConfig(c, constants.PaymentInteractionQR); !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
			t.Errorf("%s: err = %v, want ErrGatewayConfigInvalid", name, err)
		}
	}
	if err := a.ValidateConfig(good, constants.PaymentInteractionWAP); !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Errorf("wap mode accepted: %v", err)
	}
	if err := a.ValidateConfig(nil, ""); !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Errorf("unset channel accepted: %v", err)
	}
}

func TestNanoAdapter_CreatePaymentBindsAmountToPayment(t *testing.T) {
	a := NewNanoAdapter()
	cfg := testConfig(t, "https://node.example")
	res, err := a.CreatePayment(context.Background(), cfg, createInput(7, "10.00"))
	if err != nil {
		t.Fatal(err)
	}
	// 10 CNY × 0.25 = 2.50 XNO
	if res.AmountSent != "2.50" || res.CurrencySent != "XNO" {
		t.Fatalf("sent %s %s", res.AmountSent, res.CurrencySent)
	}
	raw, ok := new(big.Int).SetString(res.ProviderRef, 10)
	if !ok {
		t.Fatalf("provider ref %q is not a raw amount", res.ProviderRef)
	}
	// 1 XNO = 10^30 raw：2.50 XNO 是 250 × 10^28 raw，加上一个小于 0.01 XNO 的标记。
	unit := new(big.Int).Exp(big.NewInt(10), big.NewInt(28), nil)
	tag := new(big.Int).Sub(raw, new(big.Int).Mul(big.NewInt(250), unit))
	if tag.Sign() <= 0 || tag.Cmp(unit) >= 0 {
		t.Fatalf("raw %s is not 2.50 XNO plus a tag", raw)
	}
	if want := "nano:" + shopAccount + "?amount=" + res.ProviderRef; res.QRCodeURL != want || res.RedirectURL != want {
		t.Fatalf("uri %q", res.QRCodeURL)
	}
	if res.Payload["original_amount"] != "10" || res.Payload["exchange_rate"] != "0.25" {
		t.Fatalf("payload %v", res.Payload)
	}
	other, err := a.CreatePayment(context.Background(), cfg, createInput(8, "10.00"))
	if err != nil || other.ProviderRef == res.ProviderRef {
		t.Fatalf("two payments share one amount: %v", err)
	}
	again, _ := a.CreatePayment(context.Background(), cfg, createInput(7, "10.00"))
	if again.ProviderRef != res.ProviderRef {
		t.Fatal("same payment, different amount")
	}
}

func TestNanoAdapter_CreatePaymentRefuses(t *testing.T) {
	a := NewNanoAdapter()
	cfg := testConfig(t, "https://node.example")
	delete(cfg, "exchange_rate")
	if _, err := a.CreatePayment(context.Background(), cfg, createInput(1, "10.00")); !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("CNY order without a rate: %v", err)
	}
	in := createInput(1, "1.25")
	in.Currency = "XNO"
	res, err := a.CreatePayment(context.Background(), cfg, in)
	if err != nil || res.AmountSent != "1.25" || res.Payload["exchange_rate"] != nil {
		t.Fatalf("XNO order: %+v %v", res, err)
	}
	in.PaymentID = 0
	if _, err := a.CreatePayment(context.Background(), cfg, in); !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("payment without id: %v", err)
	}
}

func TestNanoAdapter_QueryPayment(t *testing.T) {
	a := NewNanoAdapter().(paymentcontract.GatewayCapturer)
	node := &fakeNode{confirmed: "true"}
	srv := node.server(t)
	defer srv.Close()
	cfg := testConfig(t, srv.URL)
	created, err := a.CreatePayment(context.Background(), cfg, createInput(7, "10.00"))
	if err != nil {
		t.Fatal(err)
	}
	ref := created.ProviderRef

	// 还没付款
	q, err := a.QueryPayment(context.Background(), cfg, ref)
	if err != nil || q.Status != constants.PaymentStatusPending {
		t.Fatalf("before paying: %+v %v", q, err)
	}

	// 区块已广播但未确认
	node.block = sendOf(shopAccount, ref)
	node.confirmed = "false"
	q, err = a.QueryPayment(context.Background(), cfg, ref)
	if err != nil || q.Status != constants.PaymentStatusPending {
		t.Fatalf("unconfirmed: %+v %v", q, err)
	}

	// 确认后：成功，金额与 CreatePayment 的 AmountSent 一致
	node.confirmed = "true"
	q, err = a.QueryPayment(context.Background(), cfg, ref)
	if err != nil || q.Status != constants.PaymentStatusSuccess || q.Currency != "XNO" || q.Amount.String() != "2.50" {
		t.Fatalf("paid: %+v %v", q, err)
	}
	if q.Payload["nano_block_hash"] != blockHash || q.Payload["nano_payer"] != payer {
		t.Fatalf("payload %v", q.Payload)
	}
	// 同一笔支付再次查询（重复轮询）仍是成功，由业务层幂等处理
	if q, err = a.QueryPayment(context.Background(), cfg, ref); err != nil || q.Status != constants.PaymentStatusSuccess {
		t.Fatalf("second query: %+v %v", q, err)
	}
}

func TestNanoAdapter_QueryPaymentRefusesWrongSend(t *testing.T) {
	a := NewNanoAdapter().(paymentcontract.GatewayCapturer)
	node := &fakeNode{confirmed: "true"}
	srv := node.server(t)
	defer srv.Close()
	cfg := testConfig(t, srv.URL)
	created, _ := a.CreatePayment(context.Background(), cfg, createInput(7, "10.00"))
	// 付给了别的地址
	node.block = sendOf(payer, created.ProviderRef)
	if _, err := a.QueryPayment(context.Background(), cfg, created.ProviderRef); !errors.Is(err, paymentcontract.ErrGatewayResponseInvalid) {
		t.Fatalf("send to another account: %v", err)
	}
	if _, err := a.QueryPayment(context.Background(), cfg, "not-a-number"); !errors.Is(err, paymentcontract.ErrGatewayResponseInvalid) {
		t.Fatalf("bad provider ref: %v", err)
	}
}

// 同一个区块哈希已经给另一笔支付入账（claim_dir 里有记录，例如重启前写下的），
// 这笔支付不能再用它入账。
func TestNanoAdapter_QueryPaymentRefusesReplayAcrossRestart(t *testing.T) {
	node := &fakeNode{confirmed: "true"}
	srv := node.server(t)
	defer srv.Close()
	cfg := testConfig(t, srv.URL)
	first := NewNanoAdapter().(paymentcontract.GatewayCapturer)
	created, _ := first.CreatePayment(context.Background(), cfg, createInput(7, "10.00"))
	if _, err := (nano.FileClaims{Dir: cfg["claim_dir"].(string)}).Claim(blockHash, "another-payment"); err != nil {
		t.Fatal(err)
	}
	node.block = sendOf(shopAccount, created.ProviderRef)
	restarted := NewNanoAdapter().(paymentcontract.GatewayCapturer)
	q, err := restarted.QueryPayment(context.Background(), cfg, created.ProviderRef)
	if !errors.Is(err, paymentcontract.ErrGatewayResponseInvalid) || !strings.Contains(err.Error(), "already settled") {
		t.Fatalf("replayed block accepted: %+v %v", q, err)
	}
}
