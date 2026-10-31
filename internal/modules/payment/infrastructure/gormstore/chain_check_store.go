package gormstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Store) LeaseChainChecks(ctx context.Context, now time.Time, limit int) ([]paymentdomain.ChainCheckWork, error) {
	if limit < 1 || limit > 5 {
		limit = 5
	}
	var rows []paymentdomain.CandidateEvidence
	if err := r.db.WithContext(ctx).Where("check_state IN ? AND (check_after IS NULL OR check_after <= ?)", []string{"pending", "retry", "checking"}, now).Order("check_after ASC, id ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	var work []paymentdomain.ChainCheckWork
	for _, row := range rows {
		token := uuid.NewString()
		lease := now.Add(2 * time.Minute)
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			update := tx.Model(&paymentdomain.CandidateEvidence{}).Where("id = ? AND check_state IN ? AND (check_after IS NULL OR check_after <= ?)", row.ID, []string{"pending", "retry", "checking"}, now).
				Updates(map[string]interface{}{"check_state": "checking", "check_token": token, "check_after": lease, "check_attempts": gorm.Expr("check_attempts + 1")})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected != 1 {
				return nil
			}
			var item paymentdomain.ChainCheckWork
			if err := tx.First(&item.Evidence, row.ID).Error; err != nil {
				return err
			}
			if err := tx.First(&item.Receipt, row.ReceiptID).Error; err != nil {
				return err
			}
			var candidate paymentdomain.TransferCandidate
			if json.Unmarshal([]byte(row.Payload), &candidate) == nil && candidate.OrderID != "" {
				if err := tx.Where("gateway_order_no = ?", candidate.OrderID).Limit(2).Find(&item.Payments).Error; err != nil {
					return err
				}
			}
			work = append(work, item)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return work, nil
}

// FinishChainCheck can only write verification metadata. It has no order,
// wallet, event-consumption or delivery side effects. An expiring lease cannot
// overwrite a newer attempt, conflicting observation, or changed authorization.
func (r *Store) FinishChainCheck(ctx context.Context, work paymentdomain.ChainCheckWork, state, code string, proof *paymentdomain.ChainProof, next time.Time) error {
	if state != "retry" && state != "manual" && state != "chain_checked" {
		return errors.New("invalid chain check state")
	}
	if (proof != nil) != (state == "chain_checked") {
		return errors.New("invalid chain check proof")
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt paymentdomain.CandidateReceipt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&receipt, work.Receipt.ID).Error; err != nil {
			return err
		}
		var current paymentdomain.CandidateEvidence
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, work.Evidence.ID).Error; err != nil {
			return err
		}
		if current.CheckToken != work.Evidence.CheckToken || current.CheckState != "checking" {
			return nil
		}
		if receipt.Conflicted {
			state, code, proof = "manual", "candidate_conflict", nil
		}
		if !receipt.Conflicted && len(work.Payments) == 1 {
			var payment paymentdomain.Payment
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&payment, work.Payments[0].ID).Error; err != nil {
				return err
			}
			before, _ := json.Marshal(work.Payments[0].PaymentIntent)
			after, _ := json.Marshal(payment.PaymentIntent)
			if payment.OrderID != work.Payments[0].OrderID || payment.ChannelID != work.Payments[0].ChannelID || payment.ProviderType != work.Payments[0].ProviderType || !bytes.Equal(before, after) || !payment.RequiresChainVerification || payment.ProviderRef != work.Payments[0].ProviderRef || payment.GatewayOrderNo != work.Payments[0].GatewayOrderNo ||
				payment.Currency != work.Payments[0].Currency || !payment.Amount.Decimal.Equal(work.Payments[0].Amount.Decimal) {
				state, code, proof = "retry", "authorization_changed", nil
			}
		}
		fields := map[string]interface{}{"check_state": state, "check_code": code, "check_token": "", "check_after": next, "chain_proof": "", "event_key": ""}
		if state != "retry" {
			fields["check_after"] = nil
		}
		if proof != nil {
			if len(work.Payments) != 1 || proof.Version != 1 || len(proof.Sources) != 2 || len(proof.Finality) != 2 {
				return errors.New("invalid chain check proof")
			}
			payload, err := json.Marshal(proof)
			if err != nil {
				return err
			}
			fields["chain_proof"], fields["event_key"], fields["payment_id"] = string(payload), proof.EventKey(), work.Payments[0].ID
		}
		return tx.Model(&current).Updates(fields).Error
	})
}
