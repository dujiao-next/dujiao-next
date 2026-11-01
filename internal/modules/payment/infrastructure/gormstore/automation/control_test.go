package automation_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func database(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	return db
}
func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestControlDefaultsClosedAndCannotBeReopenedByStaleOperator(t *testing.T) {
	db := database(t, filepath.Join(t.TempDir(), "gate.db"))
	require(t, db.AutoMigrate(&pd.AutomationControl{}))
	if _, err := automation.OpenRevision(db); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatal("missing control admitted work")
	}
	require(t, automation.Ensure(db))
	if _, err := automation.OpenRevision(db); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatal("new migration opened payments")
	}
	require(t, automation.SetFrozen(context.Background(), db, false, 1, "synthetic acceptance"))
	require(t, automation.Ensure(db))
	if rev, err := automation.OpenRevision(db); err != nil || rev != 2 {
		t.Fatalf("migration reset operator state: %d %v", rev, err)
	}
	require(t, automation.SetFrozen(context.Background(), db, true, 2, "synthetic incident"))
	if err := automation.SetFrozen(context.Background(), db, false, 2, "stale review"); err == nil {
		t.Fatal("stale resume overwrote newer freeze")
	}
	require(t, automation.Ensure(db))
	if err := db.Transaction(func(tx *gorm.DB) error { return automation.LockOpen(tx, 0) }); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatal("restart reopened control")
	}
}

func TestFreezeWaitsForAdmittedSQLTransaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gate.db")
	writer := database(t, path)
	operator := database(t, path) // distinct connections, as with API and worker processes
	require(t, writer.AutoMigrate(&pd.AutomationControl{}))
	require(t, automation.Ensure(writer))
	require(t, automation.SetFrozen(context.Background(), writer, false, 1, "synthetic acceptance"))
	tx := writer.Begin()
	require(t, tx.Error)
	defer tx.Rollback()
	require(t, automation.LockOpen(tx, 2))
	started := make(chan struct{})
	done := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	go func() { close(started); done <- automation.SetFrozen(ctx, operator, true, 2, "synthetic incident") }()
	<-started
	select {
	case err := <-done:
		t.Fatalf("freeze returned before admitted commit: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require(t, tx.Commit().Error)
	require(t, <-done)
	if err := writer.Transaction(func(tx *gorm.DB) error { return automation.LockOpen(tx, 2) }); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatal("work committed after freeze")
	}
	require(t, automation.SetFrozen(context.Background(), operator, false, 3, "synthetic review completed"))
	if err := writer.Transaction(func(tx *gorm.DB) error { return automation.LockOpen(tx, 2) }); !errors.Is(err, pd.ErrAutomationFrozen) {
		t.Fatal("stale proof admitted after resume")
	}
	require(t, writer.Transaction(func(tx *gorm.DB) error { return automation.LockOpen(tx, 4) }))
}
