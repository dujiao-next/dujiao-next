package integrationtest

import (
	"errors"
	"fmt"
	"testing"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openResellerOrderServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:reseller_order_service_%d?mode=memory&cache=shared", uniqueTestID())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&userdomain.User{},
		&orderdomain.Order{},
		&orderdomain.OrderItem{},
		&resellerdomain.Profile{},
		&resellerdomain.OrderSnapshot{},
		&resellerdomain.LedgerEntry{},
	); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func seedResellerOrderFixture(t *testing.T, db *gorm.DB, email string) (resellerdomain.Profile, orderdomain.Order, resellerdomain.OrderSnapshot) {
	t.Helper()
	user := userdomain.User{Email: email, PasswordHash: "hash", Status: constants.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	profile := resellerdomain.Profile{
		UserID:           user.ID,
		Status:           resellerdomain.ProfileStatusActive,
		SettlementStatus: resellerdomain.SettlementStatusNormal,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	paidAt := time.Now().Add(-time.Hour)
	order := orderdomain.Order{
		OrderNo:              fmt.Sprintf("DJ-RES-%d", uniqueTestID()),
		UserID:               999,
		Status:               constants.OrderStatusPaid,
		Currency:             "USD",
		TotalAmount:          money.FromDecimal(decimal.RequireFromString("130.00")),
		ResellerID:           &profile.ID,
		ResellerDomain:       "shop.example.test",
		ResellerProfitAmount: money.FromDecimal(decimal.RequireFromString("30.00")),
		PaidAt:               &paidAt,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order failed: %v", err)
	}
	item := orderdomain.OrderItem{
		OrderID:         order.ID,
		ProductID:       10,
		SKUID:           20,
		TitleJSON:       jsonmap.JSON{"zh-CN": "测试商品"},
		SKUSnapshotJSON: jsonmap.JSON{"规格": "A"},
		Quantity:        2,
		UnitPrice:       money.FromDecimal(decimal.RequireFromString("65.00")),
		TotalPrice:      money.FromDecimal(decimal.RequireFromString("130.00")),
		CostPrice:       money.FromDecimal(decimal.RequireFromString("1.00")),
	}
	if err := db.Create(&item).Error; err != nil {
		t.Fatalf("create order item failed: %v", err)
	}
	snapshot := resellerdomain.OrderSnapshot{
		OrderID:           order.ID,
		ResellerID:        profile.ID,
		Domain:            order.ResellerDomain,
		Currency:          order.Currency,
		ResellerUserID:    profile.UserID,
		BuyerUserID:       order.UserID,
		BaseAmount:        money.FromDecimal(decimal.RequireFromString("100.00")),
		ResellerAmount:    money.FromDecimal(decimal.RequireFromString("130.00")),
		ProfitAmount:      money.FromDecimal(decimal.RequireFromString("30.00")),
		ProfitEligible:    false,
		ProfitBlockReason: "self_dealing_owner",
		PricingSnapshotJSON: jsonmap.JSON{"items": []interface{}{
			map[string]interface{}{
				"order_item_id":          item.ID,
				"base_unit_amount":       "50.00",
				"reseller_unit_amount":   "65.00",
				"base_total_amount":      "100.00",
				"reseller_total_amount":  "130.00",
				"profit_amount":          "30.00",
				"profit_block_reason":    "self_dealing_owner",
				"internal_risk_decision": "blocked",
			},
		}},
	}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}
	if err := db.Model(&resellerdomain.OrderSnapshot{}).
		Where("id = ?", snapshot.ID).
		Update("profit_eligible", false).Error; err != nil {
		t.Fatalf("force ineligible snapshot failed: %v", err)
	}
	snapshot.ProfitEligible = false
	return profile, order, snapshot
}

func seedResellerOrderWithChildItemsFixture(t *testing.T, db *gorm.DB, email string) (resellerdomain.Profile, orderdomain.Order, []orderdomain.OrderItem) {
	t.Helper()
	user := userdomain.User{Email: email, PasswordHash: "hash", Status: constants.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	profile := resellerdomain.Profile{
		UserID:           user.ID,
		Status:           resellerdomain.ProfileStatusActive,
		SettlementStatus: resellerdomain.SettlementStatusNormal,
	}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	paidAt := time.Now().Add(-time.Hour)
	parent := orderdomain.Order{
		OrderNo:              fmt.Sprintf("DJ-RES-PARENT-%d", uniqueTestID()),
		UserID:               user.ID,
		Status:               constants.OrderStatusPaid,
		Currency:             "USD",
		TotalAmount:          money.FromDecimal(decimal.RequireFromString("150.00")),
		ResellerID:           &profile.ID,
		ResellerDomain:       "child-items.example.test",
		ResellerProfitAmount: money.FromDecimal(decimal.RequireFromString("30.00")),
		PaidAt:               &paidAt,
	}
	if err := db.Create(&parent).Error; err != nil {
		t.Fatalf("create parent order failed: %v", err)
	}
	var items []orderdomain.OrderItem
	childAmounts := []string{"70.00", "80.00"}
	for idx, amount := range childAmounts {
		child := orderdomain.Order{
			OrderNo:              fmt.Sprintf("%s-%d", parent.OrderNo, idx+1),
			ParentID:             &parent.ID,
			UserID:               user.ID,
			Status:               constants.OrderStatusPaid,
			Currency:             "USD",
			TotalAmount:          money.FromDecimal(decimal.RequireFromString(amount)),
			ResellerID:           &profile.ID,
			ResellerDomain:       parent.ResellerDomain,
			ResellerProfitAmount: money.FromDecimal(decimal.RequireFromString("15.00")),
			PaidAt:               &paidAt,
		}
		if err := db.Create(&child).Error; err != nil {
			t.Fatalf("create child order failed: %v", err)
		}
		item := orderdomain.OrderItem{
			OrderID:         child.ID,
			ProductID:       uint(100 + idx),
			SKUID:           uint(200 + idx),
			TitleJSON:       jsonmap.JSON{"zh-CN": fmt.Sprintf("子订单商品 %d", idx+1)},
			SKUSnapshotJSON: jsonmap.JSON{"规格": fmt.Sprintf("S%d", idx+1)},
			Quantity:        1,
			UnitPrice:       money.FromDecimal(decimal.RequireFromString(amount)),
			TotalPrice:      money.FromDecimal(decimal.RequireFromString(amount)),
			CostPrice:       money.FromDecimal(decimal.RequireFromString("2.00")),
		}
		if err := db.Create(&item).Error; err != nil {
			t.Fatalf("create child item failed: %v", err)
		}
		items = append(items, item)
	}
	snapshot := resellerdomain.OrderSnapshot{
		OrderID:        parent.ID,
		ResellerID:     profile.ID,
		Domain:         parent.ResellerDomain,
		Currency:       parent.Currency,
		ResellerUserID: profile.UserID,
		BuyerUserID:    parent.UserID,
		BaseAmount:     money.FromDecimal(decimal.RequireFromString("120.00")),
		ResellerAmount: money.FromDecimal(decimal.RequireFromString("150.00")),
		ProfitAmount:   money.FromDecimal(decimal.RequireFromString("30.00")),
		ProfitEligible: true,
		PricingSnapshotJSON: jsonmap.JSON{"items": []interface{}{
			map[string]interface{}{
				"order_item_id":         items[0].ID,
				"base_unit_amount":      "55.00",
				"reseller_unit_amount":  "70.00",
				"base_total_amount":     "55.00",
				"reseller_total_amount": "70.00",
				"profit_amount":         "15.00",
			},
			map[string]interface{}{
				"order_item_id":         items[1].ID,
				"base_unit_amount":      "65.00",
				"reseller_unit_amount":  "80.00",
				"base_total_amount":     "65.00",
				"reseller_total_amount": "80.00",
				"profit_amount":         "15.00",
			},
		}},
	}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("create snapshot failed: %v", err)
	}
	return profile, parent, items
}

func TestResellerOrderServiceListUsesSnapshotAndHidesRiskFields(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, order, _ := seedResellerOrderFixture(t, db, "reseller-orders@example.test")
	_, otherOrder, _ := seedResellerOrderFixture(t, db, "other-reseller-orders@example.test")
	svc := NewResellerOrderService(resellergormstore.New(db))

	rows, total, err := svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed: %v", err)
	}
	if total != 1 || len(rows) != 1 {
		t.Fatalf("expected one row, total=%d rows=%d", total, len(rows))
	}
	if rows[0].OrderNo != order.OrderNo || rows[0].OrderNo == otherOrder.OrderNo {
		t.Fatalf("unexpected order isolation: %+v", rows[0])
	}
	if rows[0].BaseAmount.StringFixed(2) != "100.00" || rows[0].ProfitAmount.StringFixed(2) != "30.00" {
		t.Fatalf("expected snapshot amounts, got base=%s profit=%s", rows[0].BaseAmount.StringFixed(2), rows[0].ProfitAmount.StringFixed(2))
	}
	if rows[0].ProfitStatus != ResellerProfitStatusUnavailable {
		t.Fatalf("blocked or ineligible profit must stay neutral unavailable, got %+v", rows[0])
	}
}

