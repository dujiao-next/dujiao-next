package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newMemberLevelServiceForTest(t *testing.T) (*MemberLevelService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:member_level_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.MemberLevel{}, &models.MemberLevelPrice{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	levelRepo := repository.NewMemberLevelRepository(db)
	priceRepo := repository.NewMemberLevelPriceRepository(db)
	userRepo := repository.NewUserRepository(db)
	return NewMemberLevelService(levelRepo, priceRepo, userRepo), db
}

func createMemberLevelFixture(
	t *testing.T,
	db *gorm.DB,
	slug string,
	sortOrder int,
	spendThreshold string,
	isDefault bool,
) models.MemberLevel {
	t.Helper()

	level := models.MemberLevel{
		NameJSON: models.JSON{
			"zh-CN": slug,
		},
		Slug:              slug,
		DiscountRate:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		RechargeThreshold: models.NewMoneyFromDecimal(decimal.Zero),
		SpendThreshold:    models.NewMoneyFromDecimal(decimal.RequireFromString(spendThreshold)),
		IsDefault:         isDefault,
		SortOrder:         sortOrder,
		IsActive:          true,
	}
	if err := db.Create(&level).Error; err != nil {
		t.Fatalf("create member level fixture failed: %v", err)
	}
	return level
}

func createUserFixture(t *testing.T, db *gorm.DB, email string, memberLevelID uint) models.User {
	t.Helper()

	user := models.User{
		Email:          email,
		PasswordHash:   "test-hash",
		Status:         "active",
		MemberLevelID:  memberLevelID,
		TotalRecharged: models.NewMoneyFromDecimal(decimal.Zero),
		TotalSpent:     models.NewMoneyFromDecimal(decimal.Zero),
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user fixture failed: %v", err)
	}
	return user
}

func TestMemberLevelServiceOnOrderPaidDoesNotMoveToEqualSortOrder(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	defaultLevel := createMemberLevelFixture(t, db, "default", 0, "0", true)
	_ = createMemberLevelFixture(t, db, "vip", 0, "0.01", false)
	user := createUserFixture(t, db, "equal-sort@example.com", defaultLevel.ID)

	if err := svc.OnOrderPaid(user.ID, decimal.RequireFromString("0.01")); err != nil {
		t.Fatalf("OnOrderPaid failed: %v", err)
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("fetch updated user failed: %v", err)
	}
	if updated.MemberLevelID != defaultLevel.ID {
		t.Fatalf("expected keep member_level_id=%d, got %d", defaultLevel.ID, updated.MemberLevelID)
	}
	if !updated.TotalSpent.Decimal.Equal(decimal.RequireFromString("0.01")) {
		t.Fatalf("expected total_spent=0.01, got %s", updated.TotalSpent.Decimal.String())
	}
}

func TestMemberLevelServiceOnOrderPaidKeepsHigherLevel(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	highLevel := createMemberLevelFixture(t, db, "high", 100, "0", true)
	_ = createMemberLevelFixture(t, db, "low", 10, "0.01", false)
	user := createUserFixture(t, db, "no-downgrade@example.com", highLevel.ID)

	if err := svc.OnOrderPaid(user.ID, decimal.RequireFromString("50")); err != nil {
		t.Fatalf("OnOrderPaid failed: %v", err)
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("fetch updated user failed: %v", err)
	}
	if updated.MemberLevelID != highLevel.ID {
		t.Fatalf("expected keep member_level_id=%d, got %d", highLevel.ID, updated.MemberLevelID)
	}
}

func TestMemberLevelServiceOnOrderPaidUpgradesToHigherSortOrder(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	defaultLevel := createMemberLevelFixture(t, db, "default2", 0, "0", true)
	goldLevel := createMemberLevelFixture(t, db, "gold", 20, "0.01", false)
	user := createUserFixture(t, db, "higher-sort@example.com", defaultLevel.ID)

	if err := svc.OnOrderPaid(user.ID, decimal.RequireFromString("0.01")); err != nil {
		t.Fatalf("OnOrderPaid failed: %v", err)
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("fetch updated user failed: %v", err)
	}
	if updated.MemberLevelID != goldLevel.ID {
		t.Fatalf("expected member_level_id=%d, got %d", goldLevel.ID, updated.MemberLevelID)
	}
}

type concurrentUserRepository struct {
	base                     repository.UserRepository
	afterFirstSpendIncrement func()
	spendIncrementCalled     bool
}

func (r *concurrentUserRepository) GetByEmail(email string) (*models.User, error) {
	return r.base.GetByEmail(email)
}

func (r *concurrentUserRepository) GetByID(id uint) (*models.User, error) {
	return r.base.GetByID(id)
}

func (r *concurrentUserRepository) ListByIDs(ids []uint) ([]models.User, error) {
	return r.base.ListByIDs(ids)
}

func (r *concurrentUserRepository) Create(user *models.User) error {
	return r.base.Create(user)
}

func (r *concurrentUserRepository) Update(user *models.User) error {
	return r.base.Update(user)
}

func (r *concurrentUserRepository) IncrementTotalRecharged(userID uint, amount decimal.Decimal) error {
	return r.base.IncrementTotalRecharged(userID, amount)
}

func (r *concurrentUserRepository) IncrementTotalSpent(userID uint, amount decimal.Decimal) error {
	if err := r.base.IncrementTotalSpent(userID, amount); err != nil {
		return err
	}
	if !r.spendIncrementCalled && r.afterFirstSpendIncrement != nil {
		r.spendIncrementCalled = true
		r.afterFirstSpendIncrement()
	}
	return nil
}

func (r *concurrentUserRepository) UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID uint) (int64, error) {
	return r.base.UpdateMemberLevelIfCurrent(userID, currentLevelID, nextLevelID)
}

