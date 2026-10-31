// Package chainverification checks untrusted candidates without settling them.
package chainverification

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
)

type Service struct {
	Store    paymentcontract.ChainCheckStore
	Registry paymentcontract.GatewayRegistry
	Verifier paymentcontract.ChainVerifier
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.Store == nil || s.Registry == nil || s.Verifier == nil {
		return errors.New("chain verification unavailable")
	}
	var failures []error
	// Claim one item at a time so its two-minute lease never expires while it
	// waits behind other RPC calls. The periodic task has a bounded overall time.
	for count := 0; count < 5; count++ {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		items, err := s.Store.LeaseChainChecks(ctx, time.Now(), 1)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			break
		}
		item := items[0]
		state, code, proof := s.check(ctx, item)
		attempts := item.Evidence.CheckAttempts
		if attempts > 10 {
			attempts = 10
		}
		if attempts < 1 {
			attempts = 1
		}
		next := time.Now().Add(time.Duration(attempts) * time.Minute)
		if err := s.Store.FinishChainCheck(ctx, item, state, code, proof, next); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) check(ctx context.Context, w paymentdomain.ChainCheckWork) (string, string, *paymentdomain.ChainProof) {
	if w.Receipt.Conflicted {
		return "manual", "candidate_conflict", nil
	}
	var candidate paymentdomain.TransferCandidate
	if json.Unmarshal([]byte(w.Evidence.Payload), &candidate) != nil || len(w.Payments) != 1 {
		return "manual", "unmatched_or_ambiguous", nil
	}
	p := w.Payments[0]
	endpoint, eok := p.PaymentIntent["gateway_url"].(string)
	pid, pok := p.PaymentIntent["pid"].(string)
	if !eok || !pok || (paymentdomain.CandidateSource{GatewayURL: endpoint, PID: pid}).Key() != w.Receipt.SourceKey || candidate.OrderID != p.GatewayOrderNo {
		return "manual", "identity_mismatch", nil
	}
	provider, ok := s.Registry.Lookup(p.ProviderType, p.ChannelType)
	if !ok {
		return "manual", "provider_unavailable", nil
	}
	reader, ok := provider.(paymentcontract.GatewayChainIntentReader)
	if !ok {
		return "manual", "provider_unavailable", nil
	}
	expected, err := reader.ReadChainExpectation(p)
	if err != nil {
		if errors.Is(err, paymentcontract.ErrChainPending) && (p.ExpiredAt == nil || p.ExpiredAt.After(time.Now())) {
			return "retry", "awaiting_intent_binding", nil
		}
		return "manual", "invalid_or_unbound_intent", nil
	}
	if candidate.TradeID != p.ProviderRef || candidate.Network != expected.Network {
		return "manual", "identity_mismatch", nil
	}
	// Only the transaction hash is used as an RPC lookup hint. Candidate amount,
	// recipient, time, event index, Removed and review reason are not chain facts.
	proof, err := s.Verifier.Verify(ctx, expected, candidate.TransactionHash)
	switch {
	case err == nil && proof != nil:
		return "chain_checked", "independent_sources_agree", proof
	case errors.Is(err, paymentcontract.ErrChainPending):
		return "retry", "not_finalized", nil
	case errors.Is(err, paymentcontract.ErrChainUnavailable):
		return "retry", "rpc_unavailable", nil
	case errors.Is(err, paymentcontract.ErrChainDisagreement):
		return "retry", "rpc_disagreement", nil
	case errors.Is(err, paymentcontract.ErrChainOutsideWindow):
		return "manual", "outside_payment_window", nil
	case errors.Is(err, paymentcontract.ErrChainAmbiguous):
		return "manual", "multiple_matching_transfers", nil
	default:
		return "manual", "chain_evidence_mismatch", nil
	}
}
