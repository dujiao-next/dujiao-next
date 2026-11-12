package integrationtest

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	walletapp "github.com/dujiao-next/internal/modules/wallet/application"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
	walletgormstore "github.com/dujiao-next/internal/modules/wallet/infrastructure/gormstore"
)

// rechargeRefundEmailQueueStub 记录退款邮件任务并验证入队时事务已提交。
type rechargeRefundEmailQueueStub struct {
	calls     int
	statuses  []string
	refundIDs []uint
	check     func(uint, uint)
	err       error
}

// EnqueueRechargeRefundEmail 捕获退款邮件并返回指定的队列错误。
func (q *rechargeRefundEmailQueueStub) EnqueueRechargeRefundEmail(rechargeID, refundID uint, status string) error {
	q.calls++
	q.statuses = append(q.statuses, status)
	q.refundIDs = append(q.refundIDs, refundID)
	if q.check != nil {
		q.check(rechargeID, refundID)
	}
	return q.err
}

func TestRechargeRefundEmailsOnlyAfterCommitAndOncePerRefund(t *testing.T) {
	_, db, recharge := setupRechargeRefund(t)
	repo := walletgormstore.New(db)
	emails := &rechargeRefundEmailQueueStub{check: func(rechargeID, refundID uint) {
		var record walletdomain.Transaction
		if err := db.First(&record, refundID).Error; err != nil {
			t.Fatal(err)
		}
		if record.RechargeOrderID == nil || *record.RechargeOrderID != rechargeID {
			t.Fatalf("wrong refund: %+v", record)
		}
	}}
	svc := walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo, RefundEmails: emails})
	for i, amount := range []int64{40, 60} {
		input := walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(amount)), RequestID: fmt.Sprintf("refund-email-request-%04d", i)}
		for retry := 0; retry < 2; retry++ {
			if err := svc.RefundRecharge(input); err != nil {
				t.Fatal(err)
			}
		}
	}
	if emails.calls != 2 || emails.statuses[0] != constants.WalletRechargeStatusPartiallyRefunded || emails.statuses[1] != constants.WalletRechargeStatusRefunded || emails.refundIDs[0] == emails.refundIDs[1] {
		t.Fatalf("emails: %+v", emails)
	}
	if err := svc.RefundRecharge(walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(1)), RequestID: "refund-email-exceeded"}); !errors.Is(err, walletcontract.ErrRefundExceeded) {
		t.Fatal(err)
	}
	if emails.calls != 2 {
		t.Fatal("rejected refund sent email")
	}
}

func TestRechargeRefundEmailQueueFailureDoesNotFailRefund(t *testing.T) {
	_, db, recharge := setupRechargeRefund(t)
	repo := walletgormstore.New(db)
	emails := &rechargeRefundEmailQueueStub{err: errors.New("queue unavailable")}
	svc := walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo, RefundEmails: emails})
	if err := svc.RefundRecharge(walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(20)), RequestID: "refund-email-queue-error"}); err != nil {
		t.Fatal(err)
	}
	account, err := svc.GetAccount(recharge.UserID)
	if err != nil || account.Balance.String() != "80.00" || emails.calls != 1 {
		t.Fatalf("refund state: %+v error=%v emails=%d", account, err, emails.calls)
	}
}

func TestWalletTransactionsDoNotRequestASecondConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("skip in short mode")
	}
	dsn := fmt.Sprintf("file:wallet_concurrency_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &walletdomain.Account{}, &walletdomain.Transaction{}); err != nil {
		t.Fatalf("migrate wallet schema: %v", err)
	}
	store := walletgormstore.New(db)
	wallets := walletapp.NewService(walletapp.Options{Repository: store, Transactions: store})

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get raw db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	const workers = 8
	var wait sync.WaitGroup
	errorsCh := make(chan error, workers)
	done := make(chan struct{})
	for userID := 1; userID <= workers; userID++ {
		wait.Add(1)
		go func(id uint) {
			defer wait.Done()
			_, _, rechargeErr := wallets.Recharge(walletcontract.RechargeInput{
				UserID:   id,
				Amount:   money.FromDecimal(decimal.NewFromInt(100)),
				Currency: "CNY",
				Remark:   fmt.Sprintf("concurrency_%d", id),
			})
			if rechargeErr != nil {
				errorsCh <- fmt.Errorf("user %d: %w", id, rechargeErr)
			}
		}(uint(userID))
	}
	go func() {
		wait.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("wallet transactions deadlocked with MaxOpenConns=1")
	}
	close(errorsCh)
	for err := range errorsCh {
		t.Errorf("recharge failed: %v", err)
	}
	for userID := 1; userID <= workers; userID++ {
		account, err := wallets.GetAccount(uint(userID))
		if err != nil {
			t.Fatalf("get account %d: %v", userID, err)
		}
		if account == nil || !account.Balance.Decimal.Equal(decimal.NewFromInt(100)) {
			t.Fatalf("unexpected balance for user %d: %+v", userID, account)
		}
	}
}

