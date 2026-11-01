package epusdt

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

// QuotePolicy is merchant-owned configuration. Rate means USDT per fiat unit.
// Version is an operator-supplied revision, never a gateway-provided quote ID.
type QuotePolicy struct {
	Version         string `json:"version"`
	Rate            string `json:"rate"`
	Precision       int    `json:"precision"`
	MaxTail         string `json:"max_tail"`
	LifetimeSeconds int    `json:"lifetime_seconds"`
	Recipient       string `json:"recipient"`
}

// FixedTerms mirrors the versioned GMPay wire contract, not its float fields.
type FixedTerms struct {
	Version       int    `json:"version"`
	TradeID       string `json:"trade_id"`
	OrderID       string `json:"order_id"`
	APIKeyID      uint64 `json:"api_key_id"`
	Network       string `json:"network"`
	Token         string `json:"token"`
	Contract      string `json:"contract"`
	Decimals      int    `json:"decimals"`
	Recipient     string `json:"recipient"`
	RawAmount     string `json:"raw_amount"`
	FiatAmount    string `json:"fiat_amount"`
	Currency      string `json:"currency"`
	RequestAmount string `json:"request_amount"`
	Rate          string `json:"rate"`
	CreatedAtMs   int64  `json:"created_at_ms"`
	ExpiresAtMs   int64  `json:"expires_at_ms"`
}

// PaymentIntent contains no credentials and is stored separately from callbacks.
// The entire authorization is persisted before HTTP; Terms is bound only once.
type PaymentIntent struct {
	Version     int         `json:"version"`
	PaymentID   uint        `json:"payment_id"`
	OrderID     uint        `json:"order_id"`
	GatewayURL  string      `json:"gateway_url"`
	PID         string      `json:"pid"`
	OrderNo     string      `json:"order_no"`
	Name        string      `json:"name"`
	NotifyURL   string      `json:"notify_url"`
	ReturnURL   string      `json:"return_url"`
	FiatAmount  string      `json:"fiat_amount"`
	Currency    string      `json:"currency"`
	Network     string      `json:"network"`
	Contract    string      `json:"contract"`
	Decimals    int         `json:"decimals"`
	Policy      QuotePolicy `json:"policy"`
	BaseRaw     string      `json:"base_raw"`
	MaxRaw      string      `json:"max_raw"`
	StepRaw     string      `json:"step_raw"`
	CreatedAtMs int64       `json:"created_at_ms"`
	ExpiresAtMs int64       `json:"expires_at_ms"`
	Terms       *FixedTerms `json:"terms,omitempty"`
}

var plainDecimal = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
var tradeIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func exactDecimal(s string) (decimal.Decimal, error) {
	if len(s) > 80 || !plainDecimal.MatchString(s) {
		return decimal.Zero, ErrConfigInvalid
	}
	return decimal.NewFromString(s)
}

func fixedAsset(network string) (string, int, error) {
	switch network {
	case "tron":
		return "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t", 6, nil
	case "binance":
		return "0x55d398326f99059ff775485246999027b3197955", 18, nil
	default:
		return "", 0, ErrConfigInvalid
	}
}

func fixedAddress(network, address string) (string, error) {
	if network == "binance" {
		if len(address) != 42 || !strings.HasPrefix(address, "0x") {
			return "", ErrConfigInvalid
		}
		b, err := hex.DecodeString(address[2:])
		if err != nil || bytes.Equal(b, make([]byte, 20)) {
			return "", ErrConfigInvalid
		}
		return strings.ToLower(address), nil
	}
	if network != "tron" || len(address) != 34 {
		return "", ErrConfigInvalid
	}
	n := new(big.Int)
	for _, c := range address {
		v := strings.IndexRune("123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz", c)
		if v < 0 {
			return "", ErrConfigInvalid
		}
		n.Mul(n, big.NewInt(58)).Add(n, big.NewInt(int64(v)))
	}
	b := n.Bytes()
	if len(b) != 25 || b[0] != 0x41 || bytes.Equal(b[1:21], make([]byte, 20)) {
		return "", ErrConfigInvalid
	}
	h := sha256.Sum256(b[:21])
	h = sha256.Sum256(h[:])
	if !bytes.Equal(b[21:], h[:4]) {
		return "", ErrConfigInvalid
	}
	return address, nil
}

