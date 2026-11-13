package refund_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	refundapp "github.com/dujiao-next/internal/modules/order/application/refund"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	walletcontract "github.com/dujiao-next/internal/modules/wallet/contract"
	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"
)

// refundOrderForMode 分别调用手动退款与订单退款到钱包的真实入口。
func refundOrderForMode(svc *refundapp.Service, mode string, orderID uint, amount int64) error {
	value := money.FromDecimal(decimal.NewFromInt(amount))
	if mode == "wallet" {
		_, _, _, err := svc.AdminRefundToWallet(refundapp.AdminRefundToWalletInput{OrderID: orderID, Amount: value})
		return err
	}
	_, _, err := svc.AdminManualRefund(refundapp.AdminManualRefundInput{OrderID: orderID, Amount: value})
	return err
}

// setupSpentRefund 构造已付款订单与累计消费，不依赖额外数据库连接。
func setupSpentRefund(t *testing.T, totalSpent int64) (*refundapp.Service, *gorm.DB, *orderdomain.Order) {
	t.Helper()
	svc, db := setupOrderRefundWalletTest(t)
	raw, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	createTestUser(t, db, 1)
	if err := db.Model(&userdomain.User{}).Where("id = 1").Updates(map[string]interface{}{
		"total_spent": totalSpent, "total_recharged": 500, "member_level_id": 17,
	}).Error; err != nil {
		t.Fatal(err)
	}
	order := createTestOrder(t, db, 1, "SPENT-REFUND-001", decimal.NewFromInt(100))
	paid := time.Now()
	order.PaidAt, order.Status = &paid, constants.OrderStatusCompleted
	if err := db.Save(order).Error; err != nil {
		t.Fatal(err)
	}
	return svc, db, order
}

func TestBothOrderRefundMethodsDecreaseTotalSpent(t *testing.T) {
	for _, mode := range []string{"manual", "wallet"} {
		for _, tc := range []struct {
			name    string
			initial int64
			amounts []int64
			want    int64
		}{
			{"partial_then_full", 250, []int64{30, 70}, 150},
			{"full", 100, []int64{100}, 0},
			{"historical_total_clamped", 20, []int64{100}, 0},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				svc, db, order := setupSpentRefund(t, tc.initial)
				for _, amount := range tc.amounts {
					if err := refundOrderForMode(svc, mode, order.ID, amount); err != nil {
						t.Fatal(err)
					}
				}
				var user userdomain.User
				if err := db.First(&user, 1).Error; err != nil {
					t.Fatal(err)
				}
				if !user.TotalSpent.Decimal.Equal(decimal.NewFromInt(tc.want)) || user.TotalRecharged.String() != "500.00" || user.MemberLevelID != 17 {
					t.Fatalf("refund cumulative values: spent=%s recharged=%s level=%d", user.TotalSpent, user.TotalRecharged, user.MemberLevelID)
				}
				account, err := walletServiceForTest(db).GetAccount(1)
				wantBalance := "0.00"
				if mode == "wallet" {
					wantBalance = "100.00"
				}
				if err != nil || account.Balance.String() != wantBalance {
					t.Fatalf("wallet=%+v error=%v", account, err)
				}
				if err := refundOrderForMode(svc, mode, order.ID, 1); !errors.Is(err, walletcontract.ErrRefundExceeded) {
					t.Fatalf("excess refund: %v", err)
				}
				if err := db.First(&user, 1).Error; err != nil {
					t.Fatal(err)
				}
				if !user.TotalSpent.Decimal.Equal(decimal.NewFromInt(tc.want)) {
					t.Fatalf("rejected refund changed cumulative spending: %s", user.TotalSpent)
				}
			})
		}
	}
}

