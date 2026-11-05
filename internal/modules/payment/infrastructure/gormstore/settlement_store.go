package gormstore

import (
	"context"
	"errors"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epusdt"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"time"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberlevelstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	pc "github.com/dujiao-next/internal/modules/payment/contract"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *Store) ReserveAllocation(a pd.Allocation) error {
	if a.PaymentID == 0 || a.SlotKey == "" || a.Snapshot == "" {
		return pc.ErrAllocationConflict
	}
	// Read every unindexed fixed row, including expired/deleted ones. A provider
	// reference alone cannot distinguish an unbound request from corrupt history.
	// Keyset batches bound memory; JSON is validated in Go on both SQL dialects.
	var last uint
	for {
		var history []pd.Payment
		if err := r.db.Table("payments p").Select("p.*").
			Joins("LEFT JOIN payment_allocations a ON a.payment_id = p.id").
			Where("p.requires_chain_verification = ? AND a.id IS NULL AND p.id > ?", true, last).
			Order("p.id ASC").Limit(100).Find(&history).Error; err != nil {
			return err
		}
		for _, p := range history {
			if !epusdt.ValidUnboundHistory(p) {
				return pc.ErrAllocationConflict
			}
			last = p.ID
		}
		if len(history) < 100 {
			break
		}
	}
	if err := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&a).Error; err != nil {
		return err
	}
	var saved pd.Allocation
	if err := r.db.Where("slot_key = ?", a.SlotKey).First(&saved).Error; err != nil {
		return pc.ErrAllocationConflict
	}
	if saved.PaymentID != a.PaymentID || saved.OrderID != a.OrderID || saved.Snapshot != a.Snapshot ||
		saved.Network != a.Network || saved.Contract != a.Contract || saved.Recipient != a.Recipient || saved.RawAmount != a.RawAmount {
		return pc.ErrAllocationConflict
	}
	return nil
}

func (r *Store) OpenAutomationRevision(ctx context.Context) (uint64, error) {
	return automation.OpenRevision(r.db.WithContext(ctx))
}
func (tx transaction) LockAutomation(revision uint64) error {
	return automation.LockOpen(tx.db, revision)
}

func (tx transaction) Users() usercontract.Store { return userstore.New(tx.db) }

func (tx transaction) MemberLevelUsers() memberlevelcontract.UserRepository {
	return memberlevelstore.NewUserStore(tx.db)
}
func (tx transaction) Allocation(id uint) (*pd.Allocation, error) {
	var a pd.Allocation
	err := tx.db.Where("payment_id = ?", id).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &a, err
}
func (tx transaction) ConsumeEvent(c pd.EventConsumption) (bool, error) {
	r := tx.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&c)
	return r.RowsAffected == 1, r.Error
}
func (tx transaction) AddPaymentEffects(rows []pd.PaymentEffect) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (tx transaction) CompletePaymentEffect(w pd.PaymentEffect) (bool, error) {
	r := tx.db.Model(&pd.PaymentEffect{}).Where("id = ? AND payment_id = ? AND order_id = ? AND kind = ? AND state = ? AND token = ? AND after > ?", w.ID, w.PaymentID, w.OrderID, w.Kind, "working", w.Token, time.Now()).
		Updates(map[string]interface{}{"state": "done", "token": "", "after": nil, "completed_at": time.Now(), "code": "effect_completed"})
	return r.RowsAffected == 1, r.Error
}

func (r *Store) LeaseSettlements(ctx context.Context, now time.Time) ([]pc.SettlementWork, error) {
	revision, err := automation.OpenRevision(r.db.WithContext(ctx))
	if errors.Is(err, pd.ErrAutomationFrozen) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []pd.CandidateEvidence
	if err := r.db.WithContext(ctx).Where("check_state IN ? AND (check_after IS NULL OR check_after <= ?)", []string{"chain_checked", "settling"}, now).Order("id ASC").Limit(1).Find(&rows).Error; err != nil {
		return nil, err
	}
	var result []pc.SettlementWork
	for _, e := range rows {
		err := r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
			token := uuid.NewString()
			res := db.Model(&pd.CandidateEvidence{}).Where("id = ? AND check_state IN ? AND (check_after IS NULL OR check_after <= ?)", e.ID, []string{"chain_checked", "settling"}, now).
				Updates(map[string]interface{}{"check_state": "settling", "check_token": token, "check_after": now.Add(2 * time.Minute)})
			if res.Error != nil || res.RowsAffected != 1 {
				return res.Error
			}
			w := pc.SettlementWork{AutomationRevision: revision}
			if err := db.First(&w.Evidence, e.ID).Error; err != nil {
				return err
			}
			if err := db.First(&w.Receipt, e.ReceiptID).Error; err != nil {
				return err
			}
			if e.PaymentID != nil {
				err := db.First(&w.Payment, *e.PaymentID).Error
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			result = append(result, w)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (r *Store) WithSettlement(ctx context.Context, w pc.SettlementWork, fn func(pc.SettlementTransaction, *pd.Payment, *orderdomain.Order, *pd.CandidateEvidence, *pd.CandidateReceipt) error) error {
	return r.db.WithContext(ctx).Transaction(func(db *gorm.DB) error {
		if err := automation.LockOpen(db, w.AutomationRevision); err != nil {
			return err
		}
		var receipt pd.CandidateReceipt
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&receipt, w.Receipt.ID).Error; err != nil {
			return err
		}
		var e pd.CandidateEvidence
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&e, w.Evidence.ID).Error; err != nil {
			return err
		}
		if e.CheckState != "settling" || e.CheckToken != w.Evidence.CheckToken {
			return nil
		}
		tx := UseTransaction(db, r.guestCredentialSecret).(pc.SettlementTransaction)
		// Match create/cancel ordering: order (and children) before payment.
		var o *orderdomain.Order
		var p *pd.Payment
		if w.Payment.OrderID != 0 {
			var err error
			o, err = tx.Orders().GetByIDForUpdateWithChildren(w.Payment.OrderID)
			if err != nil {
				return err
			}
			if o != nil {
				// GORM preloads do not inherit FOR UPDATE; explicitly lock children.
				for i := range o.Children {
					child, err := tx.Orders().GetByIDForUpdateWithChildren(o.Children[i].ID)
					if err != nil {
						return err
					}
					if child == nil {
						return errors.New("settlement child disappeared")
					}
					o.Children[i] = *child
				}
			}
		}
		if w.Payment.ID != 0 {
			var err error
			p, err = tx.Payments().GetByIDForUpdate(w.Payment.ID)
			if err != nil {
				return err
			}
		}
		return fn(tx, p, o, &e, &receipt)
	})
}

func (r *Store) FinishSettlement(tx pc.SettlementTransaction, e *pd.CandidateEvidence, state, code string, next time.Time) error {
	t, ok := tx.(transaction)
	if !ok {
		return errors.New("invalid settlement transaction")
	}
	var after *time.Time
	if state == "chain_checked" || state == "retry" {
		after = &next
	}
	return t.db.Model(e).Updates(map[string]interface{}{"check_state": state, "check_code": code, "check_token": "", "check_after": after}).Error
}