func validateQuotePolicy(cfg *Config) error {
	if !cfg.FixedPaymentIntent {
		if cfg.QuotePolicy != nil {
			return fmt.Errorf("%w: quote policy requires fixed mode", ErrConfigInvalid)
		}
		return nil
	}
	p := cfg.QuotePolicy
	if p == nil || cfg.OrderMode != constants.PaymentEpusdtOrderModeTransaction || cfg.Token != "usdt" ||
		(cfg.PaymentType != "" && !strings.EqualFold(cfg.PaymentType, "Gmpay")) ||
		(cfg.Currency != "cny" && cfg.Currency != "usd") || p.Version == "" || len(p.Version) > 64 ||
		p.Precision < 2 || p.Precision > 6 || p.LifetimeSeconds < 60 || p.LifetimeSeconds > 86400 {
		return fmt.Errorf("%w: invalid fixed quote policy", ErrConfigInvalid)
	}
	if _, _, err := fixedAsset(cfg.Network); err != nil {
		return err
	}
	if _, err := fixedAddress(cfg.Network, p.Recipient); err != nil {
		return fmt.Errorf("%w: invalid recipient", ErrConfigInvalid)
	}
	rate, err := exactDecimal(p.Rate)
	if err != nil || !rate.IsPositive() {
		return fmt.Errorf("%w: invalid quote rate", ErrConfigInvalid)
	}
	tail, err := exactDecimal(p.MaxTail)
	step := decimal.New(1, -int32(p.Precision))
	if err != nil || tail.IsNegative() || !tail.Mod(step).IsZero() || tail.GreaterThan(step.Mul(decimal.NewFromInt(99))) {
		return fmt.Errorf("%w: tail must fit the 100 fixed allocation slots", ErrConfigInvalid)
	}
	return nil
}

func intentMap(intent *PaymentIntent) (jsonmap.JSON, error) {
	b, err := json.Marshal(intent)
	if err != nil {
		return nil, err
	}
	var m jsonmap.JSON
	err = json.Unmarshal(b, &m)
	return m, err
}

func decodeIntent(m jsonmap.JSON) (*PaymentIntent, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	var intent PaymentIntent
	if json.Unmarshal(b, &intent) != nil || intent.Version != 1 {
		return nil, ErrConfigInvalid
	}
	return &intent, nil
}

