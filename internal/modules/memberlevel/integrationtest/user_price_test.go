package integrationtest

import (
	"errors"
	"testing"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func newUserPriceServiceForTest(t *testing.T) (userID uint, svc interface {
	BatchUpsertUserPrices([]memberleveldomain.UserProductPrice) error
	ResolveUserPrice(userID, productID, skuID uint, base decimal.Decimal) (decimal.Decimal, bool)
	DeleteUserPrice(id uint) error
	ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error)
}) {
	t.Helper()
	base, db := newMemberLevelServiceForTest(t)
	if err := db.AutoMigrate(&memberleveldomain.UserProductPrice{}); err != nil {
		t.Fatalf("auto migrate user price failed: %v", err)
	}
	base.WithUserPriceRepository(gormstore.NewUserPriceStore(db))
	user := createUserFixture(t, db, "user-price@example.com", 0)
	return user.ID, base
}

func userPrice(userID, productID, skuID uint, amount string) memberleveldomain.UserProductPrice {
	return memberleveldomain.UserProductPrice{
		UserID:      userID,
		ProductID:   productID,
		SKUID:       skuID,
		PriceAmount: money.FromDecimal(decimal.RequireFromString(amount)),
	}
}

func TestResolveUserPricePrefersSKUOverProductAndOnlyLowersPrice(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{
		userPrice(userID, 10, 0, "95"),
		userPrice(userID, 10, 7, "90"),
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	base := decimal.NewFromInt(100)

	if got, ok := svc.ResolveUserPrice(userID, 10, 7, base); !ok || !got.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("sku-level price = (%s, %v), want (90, true)", got, ok)
	}
	if got, ok := svc.ResolveUserPrice(userID, 10, 8, base); !ok || !got.Equal(decimal.NewFromInt(95)) {
		t.Fatalf("product-level fallback = (%s, %v), want (95, true)", got, ok)
	}
	// 基础价降到专属价以下时不得反向加价。
	if got, ok := svc.ResolveUserPrice(userID, 10, 7, decimal.NewFromInt(80)); ok || !got.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("price above base = (%s, %v), want (80, false)", got, ok)
	}
	if _, ok := svc.ResolveUserPrice(userID+1, 10, 7, base); ok {
		t.Fatal("other user must not see this user's price")
	}
	if _, ok := svc.ResolveUserPrice(userID, 11, 0, base); ok {
		t.Fatal("other product must not match")
	}
}

func TestBatchUpsertUserPricesValidatesAndUpdatesInPlace(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)

	for _, bad := range []memberleveldomain.UserProductPrice{
		userPrice(userID, 10, 0, "0"),
		userPrice(userID, 10, 0, "0.004"),
		userPrice(userID, 0, 0, "5"),
		userPrice(0, 10, 0, "5"),
	} {
		if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{bad}); !errors.Is(err, memberlevelcontract.ErrUserPriceInvalid) {
			t.Fatalf("expected ErrUserPriceInvalid for %+v, got %v", bad, err)
		}
	}
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID+999, 10, 0, "5")}); !errors.Is(err, memberlevelcontract.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}

	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID, 10, 0, "95")}); err != nil {
		t.Fatalf("first upsert failed: %v", err)
	}
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID, 10, 0, "92.5")}); err != nil {
		t.Fatalf("second upsert failed: %v", err)
	}
	rows, total, err := svc.ListUserPrices(userID, 1, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("list = (%d rows, total %d, %v), want exactly one row", len(rows), total, err)
	}
	if rows[0].PriceAmount.String() != "92.50" {
		t.Fatalf("price = %s, want 92.50", rows[0].PriceAmount.String())
	}
}

func TestDeleteUserPriceThenReUpsertRevivesRow(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID, 10, 0, "95")}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	rows, _, _ := svc.ListUserPrices(userID, 1, 20)
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	if err := svc.DeleteUserPrice(rows[0].ID); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, ok := svc.ResolveUserPrice(userID, 10, 0, decimal.NewFromInt(100)); ok {
		t.Fatal("deleted price must not resolve")
	}
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID, 10, 0, "90")}); err != nil {
		t.Fatalf("re-upsert after soft delete failed (unique key collision?): %v", err)
	}
	if got, ok := svc.ResolveUserPrice(userID, 10, 0, decimal.NewFromInt(100)); !ok || !got.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("revived price = (%s, %v), want (90, true)", got, ok)
	}
}