func TestOrderRefundSpendingFailureRollsBackAllChanges(t *testing.T) {
	for _, mode := range []string{"manual", "wallet"} {
		t.Run(mode, func(t *testing.T) {
			svc, db, order := setupSpentRefund(t, 250)
			failure := errors.New("total spent update failed")
			if err := db.Callback().Update().Before("gorm:update").Register("test_spent_refund_failure", func(tx *gorm.DB) {
				values, ok := tx.Statement.Dest.(map[string]interface{})
				if _, changesSpent := values["total_spent"]; ok && changesSpent && tx.Statement.Table == "users" {
					tx.AddError(failure)
				}
			}); err != nil {
				t.Fatal(err)
			}
			if err := refundOrderForMode(svc, mode, order.ID, 40); !errors.Is(err, failure) {
				t.Fatalf("refund error: %v", err)
			}
			var user userdomain.User
			if err := db.First(&user, 1).Error; err != nil {
				t.Fatal(err)
			}
			if user.TotalSpent.String() != "250.00" || user.TotalRecharged.String() != "500.00" || user.MemberLevelID != 17 {
				t.Fatalf("user not rolled back: %+v", user)
			}
			var current orderdomain.Order
			if err := db.First(&current, order.ID).Error; err != nil {
				t.Fatal(err)
			}
			if current.RefundedAmount.String() != "0.00" || current.Status != constants.OrderStatusCompleted {
				t.Fatalf("order not rolled back: %+v", current)
			}
			for _, model := range []interface{}{&orderdomain.OrderRefundRecord{}, &walletdomain.Transaction{}, &walletdomain.Account{}} {
				var count int64
				if err := db.Model(model).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("rollback records for %T: count=%d error=%v", model, count, err)
				}
			}
		})
	}
}

func TestParentAndChildRefundsOnlyDeductActualRefundAmount(t *testing.T) {
	for _, mode := range []string{"manual", "wallet"} {
		t.Run(mode, func(t *testing.T) {
			svc, db, parent := setupSpentRefund(t, 1000)
			children := make([]*orderdomain.Order, 2)
			for i, amount := range []int64{60, 40} {
				children[i] = createTestOrder(t, db, 1, fmt.Sprintf("SPENT-CHILD-%d", i), decimal.NewFromInt(amount))
				children[i].ParentID, children[i].PaidAt, children[i].Status = &parent.ID, parent.PaidAt, constants.OrderStatusCompleted
				if err := db.Save(children[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := refundOrderForMode(svc, mode, children[0].ID, 60); err != nil {
				t.Fatal(err)
			}
			if err := refundOrderForMode(svc, mode, parent.ID, 40); err != nil {
				t.Fatal(err)
			}
			if err := refundOrderForMode(svc, mode, children[1].ID, 1); !errors.Is(err, walletcontract.ErrRefundExceeded) {
				t.Fatalf("parent refund limit: %v", err)
			}
			var user userdomain.User
			if err := db.First(&user, 1).Error; err != nil {
				t.Fatal(err)
			}
			if user.TotalSpent.String() != "900.00" {
				t.Fatalf("parent and child refund double deduction: %s", user.TotalSpent)
			}
			var count int64
			if err := db.Model(&orderdomain.OrderRefundRecord{}).Count(&count).Error; err != nil || count != 2 {
				t.Fatalf("refund records: count=%d error=%v", count, err)
			}
		})
	}
}

func TestConcurrentOrderRefundOnlyDeductsTotalSpentOnce(t *testing.T) {
	for _, mode := range []string{"manual", "wallet"} {
		t.Run(mode, func(t *testing.T) {
			svc, db, order := setupSpentRefund(t, 250)
			const attempts = 8
			start := make(chan struct{})
			results := make(chan error, attempts)
			for i := 0; i < attempts; i++ {
				go func() {
					<-start
					results <- refundOrderForMode(svc, mode, order.ID, 100)
				}()
			}
			close(start)
			successes := 0
			for i := 0; i < attempts; i++ {
				err := <-results
				if err == nil {
					successes++
				} else if !errors.Is(err, walletcontract.ErrRefundExceeded) {
					t.Errorf("concurrent refund: %v", err)
				}
			}
			var user userdomain.User
			if err := db.First(&user, 1).Error; err != nil {
				t.Fatal(err)
			}
			if successes != 1 || user.TotalSpent.String() != "150.00" {
				t.Fatalf("successes=%d cumulative spending=%s", successes, user.TotalSpent)
			}
		})
	}
}