func setupRechargeRefund(t *testing.T) (*walletapp.Service, *gorm.DB, walletdomain.RechargeOrder) {
	t.Helper()
	repo, db := setupWalletRepositoryTest(t)
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	recharge := walletdomain.RechargeOrder{RechargeNo: "refund-test", UserID: 1, PaymentID: 1, Amount: money.FromDecimal(decimal.NewFromInt(100)), PayableAmount: money.FromDecimal(decimal.NewFromInt(103)), Currency: "CNY", Status: constants.WalletRechargeStatusSuccess}
	if err := db.Create(&recharge).Error; err != nil {
		t.Fatal(err)
	}
	account := walletdomain.Account{UserID: 1, Balance: money.FromDecimal(decimal.NewFromInt(100))}
	if err := db.Create(&account).Error; err != nil {
		t.Fatal(err)
	}
	return walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo}), db, recharge
}

func TestRechargeRefundUsesOrderRefundWindowWithoutASecondConnection(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		days, createdAgo, paidAgo       int
		missingPaid, partial, readError bool
		expired                         bool
	}{
		{name: "paid_expired", days: 1, createdAgo: 3, paidAgo: 2, expired: true},
		{name: "paid_takes_priority", days: 1, createdAgo: 90, paidAgo: 0},
		{name: "created_fallback", days: 1, createdAgo: 2, missingPaid: true, expired: true},
		{name: "unlimited", days: 0, createdAgo: 400, paidAgo: 400},
		{name: "custom_window", days: 3, createdAgo: 4, paidAgo: 2},
		{name: "partial_expired", days: 1, createdAgo: 3, paidAgo: 2, partial: true, expired: true},
		{name: "config_failure", days: 0, readError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, db, recharge := setupRechargeRefund(t)
			if err := db.AutoMigrate(&settingsstore.SettingRecord{}); err != nil {
				t.Fatal(err)
			}
			settings := settingsstore.New(db)
			if _, err := settings.Upsert(constants.SettingKeyOrderConfig, jsonmap.JSON{"max_refund_days": tc.days}); err != nil {
				t.Fatal(err)
			}
			paid := time.Now().AddDate(0, 0, -tc.paidAgo)
			recharge.CreatedAt = time.Now().AddDate(0, 0, -tc.createdAgo)
			recharge.PaidAt = &paid
			if tc.missingPaid {
				recharge.PaidAt = nil
			}
			if tc.partial {
				recharge.Status = constants.WalletRechargeStatusPartiallyRefunded
				recharge.RefundedAmount = money.FromDecimal(decimal.NewFromInt(20))
			}
			if err := db.Save(&recharge).Error; err != nil {
				t.Fatal(err)
			}
			if tc.readError {
				if err := db.Migrator().DropTable(&settingsstore.SettingRecord{}); err != nil {
					t.Fatal(err)
				}
			}
			repo := walletgormstore.New(db)
			svc := walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo, RefundConfig: settingsapp.NewService(settings)})
			expired, queryErr := svc.IsRechargeRefundWindowExpired(&recharge)
			if tc.readError {
				if queryErr == nil {
					t.Fatal("configuration failure was ignored")
				}
			} else if queryErr != nil || expired != tc.expired {
				t.Fatalf("refund window: expired=%v error=%v", expired, queryErr)
			}
			err := svc.RefundRecharge(walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(10)), RequestID: "refund-window-request-0001"})
			if tc.expired && !errors.Is(err, walletcontract.ErrRechargeRefundExpired) {
				t.Fatalf("expected expired refund rejection: %v", err)
			}
			if tc.readError && err == nil {
				t.Fatal("refund accepted after configuration failure")
			}
			if !tc.expired && !tc.readError && err != nil {
				t.Fatal(err)
			}
			account, accountErr := svc.GetAccount(recharge.UserID)
			if accountErr != nil {
				t.Fatal(accountErr)
			}
			var current walletdomain.RechargeOrder
			if err := db.First(&current, recharge.ID).Error; err != nil {
				t.Fatal(err)
			}
			rows, listErr := svc.ListRechargeRefunds(recharge.ID)
			if listErr != nil {
				t.Fatal(listErr)
			}
			if tc.expired || tc.readError {
				if account.Balance.String() != "100.00" || len(rows) != 0 || current.Status != recharge.Status || !current.RefundedAmount.Decimal.Equal(recharge.RefundedAmount.Decimal) {
					t.Fatalf("rejected refund changed state: %+v %+v %+v", account, current, rows)
				}
			} else if account.Balance.String() != "90.00" || len(rows) != 1 {
				t.Fatalf("accepted refund state: %+v %+v", account, rows)
			}
		})
	}
}

