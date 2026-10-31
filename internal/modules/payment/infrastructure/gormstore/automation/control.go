// Package automation owns the SQL fence shared by settlement and fulfillment.
// LockOpen must run in the caller's transaction, before order/payment locks.
package automation

import (
	"context"
	"errors"
	"strings"
	"time"

	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Ensure creates a CLOSED control row only. Restart/migration never reopens it.
func Ensure(db *gorm.DB) error {
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&pd.AutomationControl{
		ID: 1, Frozen: true, Revision: 1, Reason: "initial_acceptance_pending", UpdatedAt: time.Now(),
	}).Error
}

func OpenRevision(db *gorm.DB) (uint64, error) {
	var row pd.AutomationControl
	if err := db.First(&row, 1).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, pd.ErrAutomationFrozen
		}
		return 0, err
	}
	if row.Frozen || row.Revision == 0 {
		return 0, pd.ErrAutomationFrozen
	}
	return row.Revision, nil
}

// LockOpen uses a conditional write, not just FOR UPDATE (ignored by SQLite).
// The lock is retained until the surrounding SQL transaction commits/rolls back.
// revision=0 admits the current generation, otherwise stale work is rejected.
func LockOpen(db *gorm.DB, revision uint64) error {
	q := db.Model(&pd.AutomationControl{}).Where("id = 1 AND frozen = ? AND revision > 0", false)
	if revision != 0 {
		q = q.Where("revision = ?", revision)
	}
	r := q.UpdateColumn("revision", gorm.Expr("revision"))
	if r.Error != nil {
		return r.Error
	}
	if r.RowsAffected != 1 {
		return pd.ErrAutomationFrozen
	}
	return nil
}

// SetFrozen is an operator-only port; no public/merchant callback route calls it.
// A known revision and reason are required, preventing a stale resume command
// from undoing a more recent freeze. Return success only after the fence commits.
func SetFrozen(ctx context.Context, db *gorm.DB, frozen bool, expectedRevision uint64, reason string) error {
	if expectedRevision == 0 || strings.TrimSpace(reason) == "" || len(reason) > 160 {
		return errors.New("invalid payment automation change")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := tx.Model(&pd.AutomationControl{}).Where("id = 1 AND revision = ?", expectedRevision).
			Updates(map[string]interface{}{"frozen": frozen, "revision": gorm.Expr("revision + 1"), "reason": reason, "updated_at": time.Now()})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return errors.New("payment automation revision changed or control missing")
		}
		return nil
	})
}
