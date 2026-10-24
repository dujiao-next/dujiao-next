package userstore

import (
	"fmt"
	"testing"
	"time"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func setupStoreTest(t *testing.T) (*Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:user_repository_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &walletdomain.Account{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return New(db), db
}

func createTestUser(t *testing.T, db *gorm.DB, email string, createdAt time.Time, lastLogin *time.Time) *userdomain.User {
	t.Helper()
	u := &userdomain.User{Email: email, CreatedAt: createdAt, LastLoginAt: lastLogin}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("create user %q failed: %v", email, err)
	}
	return u
}

func createTestWalletAccount(t *testing.T, db *gorm.DB, userID uint, balance int64) {
	t.Helper()
	w := &walletdomain.Account{UserID: userID, Balance: money.FromDecimal(decimal.NewFromInt(balance))}
	if err := db.Create(w).Error; err != nil {
		t.Fatalf("create wallet account for user %d failed: %v", userID, err)
	}
}

func timePtr(tm time.Time) *time.Time { return &tm }

func assertUserOrder(t *testing.T, rows []userdomain.User, want []uint) {
	t.Helper()
	if len(rows) != len(want) {
		t.Fatalf("expected %d rows, got %d", len(want), len(rows))
	}
	for i, id := range want {
		if rows[i].ID != id {
			gotIDs := make([]uint, len(rows))
			for j, r := range rows {
				gotIDs[j] = r.ID
			}
			t.Fatalf("order mismatch: want %v, got %v", want, gotIDs)
		}
	}
}

// 按注册时间升序：最早注册的排在最前
func TestStoreListSortByCreatedAtAsc(t *testing.T) {
	repo, db := setupStoreTest(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newest := createTestUser(t, db, "newest@x.com", base.Add(2*time.Hour), nil)
	middle := createTestUser(t, db, "middle@x.com", base.Add(1*time.Hour), nil)
	oldest := createTestUser(t, db, "oldest@x.com", base, nil)

	rows, total, err := repo.List(usercontract.ListFilter{SortBy: "created_at", SortOrder: "asc"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	assertUserOrder(t, rows, []uint{oldest.ID, middle.ID, newest.ID})
}

// 按最后登录时间降序：从未登录（NULL）的用户始终排在最后
func TestStoreListSortByLastLoginDescNullsLast(t *testing.T) {
	repo, db := setupStoreTest(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := createTestUser(t, db, "recent@x.com", base, timePtr(base.Add(2*time.Hour)))
	older := createTestUser(t, db, "older@x.com", base, timePtr(base.Add(1*time.Hour)))
	never := createTestUser(t, db, "never@x.com", base, nil)

	rows, _, err := repo.List(usercontract.ListFilter{SortBy: "last_login_at", SortOrder: "desc"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	assertUserOrder(t, rows, []uint{recent.ID, older.ID, never.ID})
}

// 按钱包余额降序：无钱包账户的用户按余额 0 处理并排在最后
func TestStoreListSortByWalletBalanceDesc(t *testing.T) {
	repo, db := setupStoreTest(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	rich := createTestUser(t, db, "rich@x.com", base, nil)
	poor := createTestUser(t, db, "poor@x.com", base, nil)
	noAccount := createTestUser(t, db, "none@x.com", base, nil)
	createTestWalletAccount(t, db, rich.ID, 100)
	createTestWalletAccount(t, db, poor.ID, 10)

	rows, _, err := repo.List(usercontract.ListFilter{SortBy: "wallet_balance", SortOrder: "desc"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	assertUserOrder(t, rows, []uint{rich.ID, poor.ID, noAccount.ID})
}

// 非法 sort_by 回退到默认排序（id 倒序）
func TestStoreListSortByInvalidFallsBackToIDDesc(t *testing.T) {
	repo, db := setupStoreTest(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	u1 := createTestUser(t, db, "u1@x.com", base, nil)
	u2 := createTestUser(t, db, "u2@x.com", base, nil)
	u3 := createTestUser(t, db, "u3@x.com", base, nil)

	rows, _, err := repo.List(usercontract.ListFilter{SortBy: "drop table users", SortOrder: "desc"})
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}
	assertUserOrder(t, rows, []uint{u3.ID, u2.ID, u1.ID})
}

func TestStoreSoftDeleteExcludesUserFromReadsListsAndMutations(t *testing.T) {
	store, db := setupStoreTest(t)
	user := &userdomain.User{
		Email:          "deleted@example.test",
		PasswordHash:   "hash",
		Status:         "active",
		TokenVersion:   3,
		TotalRecharged: money.FromDecimal(decimal.NewFromInt(10)),
	}
	if err := store.Create(user); err != nil {
		t.Fatalf("create: %v", err)
	}
	deletedAt := time.Now()
	if err := db.Model(&userdomain.User{}).Where("id = ?", user.ID).Update("deleted_at", deletedAt).Error; err != nil {
		t.Fatalf("soft delete: %v", err)
	}

	byID, err := store.GetByID(user.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if byID != nil {
		t.Fatalf("deleted user returned by id: %#v", byID)
	}
	byEmail, err := store.GetByEmail(user.Email)
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if byEmail != nil {
		t.Fatalf("deleted user returned by email: %#v", byEmail)
	}
	byIDs, err := store.ListByIDs([]uint{user.ID})
	if err != nil {
		t.Fatalf("list by ids: %v", err)
	}
	if len(byIDs) != 0 {
		t.Fatalf("deleted user returned by ids: %#v", byIDs)
	}
	listed, total, err := store.List(usercontract.ListFilter{UserID: user.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(listed) != 0 {
		t.Fatalf("deleted user returned from list: total=%d rows=%#v", total, listed)
	}

	if err := store.BatchUpdateStatus([]uint{user.ID}, "disabled"); err != nil {
		t.Fatalf("batch update deleted user: %v", err)
	}
	if err := store.ClearTOTP(user.ID); err != nil {
		t.Fatalf("clear deleted user TOTP: %v", err)
	}

	var persisted userdomain.User
	if err := db.Where("id = ?", user.ID).First(&persisted).Error; err != nil {
		t.Fatalf("load raw row: %v", err)
	}
	if persisted.Status != "active" || persisted.TokenVersion != 3 || persisted.TotalRecharged.String() != "10.00" {
		t.Fatalf("deleted user was mutated: %#v", persisted)
	}
}
