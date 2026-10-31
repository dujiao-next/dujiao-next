package automation_test

import (
	"context"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"path/filepath"
	"testing"
)

func TestOperatorAuditIsAtomicWithFreezeAndCannotOverridePayment(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "operator.db")), &gorm.Config{})
	require(t, err)
	raw, _ := db.DB()
	defer raw.Close()
	require(t, db.AutoMigrate(&pd.AutomationControl{}, &pd.OperatorAudit{}))
	require(t, automation.Ensure(db))
	require(t, automation.Operate(context.Background(), db, "resume", "local-test", "acceptance", 1, 0))
	var control pd.AutomationControl
	require(t, db.First(&control, 1).Error)
	if control.Frozen || control.Revision != 2 {
		t.Fatal("resume failed")
	}
	require(t, db.Exec(`CREATE TRIGGER reject_audit BEFORE INSERT ON payment_operator_audit BEGIN SELECT RAISE(ABORT, 'audit failure'); END`).Error)
	if automation.Operate(context.Background(), db, "freeze", "local-test", "incident", 2, 0) == nil {
		t.Fatal("audit failure ignored")
	}
	require(t, db.First(&control, 1).Error)
	if control.Frozen || control.Revision != 2 {
		t.Fatal("unaudited freeze committed")
	}
	require(t, db.Exec(`DROP TRIGGER reject_audit`).Error)
	require(t, automation.Operate(context.Background(), db, "freeze", "local-test", "incident", 2, 0))
	if automation.Operate(context.Background(), db, "resume", "local-test", "stale", 2, 0) == nil {
		t.Fatal("stale revision accepted")
	}
	if automation.Operate(context.Background(), db, "mark-paid", "local-test", "override", 3, 1) == nil {
		t.Fatal("manual paid override exposed")
	}
	var count int64
	require(t, db.Model(&pd.OperatorAudit{}).Count(&count).Error)
	if count != 2 {
		t.Fatal("unexpected audit history", count)
	}
}