func TestRechargeRefundRetryAfterWindowExpiresDoesNotDebitAgain(t *testing.T) {
	svc, db, recharge := setupRechargeRefund(t)
	input := walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(40)), RequestID: "refund-window-retry-0001"}
	if err := svc.RefundRecharge(input); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&recharge).Update("paid_at", time.Now().AddDate(0, 0, -31)).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.RefundRecharge(input); err != nil {
		t.Fatalf("completed refund retry: %v", err)
	}
	input.RequestID = "refund-window-retry-0002"
	if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrRechargeRefundExpired) {
		t.Fatalf("new refund after expiry: %v", err)
	}
	account, err := svc.GetAccount(recharge.UserID)
	if err != nil || account.Balance.String() != "60.00" {
		t.Fatalf("retry changed balance: %+v error=%v", account, err)
	}
}

func TestRechargeRefundValidationAndIdempotency(t *testing.T) {
	svc, db, recharge := setupRechargeRefund(t)
	input := walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(40)), RequestID: "refund-request-0001", Remark: "offline"}
	for i := 0; i < 2; i++ {
		if err := svc.RefundRecharge(input); err != nil {
			t.Fatal(err)
		}
	}
	assertState := func(balance, refunded int64, count int) {
		t.Helper()
		var account walletdomain.Account
		if err := db.Where("user_id = ?", recharge.UserID).First(&account).Error; err != nil {
			t.Fatal(err)
		}
		if !account.Balance.Decimal.Equal(decimal.NewFromInt(balance)) {
			t.Fatalf("balance: %s", account.Balance)
		}
		var current walletdomain.RechargeOrder
		if err := db.First(&current, recharge.ID).Error; err != nil {
			t.Fatal(err)
		}
		if !current.RefundedAmount.Decimal.Equal(decimal.NewFromInt(refunded)) {
			t.Fatalf("refunded: %s", current.RefundedAmount)
		}
		rows, err := svc.ListRechargeRefunds(recharge.ID)
		if err != nil || len(rows) != count {
			t.Fatalf("refund records: %v, %v", rows, err)
		}
	}
	assertState(60, 40, 1)
	input.Amount = money.FromDecimal(decimal.NewFromInt(41))
	if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrInvalidAmount) {
		t.Fatalf("changed retry: %v", err)
	}
	input.RequestID = "refund-request-0002"
	input.Amount = money.FromDecimal(decimal.NewFromInt(61))
	if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrRefundExceeded) {
		t.Fatalf("excess: %v", err)
	}
	assertState(60, 40, 1)
	if err := db.Model(&walletdomain.Account{}).Where("user_id = 1").Update("balance", 10).Error; err != nil {
		t.Fatal(err)
	}
	input.Amount = money.FromDecimal(decimal.NewFromInt(20))
	if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrInsufficientBalance) {
		t.Fatalf("insufficient: %v", err)
	}
	assertState(10, 40, 1)
	input.Amount = money.Amount{Decimal: decimal.RequireFromString("0.001")}
	if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrInvalidAmount) {
		t.Fatalf("precision: %v", err)
	}
	input.Amount = money.FromDecimal(decimal.NewFromInt(1))
	if err := db.Model(&walletdomain.RechargeOrder{}).Where("id = ?", recharge.ID).Update("status", constants.WalletRechargeStatusExpired).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrRechargeStatusInvalid) {
		t.Fatalf("unpaid: %v", err)
	}
	assertState(10, 40, 1)
}

