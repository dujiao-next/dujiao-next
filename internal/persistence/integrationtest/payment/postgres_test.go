package settlement_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/bootstrap/database/migrations"
	"github.com/dujiao-next/internal/constants"
	pd "github.com/dujiao-next/internal/modules/payment/domain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gormstore/automation"
	"github.com/dujiao-next/internal/platform/database/gormdb"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Each invocation owns a new schema. The URL must explicitly name a disposable
// loopback database; this test never drops public or an existing schema.
func paymentPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	raw := os.Getenv("PAYMENT_TEST_POSTGRES_URL")
	if raw == "" {
		t.Skip("PAYMENT_TEST_POSTGRES_URL is unset")
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.Path != "/payment_lab" {
		t.Fatal("requires disposable loopback payment_lab database")
	}
	admin, err := gorm.Open(postgres.Open(raw), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	must(t, err)
	schema := fmt.Sprintf("payment_test_%d", time.Now().UnixNano())
	must(t, admin.Exec("CREATE SCHEMA "+schema).Error)
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	must(t, err)
	sqlDB, err := db.DB()
	must(t, err)
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() {
		_ = sqlDB.Close()
		_ = admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		a, _ := admin.DB()
		_ = a.Close()
	})
	previous := gormdb.DB
	gormdb.DB = db
	err = migrations.AutoMigrate()
	gormdb.DB = previous
	must(t, err)
	var control pd.AutomationControl
	must(t, db.First(&control, 1).Error)
	if !control.Frozen || control.Revision != 1 {
		t.Fatal("new production migration did not default to frozen")
	}
	must(t, automation.Operate(context.Background(), db, "resume", "synthetic-test", "isolated acceptance", 1, 0))
	// Running the real registry again must preserve operator state and history.
	gormdb.DB = db
	err = migrations.AutoMigrate()
	gormdb.DB = previous
	must(t, err)
	must(t, db.First(&control, 1).Error)
	if control.Frozen || control.Revision != 2 || count(t, db, &pd.OperatorAudit{}) != 1 {
		t.Fatal("repeat migration changed the gate")
	}
	return db
}

func TestPostgresPaymentSettlementReplayAndFreeze(t *testing.T) {
	db := paymentPostgres(t)
	p, _ := seed(t, db, 1, 1)
	hash := "0x" + strings.Repeat("a", 64)
	evidence(t, db, p, "1", hash)
	evidence(t, db, p, "2", hash)
	a, _, repo := services(db, &verifier{})
	b, _, _ := services(db, &verifier{})
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); errs <- a.Run(context.Background()) }()
	go func() { defer wg.Done(); errs <- b.Run(context.Background()) }()
	wg.Wait()
	close(errs)
	for err := range errs {
		must(t, err)
	}
	must(t, a.Run(context.Background()))
	var payment pd.Payment
	must(t, db.First(&payment, p.ID).Error)
	if payment.Status != constants.PaymentStatusSuccess || count(t, db, &pd.EventConsumption{}) != 1 {
		var evidence []pd.CandidateEvidence
		must(t, db.Find(&evidence).Error)
		for _, e := range evidence {
			t.Logf("evidence %d state=%s reason=%s payment=%v", e.ID, e.CheckState, e.CheckCode, e.PaymentID)
		}
		t.Fatal("concurrent PostgreSQL workers lost or duplicated settlement")
	}
	p2, _ := seed(t, db, 2, 2)
	evidence(t, db, p2, "3", hash)
	must(t, a.Run(context.Background()))
	must(t, db.First(&payment, p.ID).Error)
	var second pd.Payment
	must(t, db.First(&second, p2.ID).Error)
	if second.Status == constants.PaymentStatusSuccess || count(t, db, &pd.EventConsumption{}) != 1 {
		t.Fatal("cross-order event replay")
	}
	source := pd.CandidateSource{GatewayURL: "https://gateway.example", PID: "synthetic"}
	must(t, repo.SaveCandidateSubscription(context.Background(), source, "epusdt", jsonmap.JSON{"secret_key": "synthetic-old"}))
	must(t, repo.SaveCandidateSubscription(context.Background(), source, "epusdt", jsonmap.JSON{"secret_key": "synthetic-rotated"}))
	rows, err := repo.ListCandidateSubscriptions(context.Background(), 1, 10)
	must(t, err)
	if len(rows) != 1 || rows[0].ConfigJSON["secret_key"] != "synthetic-rotated" {
		t.Fatal("JSON subscription upsert failed")
	}
	must(t, automation.Operate(context.Background(), db, "freeze", "synthetic-test", "freeze PostgreSQL", 2, 0))
	if err := automation.Operate(context.Background(), db, "resume", "synthetic-test", "stale resume", 2, 0); err == nil {
		t.Fatal("stale revision resumed")
	}
	if work, err := repo.LeaseSettlements(context.Background(), time.Now()); err != nil || len(work) != 0 {
		t.Fatal("frozen database leased settlement", err)
	}
}

func TestPostgresPaymentAtomicRollbackAndGateLock(t *testing.T) {
	db := paymentPostgres(t)
	p, _ := seed(t, db, 1, 1)
	e := evidence(t, db, p, "1", "hash")
	must(t, db.Exec(`CREATE FUNCTION reject_effect() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'synthetic write failure'; END $$`).Error)
	must(t, db.Exec(`CREATE TRIGGER reject_effect BEFORE INSERT ON payment_effects FOR EACH ROW EXECUTE FUNCTION reject_effect()`).Error)
	svc, _, _ := services(db, &verifier{})
	if svc.Run(context.Background()) == nil {
		var evidence pd.CandidateEvidence
		must(t, db.First(&evidence, e.ID).Error)
		t.Logf("state=%s reason=%s", evidence.CheckState, evidence.CheckCode)
		t.Fatal("outbox failure swallowed")
	}
	assertUnpaid(t, db, p.ID)
	must(t, db.Exec("DROP TRIGGER reject_effect ON payment_effects").Error)
	must(t, db.Model(&e).Update("check_after", time.Now().Add(-time.Second)).Error)
	must(t, svc.Run(context.Background()))
	if count(t, db, &pd.EventConsumption{}) != 1 {
		t.Fatal("retry after PostgreSQL rollback failed")
	}
	// A freeze must wait for a transaction already admitted by the same SQL row.
	tx := db.Begin()
	must(t, tx.Error)
	must(t, tx.Exec("UPDATE payment_automation_control SET revision = revision WHERE id = 1").Error)
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		done <- automation.Operate(context.Background(), db, "freeze", "synthetic-test", "lock ordering", 2, 0)
	}()
	<-started
	select {
	case err := <-done:
		_ = tx.Rollback()
		t.Fatal("freeze bypassed held SQL lock", err)
	case <-time.After(100 * time.Millisecond):
	}
	must(t, tx.Commit().Error)
	select {
	case err := <-done:
		must(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("freeze did not complete after commit")
	}
}
