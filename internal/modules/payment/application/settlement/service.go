// Package settlement is the only automatic settlement entrance for fixed intents.
package settlement

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	od "github.com/dujiao-next/internal/modules/order/domain"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
)

type Service struct {
	Store    pc.SettlementStore
	Registry pc.GatewayRegistry
	Verifier pc.ChainVerifier
	Apply    pc.SettlementApplier
}

func (s *Service) Run(ctx context.Context) error {
	if s == nil || s.Store == nil || s.Registry == nil || s.Verifier == nil || s.Apply == nil {
		return errors.New("chain settlement unavailable")
	}
	var failures []error
	for n := 0; n < 5; n++ {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		rows, err := s.Store.LeaseSettlements(ctx, time.Now())
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		w := rows[0]
		reader, expected, code := s.expectation(w)
		var proof *pd.ChainProof
		state := "manual"
		if code == "" {
			var candidate pd.TransferCandidate
			_ = json.Unmarshal([]byte(w.Evidence.Payload), &candidate)
			// A new RPC verification under CURRENT server policy is mandatory.
			// Stored chain_checked/proof is discovery history, not a spend token.
			proof, err = s.Verifier.Verify(ctx, expected, candidate.TransactionHash)
			switch {
			case err == nil && proof != nil:
				state, code = "settled", "independent_settlement"
			case errors.Is(err, pc.ErrChainPending), errors.Is(err, pc.ErrChainUnavailable), errors.Is(err, pc.ErrChainDisagreement):
				state, code = "chain_checked", "verification_retry"
			default:
				code = "verification_rejected"
			}
		}
		err = s.Store.WithSettlement(ctx, w, func(tx pc.SettlementTransaction, p *pd.Payment, o *od.Order, e *pd.CandidateEvidence, r *pd.CandidateReceipt) error {
			now := time.Now()
			finish := func(state, code string) error {
				return s.Store.FinishSettlement(tx, e, state, code, now.Add(time.Minute))
			}
			if r.Conflicted || r.ID != w.Receipt.ID || r.SourceKey != w.Receipt.SourceKey || e.ReceiptID != r.ID || e.Payload != w.Evidence.Payload || e.PaymentID == nil || *e.PaymentID != w.Payment.ID {
				return finish("manual", "candidate_conflict")
			}
			if state != "settled" {
				return finish(state, code)
			}
			if p == nil || o == nil || p.OrderID != o.ID || p.ID != w.Payment.ID || !p.RequiresChainVerification || p.DeletedAt != nil || p.SupersededAt != nil || p.ProviderType != constants.PaymentProviderEpusdt || p.ProviderRef != w.Payment.ProviderRef || p.GatewayOrderNo != w.Payment.GatewayOrderNo {
				return finish("manual", "payment_identity_changed")
			}
			current, err := reader.ReadChainExpectation(*p)
			if err != nil || current != expected || p.SettlementBasis == "" || p.SettlementBasis != pd.OrderBasis(*p, *o) {
				return finish("manual", "authorization_changed")
			}
			if code := validateOrder(*p, *o, expected); code != "" {
				return finish("manual", code)
			}
			if !validProof(proof, expected, now) {
				return finish("chain_checked", "proof_stale_or_invalid")
			}
			a, err := tx.Allocation(p.ID)
			if err != nil {
				return err
			}
			if a == nil || a.OrderID != o.ID || a.SlotKey != expected.SlotKey() || a.Snapshot != pd.SettlementSnapshot(*p, *o) || a.Network != expected.Network || a.Contract != expected.Contract || a.Recipient != expected.Recipient || a.RawAmount != expected.RawAmount {
				return finish("manual", "allocation_missing_or_changed")
			}
			if a.CreatedAt.IsZero() || a.CreatedAt.UnixMilli() < expected.CreatedAtMs || proof.BlockTimeMs < a.CreatedAt.UnixMilli() {
				return finish("manual", "before_merchant_allocation")
			}
			body, err := json.Marshal(proof)
			if err != nil {
				return err
			}
			consumed, err := tx.ConsumeEvent(pd.EventConsumption{EventKey: proof.EventKey(), PaymentID: p.ID, OrderID: o.ID, EvidenceID: e.ID, AllocationID: a.ID, Proof: string(body), CreatedAt: now})
			if err != nil {
				return err
			}
			if !consumed {
				return finish("manual", "event_or_order_already_consumed")
			}
			// All financial changes and durable effect intents commit with consumption.
			if err := s.Apply(tx, o, p, now); err != nil {
				return err
			}
			if err := tx.AddPaymentEffects(effects(*p, *o, now)); err != nil {
				return err
			}
			return finish("settled", "independent_settlement")
		})
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

func (s *Service) expectation(w pc.SettlementWork) (pc.GatewayChainIntentReader, pd.ChainExpectation, string) {
	empty := pd.ChainExpectation{}
	var candidate pd.TransferCandidate
	p := w.Payment
	if w.Receipt.Conflicted || p.ID == 0 || json.Unmarshal([]byte(w.Evidence.Payload), &candidate) != nil {
		return nil, empty, "candidate_conflict"
	}
	endpoint, eok := p.PaymentIntent["gateway_url"].(string)
	pid, pok := p.PaymentIntent["pid"].(string)
	if !eok || !pok || (pd.CandidateSource{GatewayURL: endpoint, PID: pid}).Key() != w.Receipt.SourceKey || candidate.OrderID != p.GatewayOrderNo || candidate.TradeID != p.ProviderRef {
		return nil, empty, "identity_mismatch"
	}
	provider, ok := s.Registry.Lookup(p.ProviderType, p.ChannelType)
	if !ok {
		return nil, empty, "provider_unavailable"
	}
	reader, ok := provider.(pc.GatewayChainIntentReader)
	if !ok {
		return nil, empty, "provider_unavailable"
	}
	expected, err := reader.ReadChainExpectation(p)
	if err != nil || candidate.Network != expected.Network {
		return nil, empty, "invalid_intent"
	}
	return reader, expected, ""
}

func validateOrder(p pd.Payment, o od.Order, e pd.ChainExpectation) string {
	if o.UserID == 0 || o.ParentID != nil || o.DeletedAt != nil || o.CanceledAt != nil || o.PaidAt != nil || o.Status != constants.OrderStatusPendingPayment || !o.RefundedAmount.Decimal.IsZero() {
		return "order_not_payable"
	}
	if p.Status != constants.PaymentStatusPending || p.PaidAt != nil || p.ExceptionCode != "" {
		return "payment_not_payable"
	}
	// Passing wall-clock expiry is fine if inclusion was in-window and the
	// cancellation worker has not closed/released the order. Never reopen it.
	if o.ExpiresAt == nil || e.ExpiresAtMs > o.ExpiresAt.UnixMilli() {
		return "order_window_changed"
	}
	if !strings.EqualFold(p.Currency, o.Currency) || !p.Amount.Decimal.IsPositive() || o.WalletPaidAmount.Decimal.IsNegative() || !o.TotalAmount.Decimal.IsPositive() || p.FeeAmount.Decimal.IsNegative() {
		return "economic_facts_invalid"
	}
	// Existing checkout records the expected online share before payment.
	// Only the pending lifecycle + independent event consumption grant payment.
	if !o.OnlinePaidAmount.Decimal.IsZero() && !o.OnlinePaidAmount.Decimal.Equal(o.TotalAmount.Decimal.Sub(o.WalletPaidAmount.Decimal)) {
		return "wallet_online_split_changed"
	}
	covered := p.Amount.Decimal
	switch p.FeePolicy {
	case constants.PaymentFeePolicyCustomerSurcharge:
		covered = covered.Sub(p.FeeAmount.Decimal)
	case constants.PaymentFeePolicyNone:
		if !p.FeeAmount.Decimal.IsZero() {
			return "economic_facts_invalid"
		}
	case constants.PaymentFeePolicyMerchantAbsorbed:
	default:
		return "economic_facts_invalid"
	}
	if !covered.IsPositive() || !covered.Equal(o.TotalAmount.Decimal.Sub(o.WalletPaidAmount.Decimal)) {
		return "wallet_online_split_changed"
	}
	for _, child := range o.Children {
		if child.Status != constants.OrderStatusPendingPayment || child.PaidAt != nil || child.CanceledAt != nil || child.UserID != o.UserID || child.DeletedAt != nil || !child.RefundedAmount.Decimal.IsZero() {
			return "child_not_payable"
		}
	}
	orders := o.Children
	if len(orders) == 0 {
		orders = []od.Order{o}
	}
	for _, leaf := range orders {
		for _, item := range leaf.Items {
			if item.FulfillmentType == constants.FulfillmentTypeUpstream {
				return "upstream_delivery_not_ready"
			}
		}
	}
	return ""
}

func validProof(p *pd.ChainProof, e pd.ChainExpectation, now time.Time) bool {
	return p != nil && p.Version == 1 && len(p.Sources) == 2 && len(p.Finality) == 2 && p.PolicyVersion != "" && p.TransactionHash != "" && p.BlockHash != "" &&
		p.Network == e.Network && p.Contract == e.Contract && p.Recipient == e.Recipient && p.RawAmount == e.RawAmount && p.BlockTimeMs >= e.CreatedAtMs && p.BlockTimeMs < e.ExpiresAtMs &&
		p.CheckedAtMs <= now.Add(5*time.Second).UnixMilli() && p.CheckedAtMs >= now.Add(-time.Minute).UnixMilli()
}

func effects(p pd.Payment, o od.Order, now time.Time) []pd.PaymentEffect {
	var rows []pd.PaymentEffect
	add := func(id uint, kind string) {
		rows = append(rows, pd.PaymentEffect{PaymentID: p.ID, OrderID: id, Kind: kind, State: "pending", CreatedAt: now})
	}
	for _, kind := range []string{"member_level", "affiliate", "paid_notification", "paid_email", "paid_bot", "downstream", "delivery_email", "delivery_bot", "delivery_downstream"} {
		add(o.ID, kind)
	}
	orders := o.Children
	if len(orders) == 0 {
		orders = []od.Order{o}
	}
	for _, order := range orders {
		auto, upstream := len(order.Items) > 0, false
		for _, item := range order.Items {
			auto = auto && item.FulfillmentType == constants.FulfillmentTypeAuto
			upstream = upstream || item.FulfillmentType == constants.FulfillmentTypeUpstream
		}
		if auto {
			add(order.ID, "auto_fulfill")
		}
		if !auto && !upstream {
			add(order.ID, "manual_notice")
		}
	}
	return rows
}