func TestRechargeRefundConcurrentRequestsCannotOverdraw(t *testing.T) {
	svc, db, recharge := setupRechargeRefund(t)
	var wg sync.WaitGroup
	var successes atomic.Int32
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := svc.RefundRecharge(walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(20)), RequestID: fmt.Sprintf("refund-request-%04d", i), PaymentFeeRefunded: true, FeeSnapshot: walletcontract.RechargeRefundFeeSnapshot{PaymentAmount: money.FromDecimal(decimal.NewFromInt(100)), PaymentFee: money.FromDecimal(decimal.RequireFromString("3.01"))}})
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, walletcontract.ErrRefundExceeded) {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if successes.Load() != 5 {
		t.Fatalf("successful refunds: %d", successes.Load())
	}
	var account walletdomain.Account
	if err := db.Where("user_id = 1").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if !account.Balance.Decimal.IsZero() {
		t.Fatalf("balance: %s", account.Balance)
	}
	var current walletdomain.RechargeOrder
	if err := db.First(&current, recharge.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != constants.WalletRechargeStatusRefunded || current.RefundedAmount.String() != "100.00" {
		t.Fatalf("fully refunded recharge: %+v", current)
	}

	records, err := svc.ListRechargeRefunds(recharge.ID)
	if err != nil {
		t.Fatal(err)
	}
	fee := decimal.Zero
	for _, record := range records {
		fee = fee.Add(record.PaymentFeeRefundedAmount.Decimal)
	}
	if !fee.Equal(decimal.RequireFromString("3.01")) {
		t.Fatalf("concurrent fee reversal: %s", fee)
	}
}

func TestRechargeRefundRollsBackDebitWhenRecordUpdateFails(t *testing.T) {
	svc, db, recharge := setupRechargeRefund(t)
	emails := &rechargeRefundEmailQueueStub{}
	repo := walletgormstore.New(db)
	svc = walletapp.NewService(walletapp.Options{Repository: repo, Transactions: repo, RefundEmails: emails})
	failure := errors.New("refund record write failed")
	if err := db.Callback().Update().Before("gorm:update").Register("test_refund_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "wallet_recharge_orders" {
			tx.AddError(failure)
		}
	}); err != nil {
		t.Fatal(err)
	}
	err := svc.RefundRecharge(walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(20)), RequestID: "refund-request-rollback", PaymentFeeRefunded: true, FeeSnapshot: walletcontract.RechargeRefundFeeSnapshot{PaymentAmount: money.FromDecimal(decimal.NewFromInt(100)), PaymentFee: money.FromDecimal(decimal.NewFromInt(3))}})
	if !errors.Is(err, failure) {
		t.Fatalf("expected failure: %v", err)
	}
	var account walletdomain.Account
	if err := db.Where("user_id = 1").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if !account.Balance.Decimal.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("debit not rolled back: %s", account.Balance)
	}
	var current walletdomain.RechargeOrder
	if err := db.First(&current, recharge.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != constants.WalletRechargeStatusSuccess || !current.RefundedAmount.Decimal.IsZero() {
		t.Fatalf("recharge state not rolled back: %+v", current)
	}
	rows, err := svc.ListRechargeRefunds(recharge.ID)
	if err != nil || len(rows) != 0 {
		t.Fatalf("refund not rolled back: %v %v", rows, err)
	}
	if emails.calls != 0 {
		t.Fatal("rolled back refund sent an email")
	}
}

func TestRechargeRefundPersistsPaymentFeeFlag(t *testing.T) {
	for _, refunded := range []bool{true, false} {
		t.Run(fmt.Sprint(refunded), func(t *testing.T) {
			svc, db, recharge := setupRechargeRefund(t)
			input := walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(20)), RequestID: "refund-fee-request-0001", PaymentFeeRefunded: refunded}
			for i := 0; i < 2; i++ {
				if err := svc.RefundRecharge(input); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := svc.ListRechargeRefunds(recharge.ID)
			if err != nil || len(rows) != 1 {
				t.Fatalf("refund records: %v %v", rows, err)
			}
			if rows[0].PaymentFeeRefunded != refunded {
				t.Fatalf("fee flag: %v", rows[0].PaymentFeeRefunded)
			}
			var account walletdomain.Account
			if err := db.Where("user_id = 1").First(&account).Error; err != nil {
				t.Fatal(err)
			}
			if !account.Balance.Decimal.Equal(decimal.NewFromInt(80)) {
				t.Fatalf("unexpected debit: %s", account.Balance)
			}
			input.PaymentFeeRefunded = !refunded
			if err := svc.RefundRecharge(input); !errors.Is(err, walletcontract.ErrInvalidAmount) {
				t.Fatalf("changed fee flag on retry: %v", err)
			}
		})
	}
}