func TestResellerOrderServiceBuyerLabelMasksMemberEmail(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, order, _ := seedResellerOrderFixture(t, db, "reseller-buyer-label@example.test")
	buyer := userdomain.User{
		Email:        "buyer-label@example.test",
		PasswordHash: "hash",
		Status:       constants.UserStatusActive,
	}
	if err := db.Create(&buyer).Error; err != nil {
		t.Fatalf("create buyer user failed: %v", err)
	}
	if err := db.Model(&orderdomain.Order{}).Where("id = ?", order.ID).Update("user_id", buyer.ID).Error; err != nil {
		t.Fatalf("update order buyer failed: %v", err)
	}
	if err := db.Model(&resellerdomain.OrderSnapshot{}).Where("order_id = ?", order.ID).Update("buyer_user_id", buyer.ID).Error; err != nil {
		t.Fatalf("update snapshot buyer failed: %v", err)
	}
	svc := NewResellerOrderService(resellergormstore.New(db))

	rows, _, err := svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].BuyerLabel != "b***@example.test" {
		t.Fatalf("member buyer label should mask email instead of hardcoded user, got %q", rows[0].BuyerLabel)
	}

	detail, err := svc.GetUserOrderDetail(profile.UserID, order.OrderNo)
	if err != nil {
		t.Fatalf("GetUserOrderDetail failed: %v", err)
	}
	if detail.BuyerLabel != "b***@example.test" {
		t.Fatalf("member detail buyer label should mask email instead of hardcoded user, got %q", detail.BuyerLabel)
	}
}

