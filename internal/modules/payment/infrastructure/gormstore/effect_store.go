package gormstore

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/constants"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

func (r *Store) LeasePaymentEffects(ctx context.Context, now time.Time) ([]pd.PaymentEffect, error) {
	revision, err := automation.OpenRevision(r.db.WithContext(ctx))
	if errors.Is(err, pd.ErrAutomationFrozen) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []pd.PaymentEffect
	if err := r.db.WithContext(ctx).Where("state IN ? AND (after IS NULL OR after <= ?)", []string{"pending", "working"}, now).Order("id ASC").Limit(1).Find(&rows).Error; err != nil {
		return nil, err
	}
	var work []pd.PaymentEffect
	for _, row := range rows {
		token := uuid.NewString()
		after := now.Add(2 * time.Minute)
		rr := r.db.WithContext(ctx).Model(&pd.PaymentEffect{}).Where("id = ? AND state IN ? AND (after IS NULL OR after <= ?)", row.ID, []string{"pending", "working"}, now).
			Updates(map[string]interface{}{"state": "working", "token": token, "after": after, "attempts": gorm.Expr("attempts + 1")})
		if rr.Error != nil {
			return nil, rr.Error
		}
		if rr.RowsAffected == 0 {
			continue
		}
		row.AutomationRevision = revision
		row.Token = token
		row.After = &after
		row.State = "working"
		row.Attempts++
		work = append(work, row)
	}
	return work, nil
}
func (r *Store) FinishPaymentEffect(ctx context.Context, w pd.PaymentEffect, state, code string, next time.Time) error {
	if state != "pending" && state != "done" && state != "manual" {
		return errors.New("invalid payment effect state")
	}
	fields := map[string]interface{}{"state": state, "code": code, "token": "", "after": nil}
	if state == "pending" {
		fields["after"] = next
	}
	if state == "done" {
		fields["completed_at"] = time.Now()
	}
	return r.db.WithContext(ctx).Model(&pd.PaymentEffect{}).Where("id = ? AND state = ? AND token = ?", w.ID, "working", w.Token).Updates(fields).Error
}

// AdmitPaymentEffect commits authorization before an outbound request. No SQL
// lock crosses the network. A pre-freeze admitted request may finish afterwards.
func (r *Store) AdmitPaymentEffect(ctx context.Context, w pd.PaymentEffect) error {
	if w.AutomationRevision == 0 || w.ID == 0 || w.Token == "" {
		return pd.ErrAutomationFrozen
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := automation.LockOpen(tx, w.AutomationRevision); err != nil {
			return err
		}
		now := time.Now()
		rr := tx.Model(&pd.PaymentEffect{}).Where("id = ? AND payment_id = ? AND order_id = ? AND kind = ? AND state = ? AND token = ? AND after > ?", w.ID, w.PaymentID, w.OrderID, w.Kind, "working", w.Token, now).
			Updates(map[string]interface{}{"dispatched_at": now, "code": "dispatch_admitted"})
		if rr.Error != nil {
			return rr.Error
		}
		if rr.RowsAffected != 1 {
			return errors.New("payment effect lease lost")
		}
		return nil
	})
}

// UsesFixedPaymentEffects also suppresses old notification queue payloads. The
// SQL record, not a queue flag, decides who owns payment/delivery notifications.
func (r *Store) UsesFixedPaymentEffects(orderID uint) (bool, error) {
	var root struct {
		ID       uint
		ParentID *uint
	}
	if err := r.db.Table("orders").Where("id = ?", orderID).Take(&root).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if root.ParentID != nil {
		orderID = *root.ParentID
	}
	var count int64
	err := r.db.Table("payments").Where("order_id = ? AND requires_chain_verification = ? AND status IN ?", orderID, true, []string{constants.PaymentStatusInitiated, constants.PaymentStatusPending, constants.PaymentStatusSuccess}).Count(&count).Error
	if err != nil || count != 0 {
		return count != 0, err
	}
	err = r.db.Model(&pd.EventConsumption{}).Where("order_id = ?", orderID).Count(&count).Error
	return count != 0, err
}
