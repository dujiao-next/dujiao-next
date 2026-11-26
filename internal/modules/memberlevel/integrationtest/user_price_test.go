package integrationtest

import (
	"errors"
	"testing"

	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func newUserPriceServiceForTest(t *testing.T) (userID uint, svc interface {
	BatchUpsertUserPrices([]memberleveldomain.UserProductPrice) error
	LoadUserPriceBook(userID uint) (memberlevelapp.UserPriceBook, error)
	DeleteUserPrice(id uint) error
	ListUserPrices(userID uint, page, pageSize int) ([]memberleveldomain.UserProductPrice, int64, error)
}) {
	t.Helper()
	base, db := newMemberLevelServiceForTest(t)
	if err := db.AutoMigrate(&memberleveldomain.UserProductPrice{}); err != nil {
		t.Fatalf("auto migrate user price failed: %v", err)
	}
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

func TestUserPriceBookPrefersSKUOverProductAndOnlyLowersPrice(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{
		userPrice(userID, 10, 0, "95"),
		userPrice(userID, 10, 7, "90"),
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	book, err := svc.LoadUserPriceBook(userID)
	if err != nil {
		t.Fatalf("load book failed: %v", err)
	}
	base := decimal.NewFromInt(100)

	if got, ok := book.Resolve(10, 7, base); !ok || !got.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("sku-level price = (%s, %v), want (90, true)", got, ok)
	}
	if got, ok := book.Resolve(10, 8, base); !ok || !got.Equal(decimal.NewFromInt(95)) {
		t.Fatalf("product-level fallback = (%s, %v), want (95, true)", got, ok)
	}
	// 基础价降到专属价以下时不得反向加价。
	if got, ok := book.Resolve(10, 7, decimal.NewFromInt(80)); ok || !got.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("price above base = (%s, %v), want (80, false)", got, ok)
	}
	if _, ok := book.Resolve(11, 0, base); ok {
		t.Fatal("other product must not match")
	}
	// 其他用户读到的快照里没有这条专属价。
	otherBook, err := svc.LoadUserPriceBook(userID + 1)
	if err != nil {
		t.Fatalf("load other book failed: %v", err)
	}
	if _, ok := otherBook.Resolve(10, 7, base); ok {
		t.Fatal("other user must not see this user's price")
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

func userDiscountPrice(userID, productID, skuID uint, rate string) memberleveldomain.UserProductPrice {
	return memberleveldomain.UserProductPrice{
		UserID:       userID,
		ProductID:    productID,
		SKUID:        skuID,
		DiscountRate: money.FromDecimal(decimal.RequireFromString(rate)),
	}
}

// 折扣行按基础价打折，SKU 级优先于商品级，且与会员等级折扣同款四舍五入。
func TestUserPriceBookAppliesDiscountRate(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{
		userDiscountPrice(userID, 10, 0, "90"),
		userDiscountPrice(userID, 10, 7, "80"),
	}); err != nil {
		t.Fatalf("upsert discount failed: %v", err)
	}
	book, err := svc.LoadUserPriceBook(userID)
	if err != nil {
		t.Fatalf("load book failed: %v", err)
	}

	base := decimal.NewFromInt(100)
	if got, ok := book.Resolve(10, 7, base); !ok || !got.Equal(decimal.NewFromInt(80)) {
		t.Fatalf("sku-level discount = (%s, %v), want (80, true)", got, ok)
	}
	if got, ok := book.Resolve(10, 8, base); !ok || !got.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("product-level discount = (%s, %v), want (90, true)", got, ok)
	}
	// 折后价随基础价浮动：原价变了，专属价跟着变，不用改配置。
	if got, ok := book.Resolve(10, 7, decimal.NewFromInt(200)); !ok || !got.Equal(decimal.NewFromInt(160)) {
		t.Fatalf("discount must follow base, got (%s, %v), want (160, true)", got, ok)
	}
	// 四舍五入到分，与 ResolveMemberPrice 的 rate 口径一致。
	if got, ok := book.Resolve(10, 7, decimal.RequireFromString("99.99")); !ok || !got.Equal(decimal.RequireFromString("79.99")) {
		t.Fatalf("discount rounding = (%s, %v), want (79.99, true)", got, ok)
	}
	// base 为 0 时不得算出 0 元成交价。
	if _, ok := book.Resolve(10, 7, decimal.Zero); ok {
		t.Fatal("zero base must not hit discount")
	}
}

// SKU 级折扣行存在时不回退商品级，与旧的一口价规则保持一致。
func TestUserPriceBookSkuDiscountDoesNotFallBackToProduct(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{
		userPrice(userID, 10, 0, "50"),
		userDiscountPrice(userID, 10, 7, "95"),
	}); err != nil {
		t.Fatalf("upsert failed: %v", err)
	}
	book, err := svc.LoadUserPriceBook(userID)
	if err != nil {
		t.Fatalf("load book failed: %v", err)
	}
	if got, ok := book.Resolve(10, 7, decimal.NewFromInt(100)); !ok || !got.Equal(decimal.NewFromInt(95)) {
		t.Fatalf("sku-level discount = (%s, %v), want (95, true)", got, ok)
	}
	rule, _ := book.Rule(10, 7)
	if rule.IsFixed() {
		t.Fatal("discount rule must not be reported as fixed price")
	}
}