func TestResellerOrderServiceProfitStatusRequiresAvailableLedger(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, order, snapshot := seedResellerOrderFixture(t, db, "reseller-ledger-status@example.test")
	if err := db.Model(&resellerdomain.OrderSnapshot{}).Where("id = ?", snapshot.ID).Updates(map[string]interface{}{
		"profit_eligible":     true,
		"profit_block_reason": "",
	}).Error; err != nil {
		t.Fatalf("update snapshot failed: %v", err)
	}
	svc := NewResellerOrderService(resellergormstore.New(db))

	rows, _, err := svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed: %v", err)
	}
	if rows[0].ProfitStatus == ResellerProfitStatusCredited {
		t.Fatalf("paid order without ledger must not be credited: %+v", rows[0])
	}

	orderID := order.ID
	if err := db.Create(&resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		OrderID:        &orderID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.RequireFromString("30.00")),
		Currency:       "USD",
		IdempotencyKey: "order-profit-status-pending",
		Status:         resellerdomain.LedgerStatusPendingConfirm,
	}).Error; err != nil {
		t.Fatalf("create pending ledger failed: %v", err)
	}
	rows, _, err = svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed after pending ledger: %v", err)
	}
	if rows[0].ProfitStatus != ResellerProfitStatusPending {
		t.Fatalf("pending_confirm ledger must map to pending, got %+v", rows[0])
	}

	if err := db.Model(&resellerdomain.LedgerEntry{}).Where("idempotency_key = ?", "order-profit-status-pending").Update("status", resellerdomain.LedgerStatusAvailable).Error; err != nil {
		t.Fatalf("update ledger failed: %v", err)
	}
	rows, _, err = svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed after available ledger: %v", err)
	}
	if rows[0].ProfitStatus != ResellerProfitStatusCredited {
		t.Fatalf("available ledger must map to credited, got %+v", rows[0])
	}
}

