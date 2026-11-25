package integrationtest

import (
	"fmt"
	"testing"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	resellerstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:reseller_migration_%d?mode=memory&cache=shared", uniqueTestID())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &orderdomain.Order{}); err != nil {
		t.Fatalf("migrate base models failed: %v", err)
	}
	if err := resellerstore.Migrate(db); err != nil {
		t.Fatalf("migrate reseller models failed: %v", err)
	}
	return db
}

func TestMigrateCreatesResellerTablesAndOrderColumns(t *testing.T) {
	db := openMigrationDB(t)
	if !db.Migrator().HasTable(&resellerdomain.Profile{}) {
		t.Fatal("expected reseller_profiles table")
	}
	if !db.Migrator().HasTable(&resellerdomain.Domain{}) {
		t.Fatal("expected reseller_domains table")
	}
	if !db.Migrator().HasColumn(&orderdomain.Order{}, "reseller_id") {
		t.Fatal("expected orders.reseller_id column")
	}
	if !db.Migrator().HasColumn(&orderdomain.Order{}, "reseller_domain") {
		t.Fatal("expected orders.reseller_domain column")
	}
	if !db.Migrator().HasColumn(&orderdomain.Order{}, "reseller_profit_amount") {
		t.Fatal("expected orders.reseller_profit_amount column")
	}
}

func TestDomainActiveUniqueAllowsSoftDeleteRecreate(t *testing.T) {
	db := openMigrationDB(t)
	user := userdomain.User{Email: "reseller@example.com", PasswordHash: "x"}
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
	first := resellerdomain.Domain{
		ResellerID:         profile.ID,
		Domain:             "shop.example.test",
		Type:               resellerdomain.DomainTypeCustom,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
		Status:             resellerdomain.DomainStatusActive,
	}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("create first domain failed: %v", err)
	}
	now := time.Now()
	if err := db.Model(&resellerdomain.Domain{}).Where("id = ?", first.ID).Update("deleted_at", &now).Error; err != nil {
		t.Fatalf("soft delete domain failed: %v", err)
	}
	second := resellerdomain.Domain{
		ResellerID:         profile.ID,
		Domain:             "shop.example.test",
		Type:               resellerdomain.DomainTypeCustom,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
		Status:             resellerdomain.DomainStatusActive,
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("create second domain after soft delete failed: %v", err)
	}
}

func TestMoneyFieldsRoundTrip(t *testing.T) {
	db := openMigrationDB(t)
	amount := money.FromDecimal(decimal.RequireFromString("12.345"))
	entry := resellerdomain.LedgerEntry{
		ResellerID:     1,
		Type:           resellerdomain.LedgerTypeManualAdjust,
		Amount:         amount,
		Currency:       "CNY",
		IdempotencyKey: "manual:test:1",
		Status:         resellerdomain.LedgerStatusAvailable,
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create ledger failed: %v", err)
	}
	var got resellerdomain.LedgerEntry
	if err := db.First(&got, entry.ID).Error; err != nil {
		t.Fatalf("load ledger failed: %v", err)
	}
	if got.Amount.String() != "12.35" {
		t.Fatalf("amount should round to 12.35, got %s", got.Amount.String())
	}
}
