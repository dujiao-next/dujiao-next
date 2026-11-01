// Package chainrpc independently verifies USDT transfers using merchant-owned
// RPC configuration. No endpoint, contract or recipient is read from a gateway.
package chainrpc

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"golang.org/x/time/rate"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

const transferTopic = "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
const bscUSDT = "0x55d398326f99059ff775485246999027b3197955"
const tronUSDT = "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t"

type Source struct {
	Name     string
	Provider string
	URL      string
	APIKey   string // TRON-PRO-API-KEY, never persisted in proof or errors.
}

type NetworkPolicy struct {
	Sources      []Source
	AnchorNumber uint64
	AnchorHash   string // Operator-pinned mainnet block from an independent record.
}

type Policy struct {
	Version    string
	MaxHeadAge time.Duration
	BSC        NetworkPolicy
	TRON       NetworkPolicy
}

type Verifier struct {
	policy Policy
	client *http.Client
	now    func() time.Time
	limits map[string]*rate.Limiter
}

func New(policy Policy) (*Verifier, error) {
	if policy.Version == "" || len(policy.Version) > 64 || policy.MaxHeadAge < 30*time.Second || policy.MaxHeadAge > 10*time.Minute {
		return nil, errors.New("invalid chain verification policy")
	}
	for network, p := range map[string]NetworkPolicy{"binance": policy.BSC, "tron": policy.TRON} {
		if len(p.Sources) != 2 || p.AnchorNumber > uint64(^uint64(0)>>1) || canonicalHash(p.AnchorHash, network == "binance") == "" {
			return nil, errors.New("two RPC sources and pinned anchor required")
		}
		providers, hosts, names := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, source := range p.Sources {
			u, err := url.Parse(source.URL)
			provider := strings.ToLower(strings.TrimSpace(source.Provider))
			if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || (u.Port() != "" && u.Port() != "443") ||
				source.Name == "" || len(source.Name) > 64 || provider == "" || providers[provider] || hosts[strings.ToLower(u.Hostname())] || names[source.Name] || strings.ContainsAny(source.APIKey, "\r\n") {
				return nil, errors.New("invalid or non-independent chain RPC sources")
			}
			providers[provider], hosts[strings.ToLower(u.Hostname())], names[source.Name] = true, true, true
		}
	}
	// Defensive copy: later config mutation cannot silently change verification.
	policy.BSC.Sources = append([]Source(nil), policy.BSC.Sources...)
	policy.TRON.Sources = append([]Source(nil), policy.TRON.Sources...)
	limits := map[string]*rate.Limiter{}
	for _, network := range []NetworkPolicy{policy.BSC, policy.TRON} {
		for _, source := range network.Sources {
			key := strings.ToLower(strings.TrimSpace(source.Provider))
			limits[key] = rate.NewLimiter(rate.Limit(2), 1)
		}
	}
	return &Verifier{policy: policy, client: rpcHTTPClient(), now: time.Now, limits: limits}, nil
}

func (v *Verifier) Verify(ctx context.Context, e paymentdomain.ChainExpectation, txID string) (*paymentdomain.ChainProof, error) {
	if e.CreatedAtMs <= 0 || e.ExpiresAtMs <= e.CreatedAtMs || len(e.RawAmount) > 78 {
		return nil, paymentcontract.ErrChainMismatch
	}
	raw, ok := new(big.Int).SetString(e.RawAmount, 10)
	if !ok || raw.Sign() <= 0 || raw.BitLen() > 256 || raw.String() != e.RawAmount {
		return nil, paymentcontract.ErrChainMismatch
	}
	var policy NetworkPolicy
	switch e.Network {
	case "binance":
		if e.Contract != bscUSDT || evmAddress(e.Recipient) == "" || e.Recipient != evmAddress(e.Recipient) {
			return nil, paymentcontract.ErrChainMismatch
		}
		policy = v.policy.BSC
	case "tron":
		if e.Contract != tronUSDT || tronAddress(e.Contract) == "" || tronAddress(e.Recipient) == "" {
			return nil, paymentcontract.ErrChainMismatch
		}
		policy = v.policy.TRON
	default:
		return nil, paymentcontract.ErrChainMismatch
	}
	txID = canonicalHash(txID, e.Network == "binance")
	if txID == "" {
		return nil, paymentcontract.ErrChainMismatch
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	var agreed *paymentdomain.ChainProof
	for _, source := range policy.Sources {
		var proof *paymentdomain.ChainProof
		var err error
		if e.Network == "binance" {
			proof, err = v.verifyBSC(ctx, source, policy, e, txID)
		} else {
			proof, err = v.verifyTRON(ctx, source, policy, e, txID)
		}
		if err != nil {
			return nil, err
		} // Never fall back to a single available source.
		if agreed == nil {
			agreed = proof
		} else {
			left, right := *agreed, *proof
			left.Finality, right.Finality = nil, nil
			a, _ := json.Marshal(left)
			b, _ := json.Marshal(right)
			if !bytes.Equal(a, b) {
				return nil, paymentcontract.ErrChainDisagreement
			}
			agreed.Finality = append(agreed.Finality, proof.Finality...)
		}
	}
	if agreed == nil {
		return nil, paymentcontract.ErrChainUnavailable
	}
	agreed.Version, agreed.PolicyVersion, agreed.CheckedAtMs = 1, v.policy.Version, v.now().UnixMilli()
	for _, source := range policy.Sources {
		agreed.Sources = append(agreed.Sources, source.Name)
	}
	return agreed, nil
}

func canonicalHash(s string, prefix bool) string {
	if prefix {
		if !strings.HasPrefix(s, "0x") {
			return ""
		}
		s = s[2:]
	}
	if len(s) != 64 {
		return ""
	}
	b, err := hex.DecodeString(s)
	if err != nil || bytes.Equal(b, make([]byte, 32)) {
		return ""
	}
	if prefix {
		return "0x" + strings.ToLower(s)
	}
	return strings.ToLower(s)
}

func evmAddress(s string) string {
	if len(s) != 42 || !strings.HasPrefix(s, "0x") {
		return ""
	}
	b, err := hex.DecodeString(s[2:])
	if err != nil || bytes.Equal(b, make([]byte, 20)) {
		return ""
	}
	return "0x" + strings.ToLower(s[2:])
}

func quantity(s string) (uint64, error) {
	if !strings.HasPrefix(s, "0x") || len(s) < 3 || (len(s) > 3 && s[2] == '0') {
		return 0, paymentcontract.ErrChainUnavailable
	}
	n, err := strconv.ParseUint(s[2:], 16, 64)
	if err != nil {
		return 0, paymentcontract.ErrChainUnavailable
	}
	return n, nil
}

func (v *Verifier) freshHead(timestamp int64) bool {
	now := v.now().UnixMilli()
	return timestamp > 0 && timestamp <= now+30000 && timestamp >= now-v.policy.MaxHeadAge.Milliseconds()
}

func (v *Verifier) checkWindow(e paymentdomain.ChainExpectation, timestamp int64) error {
	if timestamp <= 0 || timestamp > v.now().Add(30*time.Second).UnixMilli() {
		return paymentcontract.ErrChainUnavailable
	}
	if timestamp < e.CreatedAtMs || timestamp >= e.ExpiresAtMs {
		return paymentcontract.ErrChainOutsideWindow
	}
	return nil
}
