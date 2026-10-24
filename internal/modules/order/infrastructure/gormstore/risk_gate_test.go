package gormstore

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	orderriskdomain "github.com/dujiao-next/internal/modules/orderrisk/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupRiskGateDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "risk-gate.db") + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&orderdomain.Order{}, &orderdomain.OrderItem{}, &orderriskdomain.LockKey{}); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(10)
	return db
}

func TestRiskGateCountsOnlyMatchingPendingIdentityAndProducts(t *testing.T) {
	db := setupRiskGateDB(t)
	store := New(db, "risk-gate-test-secret")
	riskIP := "1.2.3.4"

	guestParent := orderdomain.Order{OrderNo: "guest-parent", UserID: 0, Status: constants.OrderStatusPendingPayment, Currency: "USD", RiskIP: riskIP}
	if err := db.Create(&guestParent).Error; err != nil {
		t.Fatal(err)
	}
	guestChild := orderdomain.Order{OrderNo: "guest-child", ParentID: &guestParent.ID, UserID: 0, Status: constants.OrderStatusPendingPayment, Currency: "USD", RiskIP: riskIP}
	if err := db.Create(&guestChild).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&orderdomain.OrderItem{OrderID: guestChild.ID, ProductID: 7, TitleJSON: jsonmap.JSON{}, Quantity: 2}).Error; err != nil {
		t.Fatal(err)
	}
	for _, order := range []orderdomain.Order{
		{OrderNo: "member-parent", UserID: 9, Status: constants.OrderStatusPendingPayment, Currency: "USD", RiskIP: riskIP},
		{OrderNo: "canceled-guest", UserID: 0, Status: constants.OrderStatusCanceled, Currency: "USD", RiskIP: riskIP},
		{OrderNo: "other-ip", UserID: 0, Status: constants.OrderStatusPendingPayment, Currency: "USD", RiskIP: "5.6.7.8"},
	} {
		if err := db.Create(&order).Error; err != nil {
			t.Fatal(err)
		}
	}

	guestCount, err := store.CountPendingGuestByRiskIP(riskIP)
	if err != nil || guestCount != 1 {
		t.Fatalf("guest count=%d err=%v", guestCount, err)
	}
	memberCount, err := store.CountPendingMemberByRiskIP(riskIP)
	if err != nil || memberCount != 1 {
		t.Fatalf("member count=%d err=%v", memberCount, err)
	}
	quantities, err := store.SumPendingGuestQuantityByRiskIP(riskIP, []uint{7, 8})
	if err != nil || quantities[7] != 2 || quantities[8] != 0 {
		t.Fatalf("quantities=%v err=%v", quantities, err)
	}
}

func TestRiskGateSerializesConcurrentGuestQuotaChecks(t *testing.T) {
	db := setupRiskGateDB(t)
	store := New(db, "risk-gate-test-secret")
	const limit = int64(2)
	var allowed atomic.Int64
	var wg sync.WaitGroup
	errorsCh := make(chan error, 12)
	start := make(chan struct{})

	for index := 0; index < 12; index++ {
		index := index
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := store.db.Transaction(func(dbtx *gorm.DB) error {
				txStore := &Store{db: dbtx, guestCredentialSecret: store.guestCredentialSecret}
				if err := txStore.LockRiskKeys([]string{"guest:ip:1.2.3.4"}); err != nil {
					return err
				}
				count, err := txStore.CountPendingGuestByRiskIP("1.2.3.4")
				if err != nil {
					return err
				}
				if count >= limit {
					return errQuotaReached
				}
				time.Sleep(5 * time.Millisecond)
				order := &orderdomain.Order{
					OrderNo:  fmt.Sprintf("concurrent-%d", index),
					UserID:   0,
					Status:   constants.OrderStatusPendingPayment,
					Currency: "USD",
					RiskIP:   "1.2.3.4",
				}
				if err := txStore.Create(order, nil); err != nil {
					return err
				}
				allowed.Add(1)
				return nil
			})
			if err != nil && !errors.Is(err, errQuotaReached) {
				errorsCh <- err
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Fatalf("unexpected concurrent transaction error: %v", err)
	}
	if allowed.Load() != limit {
		t.Fatalf("allowed=%d, want %d", allowed.Load(), limit)
	}
	count, err := store.CountPendingGuestByRiskIP("1.2.3.4")
	if err != nil || count != limit {
		t.Fatalf("persisted count=%d err=%v", count, err)
	}
	var locks []orderriskdomain.LockKey
	if err := db.Find(&locks).Error; err != nil {
		t.Fatal(err)
	}
	if len(locks) != 1 || locks[0].KeyHash == "guest:ip:1.2.3.4" {
		t.Fatalf("expected one hashed lock key, got %#v", locks)
	}
}

var errQuotaReached = errors.New("quota reached")