// 折扣率越界、两种模式同时填写都必须拒绝；切换模式时清空另一个字段。
func TestBatchUpsertUserPricesValidatesDiscountRate(t *testing.T) {
	userID, svc := newUserPriceServiceForTest(t)

	for _, bad := range []memberleveldomain.UserProductPrice{
		userDiscountPrice(userID, 10, 0, "0"),
		userDiscountPrice(userID, 10, 0, "0.5"),
		userDiscountPrice(userID, 10, 0, "100"),
		userDiscountPrice(userID, 10, 0, "120"),
		userDiscountPrice(userID, 10, 0, "-80"),
		{UserID: userID, ProductID: 10, PriceAmount: money.FromDecimal(decimal.NewFromInt(80)), DiscountRate: money.FromDecimal(decimal.NewFromInt(80))},
		{UserID: userID, ProductID: 10},
	} {
		if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{bad}); !errors.Is(err, memberlevelcontract.ErrUserPriceInvalid) {
			t.Fatalf("expected ErrUserPriceInvalid for %+v, got %v", bad, err)
		}
	}

	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID, 10, 0, "95")}); err != nil {
		t.Fatalf("fixed upsert failed: %v", err)
	}
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userDiscountPrice(userID, 10, 0, "80")}); err != nil {
		t.Fatalf("discount upsert failed: %v", err)
	}
	rows, total, err := svc.ListUserPrices(userID, 1, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("list = (%d rows, total %d, %v), want exactly one row", len(rows), total, err)
	}
	if !rows[0].PriceAmount.Decimal.IsZero() {
		t.Fatalf("switching to discount must clear price_amount, got %s", rows[0].PriceAmount.String())
	}
	if rows[0].DiscountRate.String() != "80.00" {
		t.Fatalf("discount rate = %s, want 80.00", rows[0].DiscountRate.String())
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
	book, err := svc.LoadUserPriceBook(userID)
	if err != nil {
		t.Fatalf("load book failed: %v", err)
	}
	if _, ok := book.Resolve(10, 0, decimal.NewFromInt(100)); ok {
		t.Fatal("deleted price must not resolve")
	}
	if err := svc.BatchUpsertUserPrices([]memberleveldomain.UserProductPrice{userPrice(userID, 10, 0, "90")}); err != nil {
		t.Fatalf("re-upsert after soft delete failed (unique key collision?): %v", err)
	}
	book, err = svc.LoadUserPriceBook(userID)
	if err != nil {
		t.Fatalf("reload book failed: %v", err)
	}
	if got, ok := book.Resolve(10, 0, decimal.NewFromInt(100)); !ok || !got.Equal(decimal.NewFromInt(90)) {
		t.Fatalf("revived price = (%s, %v), want (90, true)", got, ok)
	}
}