// PrepareIntent authorizes a quote locally. Config changes never reprice an
// existing intent; changing endpoint, PID or disabling fixed mode stops reuse.
func PrepareIntent(cfg *Config, input CreateInput, paymentID, orderID uint, currency string, deadline *time.Time, existing jsonmap.JSON, now time.Time) (jsonmap.JSON, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	if !cfg.FixedPaymentIntent {
		if len(existing) != 0 {
			return nil, ErrConfigInvalid
		}
		return nil, nil
	}
	if paymentID == 0 || orderID == 0 || len(input.OrderNo) == 0 || len(input.OrderNo) > 32 || !strings.EqualFold(currency, cfg.Currency) {
		return nil, ErrConfigInvalid
	}
	if len(existing) != 0 {
		i, err := decodeIntent(existing)
		if err != nil || i.PaymentID != paymentID || i.OrderID != orderID || i.OrderNo != input.OrderNo ||
			i.GatewayURL != cfg.GatewayURL || i.PID != cfg.PID || i.FiatAmount != input.Amount ||
			i.Currency != strings.ToUpper(currency) || i.ExpiresAtMs <= now.UnixMilli() ||
			(deadline != nil && i.ExpiresAtMs > deadline.UnixMilli()) {
			return nil, ErrConfigInvalid
		}
		return intentMap(i)
	}
	amount, err := exactDecimal(input.Amount)
	if err != nil || !amount.GreaterThan(decimal.RequireFromString("0.01")) || !amount.Equal(amount.Round(2)) {
		return nil, ErrConfigInvalid
	}
	// The current GMPay wire still has a float amount. Refuse a lossy bridge.
	wire := amount.InexactFloat64()
	if !decimal.NewFromFloat(wire).Equal(amount) {
		return nil, ErrConfigInvalid
	}
	contract, decimals, _ := fixedAsset(cfg.Network)
	p := *cfg.QuotePolicy
	p.Recipient, _ = fixedAddress(cfg.Network, p.Recipient)
	rate, _ := exactDecimal(p.Rate)
	tail, _ := exactDecimal(p.MaxTail)
	base := amount.Mul(rate).Round(int32(p.Precision)) // Explicit positive half-up display rounding.
	if base.LessThan(decimal.RequireFromString("0.01")) {
		return nil, ErrConfigInvalid
	}
	expires := now.Add(time.Duration(p.LifetimeSeconds) * time.Second).UnixMilli()
	if deadline != nil && deadline.UnixMilli() < expires {
		expires = deadline.UnixMilli()
	}
	if expires <= now.UnixMilli() {
		return nil, ErrConfigInvalid
	}
	i := &PaymentIntent{Version: 1, PaymentID: paymentID, OrderID: orderID, GatewayURL: cfg.GatewayURL, PID: cfg.PID,
		OrderNo: input.OrderNo, Name: input.Name, NotifyURL: input.NotifyURL, ReturnURL: input.ReturnURL,
		FiatAmount: amount.String(), Currency: strings.ToUpper(currency), Network: cfg.Network, Contract: contract, Decimals: decimals,
		Policy: p, BaseRaw: base.Shift(int32(decimals)).StringFixed(0), MaxRaw: base.Add(tail).Shift(int32(decimals)).StringFixed(0),
		StepRaw: decimal.New(1, int32(decimals-p.Precision)).StringFixed(0), CreatedAtMs: now.UnixMilli(), ExpiresAtMs: expires}
	if i.NotifyURL == "" {
		i.NotifyURL = cfg.NotifyURL
	}
	if i.ReturnURL == "" {
		i.ReturnURL = cfg.ReturnURL
	}
	return intentMap(i)
}

func validateFixedTerms(i *PaymentIntent, t *FixedTerms, now time.Time) error {
	if t == nil || t.Version != 1 || !tradeIDPattern.MatchString(t.TradeID) || t.OrderID != i.OrderNo || t.APIKeyID == 0 ||
		t.Network != i.Network || t.Token != "USDT" || t.Decimals != i.Decimals || t.Currency != i.Currency ||
		t.FiatAmount != i.FiatAmount || t.RequestAmount != i.FiatAmount ||
		t.CreatedAtMs < i.CreatedAtMs || t.CreatedAtMs > now.UnixMilli() ||
		t.ExpiresAtMs <= t.CreatedAtMs || t.ExpiresAtMs <= now.UnixMilli() || t.ExpiresAtMs > i.ExpiresAtMs {
		return ErrResponseInvalid
	}
	contract, err := fixedAddress(t.Network, t.Contract)
	if err != nil || contract != i.Contract {
		return ErrResponseInvalid
	}
	address, err := fixedAddress(t.Network, t.Recipient)
	if err != nil || address != i.Policy.Recipient {
		return ErrResponseInvalid
	}
	rate, err := exactDecimal(t.Rate)
	wantRate, wantErr := exactDecimal(i.Policy.Rate)
	if err != nil || wantErr != nil || !rate.Equal(wantRate) {
		return ErrResponseInvalid
	}
	raw, ok := new(big.Int).SetString(t.RawAmount, 10)
	base, bOK := new(big.Int).SetString(i.BaseRaw, 10)
	max, mOK := new(big.Int).SetString(i.MaxRaw, 10)
	step, sOK := new(big.Int).SetString(i.StepRaw, 10)
	if !ok || len(t.RawAmount) > 80 || raw.Sign() <= 0 || raw.String() != t.RawAmount || !bOK || !mOK || !sOK || step.Sign() <= 0 ||
		raw.Cmp(base) < 0 || raw.Cmp(max) > 0 || new(big.Int).Mod(new(big.Int).Sub(raw, base), step).Sign() != 0 {
		return ErrResponseInvalid
	}
	return nil
}

