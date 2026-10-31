package gormstore

import (
	"context"
	"errors"
	nc "github.com/dujiao-next/internal/modules/notification/contract"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"gorm.io/gorm"
	"sort"
	"time"
)

func lockRecipientEffect(db *gorm.DB, w pd.PaymentEffect) (*pd.PaymentEffect, error) {
	if w.AutomationRevision == 0 || w.ID == 0 || w.Token == "" {
		return nil, pd.ErrAutomationFrozen
	}
	if err := automation.LockOpen(db, w.AutomationRevision); err != nil {
		return nil, err
	}
	var saved pd.PaymentEffect
	err := db.Where("id = ? AND payment_id = ? AND order_id = ? AND kind = ? AND state = ? AND token = ? AND after > ?", w.ID, w.PaymentID, w.OrderID, w.Kind, "working", w.Token, time.Now()).Take(&saved).Error
	if err != nil {
		return nil, err
	}
	return &saved, nil
}

func (r *Store) PrepareRecipients(ctx context.Context, w pd.PaymentEffect, keys []string) error {
	keys = append([]string(nil), keys...)
	sort.Strings(keys)
	if len(keys) > 100 {
		return pc.ErrEffectPermanent
	}
	for i, key := range keys {
		if len(key) != 64 || i > 0 && key == keys[i-1] {
			return pc.ErrEffectPermanent
		}
	}
	plan := nc.RecipientKey(keys...)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		saved, err := lockRecipientEffect(tx, w)
		if err != nil {
			return err
		}
		if saved.RecipientPlan != "" {
			if saved.RecipientPlan != plan {
				return pc.ErrEffectPermanent
			}
			var receipts []pd.RecipientReceipt
			if err := tx.Where("effect_id = ?", w.ID).Order("recipient_key ASC").Find(&receipts).Error; err != nil {
				return err
			}
			if len(receipts) != len(keys) {
				return pc.ErrEffectPermanent
			}
			for i, row := range receipts {
				if row.RecipientKey != keys[i] {
					return pc.ErrEffectPermanent
				}
			}
			return nil
		}
		// Freeze the complete destination set atomically before the first send.
		for _, key := range keys {
			if err := tx.Create(&pd.RecipientReceipt{EffectID: w.ID, RecipientKey: key, State: "pending"}).Error; err != nil {
				return err
			}
		}
		return tx.Model(saved).Update("recipient_plan", plan).Error
	})
}

func (r *Store) AdmitRecipient(ctx context.Context, w pd.PaymentEffect, key string) (bool, error) {
	send := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lockRecipientEffect(tx, w); err != nil {
			return err
		}
		var row pd.RecipientReceipt
		if err := tx.Where("effect_id = ? AND recipient_key = ?", w.ID, key).Take(&row).Error; err != nil {
			return err
		}
		if row.State == "done" {
			return nil
		}
		if row.State != "pending" && row.State != "sending" {
			return pc.ErrEffectPermanent
		}
		if row.State == "sending" && row.Token == w.Token {
			return errors.New("recipient already sending")
		}
		now := time.Now()
		if err := tx.Model(&row).Updates(map[string]interface{}{"state": "sending", "token": w.Token, "dispatched_at": now, "attempts": gorm.Expr("attempts + 1")}).Error; err != nil {
			return err
		}
		send = true
		return tx.Model(&pd.PaymentEffect{}).Where("id = ?", w.ID).Update("dispatched_at", now).Error
	})
	return send, err
}

func (r *Store) FinishRecipient(ctx context.Context, w pd.PaymentEffect, key string, success bool) error {
	state := "pending"
	var completed *time.Time
	if success {
		state = "done"
		now := time.Now()
		completed = &now
	}
	// Receipt ACK remains allowed while frozen: it records prior IO, not new IO.
	rr := r.db.WithContext(ctx).Model(&pd.RecipientReceipt{}).Where("effect_id = ? AND recipient_key = ? AND state = ? AND token = ?", w.ID, key, "sending", w.Token).
		Updates(map[string]interface{}{"state": state, "token": "", "completed_at": completed})
	if rr.Error != nil {
		return rr.Error
	}
	if rr.RowsAffected != 1 {
		return errors.New("recipient receipt lease lost")
	}
	return nil
}