func TestResellerOrderServicePartiallyRefundedOrderIsNeutralUnavailable(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, order, snapshot := seedResellerOrderFixture(t, db, "reseller-partial-refund@example.test")
	if err := db.Model(&resellerdomain.OrderSnapshot{}).Where("id = ?", snapshot.ID).Updates(map[string]interface{}{
		"profit_eligible":     true,
		"profit_block_reason": "",
	}).Error; err != nil {
		t.Fatalf("update snapshot failed: %v", err)
	}
	if err := db.Model(&orderdomain.Order{}).Where("id = ?", order.ID).Update("status", constants.OrderStatusPartiallyRefunded).Error; err != nil {
		t.Fatalf("update order failed: %v", err)
	}
	orderID := order.ID
	if err := db.Create(&resellerdomain.LedgerEntry{
		ResellerID:     profile.ID,
		OrderID:        &orderID,
		Type:           resellerdomain.LedgerTypeOrderProfit,
		Amount:         money.FromDecimal(decimal.RequireFromString("30.00")),
		Currency:       "USD",
		IdempotencyKey: "order-profit-status-partially-refunded",
		Status:         resellerdomain.LedgerStatusAvailable,
	}).Error; err != nil {
		t.Fatalf("create available ledger failed: %v", err)
	}
	svc := NewResellerOrderService(resellergormstore.New(db))

	rows, _, err := svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed: %v", err)
	}
	if rows[0].ProfitStatus != ResellerProfitStatusUnavailable {
		t.Fatalf("partially refunded order must not show gross snapshot profit as credited, got %+v", rows[0])
	}
}

func TestResellerOrderServiceDetailUsesItemSnapshot(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, order, _ := seedResellerOrderFixture(t, db, "reseller-order-detail@example.test")
	svc := NewResellerOrderService(resellergormstore.New(db))

	detail, err := svc.GetUserOrderDetail(profile.UserID, order.OrderNo)
	if err != nil {
		t.Fatalf("GetUserOrderDetail failed: %v", err)
	}
	if len(detail.Items) != 1 {
		t.Fatalf("expected one item, got %d", len(detail.Items))
	}
	if detail.Items[0].BaseUnitAmount != "50.00" || detail.Items[0].ResellerUnitAmount != "65.00" || detail.Items[0].ProfitAmount != "30.00" {
		t.Fatalf("expected item pricing snapshot, got %+v", detail.Items[0])
	}
}

func TestResellerOrderServiceDetailAggregatesItemsFromChildOrders(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, parent, childItems := seedResellerOrderWithChildItemsFixture(t, db, "reseller-child-items@example.test")
	svc := NewResellerOrderService(resellergormstore.New(db))

	rows, _, err := svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListUserOrders failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one order row, got %d", len(rows))
	}
	if rows[0].ItemsCount != len(childItems) {
		t.Fatalf("expected list items_count from child orders=%d, got %d", len(childItems), rows[0].ItemsCount)
	}

	detail, err := svc.GetUserOrderDetail(profile.UserID, parent.OrderNo)
	if err != nil {
		t.Fatalf("GetUserOrderDetail failed: %v", err)
	}
	if len(detail.Items) != len(childItems) {
		t.Fatalf("expected detail items from child orders=%d, got %d", len(childItems), len(detail.Items))
	}
	if detail.Items[0].BaseUnitAmount != "55.00" || detail.Items[0].ResellerUnitAmount != "70.00" || detail.Items[0].ProfitAmount != "15.00" {
		t.Fatalf("expected first child item pricing snapshot, got %+v", detail.Items[0])
	}
	if detail.Items[1].BaseUnitAmount != "65.00" || detail.Items[1].ResellerUnitAmount != "80.00" || detail.Items[1].ProfitAmount != "15.00" {
		t.Fatalf("expected second child item pricing snapshot, got %+v", detail.Items[1])
	}
}

func TestResellerOrderServiceRejectsInactiveProfile(t *testing.T) {
	db := openResellerOrderServiceTestDB(t)
	profile, _, _ := seedResellerOrderFixture(t, db, "inactive-reseller-orders@example.test")
	if err := db.Model(&resellerdomain.Profile{}).Where("id = ?", profile.ID).Update("status", resellerdomain.ProfileStatusPendingReview).Error; err != nil {
		t.Fatalf("update profile failed: %v", err)
	}
	svc := NewResellerOrderService(resellergormstore.New(db))
	_, _, err := svc.ListUserOrders(profile.UserID, ResellerOrderListInput{Page: 1, PageSize: 20})
	if !errors.Is(err, ErrResellerProfileInactive) {
		t.Fatalf("expected ErrResellerProfileInactive, got %v", err)
	}
}