// CreateFixedPayment accepts only previously persisted merchant authorization.
// It deliberately returns no gateway-hosted cashier URL or raw response: the
// merchant cashier and independent chain verifier are still deployment gates.
func CreateFixedPayment(ctx context.Context, cfg *Config, saved jsonmap.JSON) (*CreateResult, error) {
	if cfg == nil {
		return nil, ErrConfigInvalid
	}
	i, err := decodeIntent(saved)
	if err != nil || !cfg.FixedPaymentIntent || i.GatewayURL != cfg.GatewayURL || i.PID != cfg.PID || i.ExpiresAtMs <= time.Now().UnixMilli() {
		return nil, ErrConfigInvalid
	}
	if i.Terms != nil {
		if err = validateFixedTerms(i, i.Terms, time.Now()); err != nil {
			return nil, err
		}
		return &CreateResult{TradeID: i.Terms.TradeID, PaymentIntent: saved, ExpiresAt: time.UnixMilli(i.Terms.ExpiresAtMs)}, nil
	}
	amount, err := exactDecimal(i.FiatAmount)
	if err != nil {
		return nil, ErrConfigInvalid
	}
	params := map[string]interface{}{"fixed_payment_intent": true, "pid": i.PID, "order_id": i.OrderNo, "amount": amount.InexactFloat64(),
		"payment_expires_at_ms": i.ExpiresAtMs,
		"currency":              strings.ToLower(i.Currency), "token": "usdt", "network": i.Network, "notify_url": i.NotifyURL, "redirect_url": i.ReturnURL, "name": i.Name}
	params["signature"] = Sign(params, cfg.SecretKey)
	b, err := servicePost(ctx, cfg, i.GatewayURL+gmpayCreateTransactionPath, params, 64<<10)
	if err != nil {
		return nil, fmt.Errorf("%w: fixed create transport failed", ErrRequestFailed)
	}
	var response struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			TradeID        string      `json:"trade_id"`
			OrderID        string      `json:"order_id"`
			Status         int         `json:"status"`
			Amount         json.Number `json:"amount"`
			ActualAmount   json.Number `json:"actual_amount"`
			Currency       string      `json:"currency"`
			Token          string      `json:"token"`
			Recipient      string      `json:"receive_address"`
			ExpirationTime int64       `json:"expiration_time"`
			Terms          *FixedTerms `json:"payment_terms"`
		} `json:"data"`
	}
	if json.Unmarshal(b, &response) != nil || response.StatusCode != 200 {
		return nil, ErrResponseInvalid
	}
	d := response.Data
	if err = validateFixedTerms(i, d.Terms, time.Now()); err != nil {
		return nil, err
	}
	t := d.Terms
	fiat, fErr := exactDecimal(string(d.Amount))
	actual, aErr := exactDecimal(string(d.ActualAmount))
	recipient, rErr := fixedAddress(i.Network, d.Recipient)
	if d.Status != StatusWaiting || d.TradeID != t.TradeID || d.OrderID != i.OrderNo || d.Currency != i.Currency || d.Token != "USDT" ||
		fErr != nil || !fiat.Equal(amount) || aErr != nil || actual.Shift(int32(i.Decimals)).String() != t.RawAmount ||
		rErr != nil || recipient != i.Policy.Recipient || d.ExpirationTime != t.ExpiresAtMs/1000 {
		return nil, ErrResponseInvalid
	}
	i.Terms = t
	bound, err := intentMap(i)
	if err != nil {
		return nil, err
	}
	return &CreateResult{TradeID: t.TradeID, PaymentIntent: bound, ExpiresAt: time.UnixMilli(t.ExpiresAtMs)}, nil
}

