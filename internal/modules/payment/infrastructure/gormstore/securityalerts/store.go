package securityalerts

import (
	"context"
	"errors"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) ObserveSecurity(ctx context.Context, now time.Time) error {
	// Persist observations only after every query succeeds. Under READ COMMITTED
	// these are operational counts, not a cross-table accounting snapshot. No
	// payment gate lock: alerts must continue while payments freeze.
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var control pd.AutomationControl
		err := tx.Where("id = 1").Take(&control).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		missing, frozen := int64(0), int64(0)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			missing = 1
		} else if control.Frozen {
			frozen = 1
		}
		conditions := []pd.SecurityCondition{{Kind: "control_missing", Count: missing}, {Kind: "automation_frozen", Count: frozen}}
		queries := []struct {
			kind  string
			query *gorm.DB
		}{
			{"scanner_unhealthy", tx.Model(&pd.CandidateSubscription{}).Where("created_at < ? AND (scanner_healthy = ? OR last_success_at IS NULL OR last_success_at < ?)", now.Add(-5*time.Minute), false, now.Add(-3*time.Minute))},
			{"candidate_conflicts", tx.Model(&pd.CandidateReceipt{}).Where("conflicted = ?", true)},
			{"chain_manual", tx.Model(&pd.CandidateEvidence{}).Where("check_state = ?", "manual")},
			{"rpc_retry", tx.Model(&pd.CandidateEvidence{}).Where("check_state = ? AND check_code IN ? AND received_at < ?", "retry", []string{"rpc_unavailable", "rpc_disagreement"}, now.Add(-2*time.Minute))},
			{"chain_backlog", tx.Model(&pd.CandidateEvidence{}).Where("check_state IN ? AND received_at < ?", []string{"pending", "checking", "chain_checked", "settling"}, now.Add(-5*time.Minute))},
			{"intent_stalled", tx.Model(&pd.Payment{}).Where("requires_chain_verification = ? AND status IN ? AND (provider_ref IS NULL OR provider_ref = '') AND created_at < ?", true, []string{"initiated", "pending"}, now.Add(-5*time.Minute))},
			{"effects_manual", tx.Model(&pd.PaymentEffect{}).Where("state = ?", "manual")},
			{"effects_stalled", tx.Model(&pd.PaymentEffect{}).Where("state IN ? AND kind IN ? AND created_at < ?", []string{"pending", "working"}, []string{"member_level", "affiliate", "paid_notification", "paid_email", "paid_bot", "downstream", "auto_fulfill", "manual_notice"}, now.Add(-5*time.Minute))},
		}
		for _, q := range queries {
			var count int64
			if err := q.query.Count(&count).Error; err != nil {
				return err
			}
			conditions = append(conditions, pd.SecurityCondition{Kind: q.kind, Count: count})
		}
		for _, c := range conditions {
			if err := observe(tx, c, now); err != nil {
				return err
			}
		}
		return nil
	})
}

func observe(tx *gorm.DB, c pd.SecurityCondition, now time.Time) error {
	active := c.Count > 0
	seed := pd.SecurityAlert{Kind: c.Kind, Active: active, Count: c.Count, Version: 1, Pending: active, NextAt: now, ObservedAt: now}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&seed).Error; err != nil {
		return err
	}
	// Conditional UPDATE also serializes observers on SQLite (FOR UPDATE does not).
	if err := tx.Model(&pd.SecurityAlert{}).Where("kind = ?", c.Kind).Update("observed_at", now).Error; err != nil {
		return err
	}
	var row pd.SecurityAlert
	if err := tx.Where("kind = ?", c.Kind).Take(&row).Error; err != nil {
		return err
	}
	fields := map[string]interface{}{"count": c.Count, "observed_at": now}
	if row.Active != active {
		fields["active"] = active
		fields["version"] = row.Version + 1
		fields["pending"] = active || row.SentAt != nil || row.Token != ""
		fields["next_at"] = now
		fields["attempts"] = 0
	}
	return tx.Model(&row).Updates(fields).Error
}

func (s *Store) LeaseSecurityAlerts(ctx context.Context, now time.Time) ([]pd.SecurityAlert, error) {
	var candidates []pd.SecurityAlert
	if err := s.db.WithContext(ctx).Where("(active = ? OR pending = ?) AND next_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", true, true, now, now).Order("next_at ASC, kind ASC").Limit(3).Find(&candidates).Error; err != nil {
		return nil, err
	}
	var leased []pd.SecurityAlert
	for _, row := range candidates {
		token := uuid.NewString()
		until := now.Add(time.Minute)
		r := s.db.WithContext(ctx).Model(&pd.SecurityAlert{}).Where("kind = ? AND version = ? AND next_at <= ? AND (lease_until IS NULL OR lease_until <= ?)", row.Kind, row.Version, now, now).
			Updates(map[string]interface{}{"token": token, "lease_until": until, "attempts": gorm.Expr("attempts + 1")})
		if r.Error != nil {
			return nil, r.Error
		}
		if r.RowsAffected == 0 {
			continue
		}
		row.Token = token
		row.LeaseUntil = &until
		row.Attempts++
		leased = append(leased, row)
	}
	return leased, nil
}

func (s *Store) FinishSecurityAlert(ctx context.Context, w pd.SecurityAlert, success bool, now time.Time) error {
	delay := time.Duration(min(max(w.Attempts, 1), 15)) * time.Minute
	fields := map[string]interface{}{"pending": true, "token": "", "lease_until": nil, "next_at": now.Add(delay)}
	if success {
		fields["pending"] = false
		fields["sent_at"] = now
		fields["attempts"] = 0
		fields["next_at"] = now.Add(15 * time.Minute)
	}
	r := s.db.WithContext(ctx).Model(&pd.SecurityAlert{}).Where("kind = ? AND version = ? AND token = ? AND lease_until > ?", w.Kind, w.Version, w.Token, now).Updates(fields)
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return errors.New("security alert lease changed")
	}
	return nil
}
