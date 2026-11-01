package securityalerts_test

import (
	"context"
	"errors"
	app "github.com/dujiao-next/internal/modules/payment/application/securityalerts"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	store "github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/securityalerts"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
	"time"
)

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func open(t *testing.T, path string) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	must(t, err)
	conn, err := db.DB()
	must(t, err)
	conn.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = conn.Close() })
	must(t, db.AutoMigrate(&pd.SecurityAlert{}, &pd.CandidateSubscription{}, &pd.AutomationControl{}, &pd.CandidateReceipt{}, &pd.CandidateEvidence{}, &pd.Payment{}, &pd.PaymentEffect{}))
	return db
}

type sender struct {
	rows []pd.SecurityAlert
	fail bool
}

func (s *sender) Send(_ context.Context, row pd.SecurityAlert) error {
	s.rows = append(s.rows, row)
	if s.fail {
		return errors.New("synthetic private upstream error")
	}
	return nil
}

func TestSecurityAlertsPersistRetryCooldownRecoveryAndIgnorePaymentFreeze(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "alerts.db")
	db := open(t, path)
	must(t, automation.Ensure(db))
	must(t, automation.SetFrozen(ctx, db, false, 1, "synthetic acceptance"))
	effect := pd.PaymentEffect{PaymentID: 1, OrderID: 1, Kind: "paid_notification", State: "manual"}
	must(t, db.Create(&effect).Error)
	sink := &sender{fail: true}
	svc := &app.Service{Store: store.New(db), Sender: sink}
	if err := svc.Run(ctx); err == nil || err.Error() != "payment security alert send failed" {
		t.Fatal("failure leaked or disappeared", err)
	}
	var row pd.SecurityAlert
	must(t, db.Where("kind = ?", "effects_manual").Take(&row).Error)
	if row.SentAt != nil || !row.Pending || !row.NextAt.After(time.Now()) {
		t.Fatal("failed alert incorrectly acknowledged")
	}
	conn, _ := db.DB()
	must(t, conn.Close())
	db = open(t, path)
	svc.Store = store.New(db)
	sink.fail = false
	must(t, db.Model(&pd.SecurityAlert{}).Where("kind = ?", "effects_manual").Update("next_at", time.Now().Add(-time.Minute)).Error)
	must(t, svc.Run(ctx))
	must(t, svc.Run(ctx))
	if len(sink.rows) != 2 {
		t.Fatal("restart retry/cooldown failed")
	}
	must(t, automation.SetFrozen(ctx, db, true, 2, "synthetic incident"))
	must(t, svc.Run(ctx))
	if len(sink.rows) != 3 || sink.rows[2].Kind != "automation_frozen" {
		t.Fatal("payment freeze also silenced alert")
	}
	must(t, db.Model(&effect).Update("state", "done").Error)
	must(t, svc.Run(ctx))
	must(t, svc.Run(ctx))
	if len(sink.rows) != 4 || sink.rows[3].Kind != "effects_manual" || sink.rows[3].Active || sink.rows[3].Count != 0 {
		t.Fatal("recovery not independently acknowledged")
	}
	// A broken observation must not turn existing conditions into false recovery.
	must(t, db.Migrator().DropTable(&pd.CandidateEvidence{}))
	if err := svc.Run(ctx); err == nil {
		t.Fatal("broken observation hidden")
	}
	if len(sink.rows) != 4 {
		t.Fatal("sent after failed snapshot")
	}
}

func TestAlertLeaseAndObservationVersionRejectStaleACK(t *testing.T) {
	ctx := context.Background()
	db := open(t, filepath.Join(t.TempDir(), "lease.db"))
	repo := store.New(db)
	now := time.Now()
	must(t, automation.Ensure(db))
	must(t, repo.ObserveSecurity(ctx, now))
	old, err := repo.LeaseSecurityAlerts(ctx, now)
	must(t, err)
	if len(old) != 1 {
		t.Fatal("missing frozen alert")
	}
	other, err := repo.LeaseSecurityAlerts(ctx, now)
	must(t, err)
	if len(other) != 0 {
		t.Fatal("double alert lease")
	}
	must(t, automation.SetFrozen(ctx, db, false, 1, "synthetic thaw"))
	must(t, repo.ObserveSecurity(ctx, now))
	if err := repo.FinishSecurityAlert(ctx, old[0], true, now); err == nil {
		t.Fatal("old active ACK swallowed recovery")
	}
	current, err := repo.LeaseSecurityAlerts(ctx, now.Add(2*time.Minute))
	must(t, err)
	if len(current) != 1 || current[0].Active || current[0].Version == old[0].Version {
		t.Fatal("new recovery not retained")
	}
	must(t, repo.FinishSecurityAlert(ctx, current[0], true, now.Add(2*time.Minute)))
}

func TestObservationsAggregateOnlyKnownLedgerStates(t *testing.T) {
	ctx := context.Background()
	db := open(t, filepath.Join(t.TempDir(), "counts.db"))
	repo := store.New(db)
	now := time.Now()
	old := now.Add(-10 * time.Minute)
	must(t, automation.Ensure(db))
	must(t, automation.SetFrozen(ctx, db, false, 1, "synthetic acceptance"))
	must(t, db.Create(&pd.CandidateReceipt{ID: 1, SourceKey: "synthetic", CandidateID: "1", FirstDigest: "x", Conflicted: true}).Error)
	must(t, db.Create(&pd.CandidateEvidence{ReceiptID: 1, Digest: "a", CheckState: "retry", CheckCode: "rpc_disagreement", ReceivedAt: old, Payload: "private attacker content"}).Error)
	must(t, db.Create(&pd.Payment{OrderID: 1, RequiresChainVerification: true, Status: "initiated", CreatedAt: old}).Error)
	must(t, db.Create(&pd.PaymentEffect{PaymentID: 1, OrderID: 1, Kind: "auto_fulfill", State: "pending", CreatedAt: old}).Error)
	must(t, repo.ObserveSecurity(ctx, now))
	for _, kind := range []string{"candidate_conflicts", "rpc_retry", "intent_stalled", "effects_stalled"} {
		var row pd.SecurityAlert
		must(t, db.Where("kind = ?", kind).Take(&row).Error)
		if row.Count != 1 || !row.Active {
			t.Fatalf("missing condition %s", kind)
		}
	}
}
