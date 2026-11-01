package gormstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/dujiao-next/internal/constants"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CommitCandidates returns only after every receipt AND evidence row commits.
// Redelivery after a lost ACK is harmless. A changed payload appends evidence
// and permanently flags the receipt, including concurrent conflicting delivery.
func (r *Store) CommitCandidates(ctx context.Context, source paymentdomain.CandidateSource, candidates []paymentdomain.TransferCandidate) error {
	if source.GatewayURL == "" || source.PID == "" || len(candidates) > 100 {
		return errors.New("invalid candidate batch")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, candidate := range candidates {
			id, err := strconv.ParseUint(candidate.ID, 10, 64)
			if err != nil || id == 0 || strconv.FormatUint(id, 10) != candidate.ID {
				return errors.New("invalid candidate id")
			}
			payload, err := json.Marshal(candidate)
			if err != nil || len(payload) > 4096 {
				return errors.New("invalid candidate evidence")
			}
			h := sha256.Sum256(payload)
			digest := hex.EncodeToString(h[:])
			now := time.Now()
			receipt := paymentdomain.CandidateReceipt{SourceKey: source.Key(), CandidateID: candidate.ID, FirstDigest: digest, ReceivedAt: now}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&receipt).Error; err != nil {
				return err
			}
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("source_key = ? AND candidate_id = ?", source.Key(), candidate.ID).First(&receipt).Error; err != nil {
				return err
			}
			if receipt.FirstDigest != digest {
				if err := tx.Model(&receipt).Update("conflicted", true).Error; err != nil {
					return err
				}
				receipt.Conflicted = true
				if err := tx.Model(&paymentdomain.CandidateEvidence{}).Where("receipt_id = ?", receipt.ID).Updates(map[string]interface{}{"check_state": "manual", "check_code": "candidate_conflict", "check_token": "", "check_after": nil}).Error; err != nil {
					return err
				}
			}
			paymentID, state, err := matchCandidatePayment(tx, source, candidate)
			if err != nil {
				return err
			}
			evidence := paymentdomain.CandidateEvidence{ReceiptID: receipt.ID, Digest: digest, Payload: string(payload), PaymentID: paymentID, MatchState: state, ReceivedAt: now}
			if receipt.Conflicted {
				evidence.CheckState = "manual"
				evidence.CheckCode = "candidate_conflict"
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&evidence).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func matchCandidatePayment(tx *gorm.DB, source paymentdomain.CandidateSource, c paymentdomain.TransferCandidate) (*uint, string, error) {
	var payments []paymentdomain.Payment
	// Include expired and soft-deleted history. Multiple local identities are
	// ambiguous; never choose the most recent one to resolve an economic event.
	if err := tx.Where("gateway_order_no = ?", c.OrderID).Limit(2).Find(&payments).Error; err != nil {
		return nil, "", err
	}
	if len(payments) != 1 {
		return nil, "unmatched_or_ambiguous", nil
	}
	p := payments[0]
	var identity struct {
		PaymentID  uint   `json:"payment_id"`
		OrderID    uint   `json:"order_id"`
		GatewayURL string `json:"gateway_url"`
		PID        string `json:"pid"`
		OrderNo    string `json:"order_no"`
		Terms      *struct {
			TradeID string `json:"trade_id"`
		} `json:"terms"`
	}
	b, err := json.Marshal(p.PaymentIntent)
	if err != nil || json.Unmarshal(b, &identity) != nil || !p.RequiresChainVerification || p.ProviderType != constants.PaymentProviderEpusdt ||
		identity.PaymentID != p.ID || identity.OrderID != p.OrderID || identity.OrderNo != c.OrderID ||
		identity.GatewayURL != source.GatewayURL || identity.PID != source.PID {
		return nil, "identity_mismatch", nil
	}
	if identity.Terms == nil {
		return &p.ID, "awaiting_intent_binding", nil
	}
	if identity.Terms.TradeID != c.TradeID || p.ProviderRef != c.TradeID {
		return nil, "identity_mismatch", nil
	}
	return &p.ID, "awaiting_chain_check", nil
}

func (r *Store) ListIntentRecoveries(now time.Time, limit int) ([]paymentdomain.Payment, error) {
	if limit < 1 || limit > 20 {
		limit = 20
	}
	var payments []paymentdomain.Payment
	err := r.db.Where("requires_chain_verification = true AND provider_ref = '' AND deleted_at IS NULL AND superseded_at IS NULL AND status IN ? AND expired_at > ? AND (intent_retry_at IS NULL OR intent_retry_at <= ?)",
		[]string{constants.PaymentStatusInitiated, constants.PaymentStatusPending}, now, now).
		Order("intent_retry_at ASC, id ASC").Limit(limit).Find(&payments).Error
	return payments, err
}

// A bounded lease survives worker death; all HTTP retries still use the same
// durable gateway order number. No network request holds a database transaction.
func (r *Store) ClaimIntentRecovery(id uint, now, next time.Time) (bool, error) {
	result := r.db.Model(&paymentdomain.Payment{}).Where("id = ? AND requires_chain_verification = true AND provider_ref = '' AND deleted_at IS NULL AND superseded_at IS NULL AND status IN ? AND expired_at > ? AND (intent_retry_at IS NULL OR intent_retry_at <= ?)",
		id, []string{constants.PaymentStatusInitiated, constants.PaymentStatusPending}, now, now).Update("intent_retry_at", next)
	return result.RowsAffected == 1, result.Error
}