// IntentDeadline exposes only the merchant authorization deadline to scheduling.
func IntentDeadline(saved jsonmap.JSON) (time.Time, error) {
	i, err := decodeIntent(saved)
	if err != nil || i.ExpiresAtMs <= i.CreatedAtMs {
		return time.Time{}, ErrConfigInvalid
	}
	return time.UnixMilli(i.ExpiresAtMs), nil
}

// readSavedAuthorization reconstructs the original quote, even after expiry.
// Bound terms are validated separately; this also audits unbound history.
func readSavedAuthorization(p paymentdomain.Payment) (*PaymentIntent, error) {
	i, err := decodeIntent(p.PaymentIntent)
	if err != nil || !p.RequiresChainVerification || p.ProviderType != constants.PaymentProviderEpusdt || i.PaymentID != p.ID || i.OrderID != p.OrderID || i.OrderNo != p.GatewayOrderNo ||
		i.FiatAmount != p.Amount.Decimal.String() || i.Currency != strings.ToUpper(p.Currency) || i.CreatedAtMs <= 0 || i.ExpiresAtMs <= i.CreatedAtMs {
		return nil, paymentcontract.ErrChainMismatch
	}
	cfg := &Config{FixedPaymentIntent: true, QuotePolicy: &i.Policy, GatewayURL: i.GatewayURL, PID: i.PID, SecretKey: "local-validation-only", OrderMode: constants.PaymentEpusdtOrderModeTransaction,
		Token: "usdt", Network: i.Network, Currency: strings.ToLower(i.Currency), NotifyURL: i.NotifyURL, ReturnURL: i.ReturnURL}
	deadline := time.UnixMilli(i.ExpiresAtMs)
	rebuilt, err := PrepareIntent(cfg, CreateInput{OrderNo: i.OrderNo, Amount: i.FiatAmount, Name: i.Name, NotifyURL: i.NotifyURL, ReturnURL: i.ReturnURL}, p.ID, p.OrderID, i.Currency, &deadline, nil, time.UnixMilli(i.CreatedAtMs))
	if err != nil {
		return nil, paymentcontract.ErrChainMismatch
	}
	authorization := *i
	authorization.Terms = nil
	expected, _ := json.Marshal(rebuilt)
	original, _ := intentMap(&authorization)
	actual, _ := json.Marshal(original)
	if !bytes.Equal(expected, actual) {
		return nil, paymentcontract.ErrChainMismatch
	}
	return i, nil
}

// ValidUnboundHistory recognizes only a fully reconstructible pre-HTTP intent.
// Even an explicit null/empty terms key is suspicious: our writer omits it.
// Expiry/deletion never converts a previously bound payment into this state.
func ValidUnboundHistory(p paymentdomain.Payment) bool {
	if p.ID == 0 || p.OrderID == 0 || p.GatewayOrderNo == "" || p.ProviderRef != "" || len(p.SettlementBasis) != 64 {
		return false
	}
	if _, present := p.PaymentIntent["terms"]; present {
		return false
	}
	for _, c := range p.SettlementBasis {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	i, err := readSavedAuthorization(p)
	return err == nil && i.Terms == nil && p.PaidAt == nil && p.Status != constants.PaymentStatusSuccess
}

func ReadChainExpectation(p paymentdomain.Payment) (paymentdomain.ChainExpectation, error) {
	empty := paymentdomain.ChainExpectation{}
	i, err := readSavedAuthorization(p)
	if err != nil {
		return empty, err
	}
	if i.Terms == nil {
		return empty, paymentcontract.ErrChainPending
	}
	if i.Terms.TradeID != p.ProviderRef || validateFixedTerms(i, i.Terms, time.UnixMilli(i.Terms.CreatedAtMs)) != nil {
		return empty, paymentcontract.ErrChainMismatch
	}
	return paymentdomain.ChainExpectation{Network: i.Network, Contract: i.Contract, Recipient: i.Policy.Recipient, RawAmount: i.Terms.RawAmount, CreatedAtMs: i.Terms.CreatedAtMs, ExpiresAtMs: i.Terms.ExpiresAtMs}, nil
}
