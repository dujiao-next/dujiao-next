package consumer

import (
	"context"
	"testing"
	"time"

	resellerapplication "github.com/dujiao-next/internal/modules/reseller/application"
	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/queue"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/hibiken/asynq"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func TestResellerConfirmLedgerWorkerMarksDueEntriesAvailable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:reseller_worker?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&resellerdomain.LedgerEntry{}, &resellerdomain.BalanceAccount{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	past := time.Now().Add(-time.Minute)
	row := resellerdomain.LedgerEntry{
		ResellerID:     1,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.NewFromInt(10)),
		Currency:       "USD",
		IdempotencyKey: "order_profit:worker",
		Status:         resellerdomain.LedgerStatusPendingConfirm,
		AvailableAt:    &past,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed ledger failed: %v", err)
	}
	repo := resellergormstore.New(db)
	c := New(&container.Container{
		ResellerAccountingLedger: resellerapplication.NewAccountingLedgerService(repo, 0, 0),
	})
	if err := c.handleResellerConfirmLedger(context.Background(), queue.NewResellerConfirmLedgerTask()); err != nil {
		t.Fatalf("worker handler failed: %v", err)
	}
	var got resellerdomain.LedgerEntry
	if err := db.First(&got, row.ID).Error; err != nil {
		t.Fatalf("load ledger failed: %v", err)
	}
	if got.Status != resellerdomain.LedgerStatusAvailable {
		t.Fatalf("expected available, got %s", got.Status)
	}
}

func TestResellerConfirmLedgerTaskType(t *testing.T) {
	task := queue.NewResellerConfirmLedgerTask()
	if task == nil {
		t.Fatal("expected task")
	}
	if task.Type() != queue.TaskResellerConfirmLedger {
		t.Fatalf("unexpected task type %s", task.Type())
	}
}

func TestResellerConfirmLedgerWorkerSkipNilService(t *testing.T) {
	c := New(&container.Container{})
	if err := c.handleResellerConfirmLedger(context.Background(), asynq.NewTask(queue.TaskResellerConfirmLedger, nil)); err != nil {
		t.Fatalf("nil service should be skipped, got %v", err)
	}
}