func (r *concurrentUserRepository) List(filter repository.UserListFilter) ([]models.User, int64, error) {
	return r.base.List(filter)
}

func (r *concurrentUserRepository) BatchUpdateStatus(userIDs []uint, status string) error {
	return r.base.BatchUpdateStatus(userIDs, status)
}

func (r *concurrentUserRepository) AssignDefaultMemberLevel(defaultLevelID uint) (int64, error) {
	return r.base.AssignDefaultMemberLevel(defaultLevelID)
}

func (r *concurrentUserRepository) UpdateTOTPPending(userID uint, encSecret string, expiresAt time.Time) error {
	return r.base.UpdateTOTPPending(userID, encSecret, expiresAt)
}

func (r *concurrentUserRepository) UpdateTOTPEnabled(userID uint, encSecret string, enabledAt time.Time, recoveryCodesJSON string) error {
	return r.base.UpdateTOTPEnabled(userID, encSecret, enabledAt, recoveryCodesJSON)
}

func (r *concurrentUserRepository) UpdateRecoveryCodes(userID uint, recoveryCodesJSON string) error {
	return r.base.UpdateRecoveryCodes(userID, recoveryCodesJSON)
}

func (r *concurrentUserRepository) ClearTOTP(userID uint) error {
	return r.base.ClearTOTP(userID)
}

func (r *concurrentUserRepository) CountByMemberLevelIDs(levelIDs []uint) (map[uint]int64, error) {
	return r.base.CountByMemberLevelIDs(levelIDs)
}

func TestMemberLevelServiceOnOrderPaidDoesNotOverwriteConcurrentHigherLevel(t *testing.T) {
	_, db := newMemberLevelServiceForTest(t)
	defaultLevel := createMemberLevelFixture(t, db, "race-default", 0, "0", true)
	goldLevel := createMemberLevelFixture(t, db, "race-gold", 20, "1.00", false)
	highLevel := createMemberLevelFixture(t, db, "race-high", 100, "1000.00", false)
	user := createUserFixture(t, db, "race-no-downgrade@example.com", defaultLevel.ID)

	baseUserRepo := repository.NewUserRepository(db)
	raceRepo := &concurrentUserRepository{
		base: baseUserRepo,
		afterFirstSpendIncrement: func() {
			if err := db.Model(&models.User{}).Where("id = ?", user.ID).Update("member_level_id", highLevel.ID).Error; err != nil {
				t.Fatalf("simulate concurrent higher level update failed: %v", err)
			}
		},
	}
	svc := NewMemberLevelService(
		repository.NewMemberLevelRepository(db),
		repository.NewMemberLevelPriceRepository(db),
		raceRepo,
	)

	if err := svc.OnOrderPaid(user.ID, decimal.RequireFromString("1.00")); err != nil {
		t.Fatalf("OnOrderPaid failed: %v", err)
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("fetch updated user failed: %v", err)
	}
	if updated.MemberLevelID == goldLevel.ID {
		t.Fatalf("expected concurrent high level to be preserved, got auto-upgraded lower level %d", updated.MemberLevelID)
	}
	if updated.MemberLevelID != highLevel.ID {
		t.Fatalf("expected member_level_id=%d, got %d", highLevel.ID, updated.MemberLevelID)
	}
	if !updated.TotalSpent.Decimal.Equal(decimal.RequireFromString("1.00")) {
		t.Fatalf("expected total_spent=1.00, got %s", updated.TotalSpent.Decimal.String())
	}
}

func TestMemberLevelServiceCreateLevelRejectsActiveSortOrderConflict(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	_ = createMemberLevelFixture(t, db, "sort-existing", 10, "0", true)

	err := svc.CreateLevel(&models.MemberLevel{
		NameJSON:          models.JSON{"zh-CN": "sort-conflict"},
		Slug:              "sort-conflict",
		DiscountRate:      models.NewMoneyFromDecimal(decimal.NewFromInt(100)),
		RechargeThreshold: models.NewMoneyFromDecimal(decimal.Zero),
		SpendThreshold:    models.NewMoneyFromDecimal(decimal.NewFromInt(1)),
		IsDefault:         false,
		SortOrder:         10,
		IsActive:          true,
	})
	if err == nil {
		t.Fatalf("expected active sort_order conflict to be rejected")
	}
}

func TestMemberLevelServiceUpdateLevelRejectsActiveSortOrderConflict(t *testing.T) {
	svc, db := newMemberLevelServiceForTest(t)
	_ = createMemberLevelFixture(t, db, "sort-update-existing", 10, "0", true)
	target := createMemberLevelFixture(t, db, "sort-update-target", 20, "1.00", false)

	target.SortOrder = 10
	err := svc.UpdateLevel(&target)
	if err == nil {
		t.Fatalf("expected active sort_order conflict to be rejected")
	}
}