func TestRechargeRefundAllocatesFeeAndAbsorbsRounding(t *testing.T) {
	svc, db, recharge := setupRechargeRefund(t)
	snapshot := walletcontract.RechargeRefundFeeSnapshot{PaymentAmount: money.FromDecimal(decimal.NewFromInt(100)), PaymentFee: money.FromDecimal(decimal.RequireFromString("3.01"))}
	for i, amount := range []int64{40, 60} {
		input := walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(amount)), RequestID: fmt.Sprintf("fee-refund-request-%04d", i), PaymentFeeRefunded: true, FeeSnapshot: snapshot}
		for retry := 0; retry < 2; retry++ {
			if err := svc.RefundRecharge(input); err != nil {
				t.Fatal(err)
			}
		}
		wantStatus := constants.WalletRechargeStatusPartiallyRefunded
		if i == 1 {
			wantStatus = constants.WalletRechargeStatusRefunded
		}
		var current walletdomain.RechargeOrder
		if err := db.First(&current, recharge.ID).Error; err != nil {
			t.Fatal(err)
		}
		if current.Status != wantStatus {
			t.Fatalf("refund %d: status = %s, want %s", i, current.Status, wantStatus)
		}
		rows, total, err := svc.ListUserRechargeOrders(recharge.UserID, 1, 20, wantStatus, "")
		if err != nil || total != 1 || len(rows) != 1 || rows[0].Status != wantStatus {
			t.Fatalf("refund status filter: rows=%+v total=%d error=%v", rows, total, err)
		}
		stats, err := svc.StatsUserRechargeOrders(recharge.UserID, "")
		if err != nil || stats[wantStatus] != 1 || stats[constants.WalletRechargeStatusSuccess] != 0 {
			t.Fatalf("refund status stats: %+v error=%v", stats, err)
		}
	}
	records, err := svc.ListRechargeRefunds(recharge.ID)
	if err != nil || len(records) != 2 {
		t.Fatalf("records: %v %v", records, err)
	}
	if records[0].PaymentFeeRefundedAmount.String() != "1.81" || records[1].PaymentFeeRefundedAmount.String() != "1.20" {
		t.Fatalf("fee allocations: %+v", records)
	}
	var account walletdomain.Account
	if err := db.Where("user_id = 1").First(&account).Error; err != nil {
		t.Fatal(err)
	}
	if !account.Balance.Decimal.IsZero() {
		t.Fatalf("fee changed wallet deduction: %s", account.Balance)
	}
}

func TestRechargeRefundUncheckedFeeIsNotReversed(t *testing.T) {
	svc, _, recharge := setupRechargeRefund(t)
	snapshot := walletcontract.RechargeRefundFeeSnapshot{PaymentAmount: money.FromDecimal(decimal.NewFromInt(100)), PaymentFee: money.FromDecimal(decimal.NewFromInt(3))}
	for i, amount := range []int64{10, 30, 60} {
		err := svc.RefundRecharge(walletcontract.RechargeRefundInput{RechargeID: recharge.ID, AdminID: 1, Amount: money.FromDecimal(decimal.NewFromInt(amount)), RequestID: fmt.Sprintf("unchecked-fee-refund-%04d", i), PaymentFeeRefunded: i != 0, FeeSnapshot: snapshot})
		if err != nil {
			t.Fatal(err)
		}
	}
	records, err := svc.ListRechargeRefunds(recharge.ID)
	if err != nil {
		t.Fatal(err)
	}
	total := decimal.Zero
	for _, record := range records {
		total = total.Add(record.PaymentFeeRefundedAmount.Decimal)
		if !record.PaymentFeeRefunded && !record.PaymentFeeRefundedAmount.Decimal.IsZero() {
			t.Fatal("unchecked fee reversed")
		}
	}
	if !total.Equal(decimal.RequireFromString("2.70")) {
		t.Fatalf("returned fee: %s", total)
	}
}
