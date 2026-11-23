package integrationtest

import (
	"errors"
	"fmt"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
	"testing"
	"time"

	affiliateapp "github.com/dujiao-next/internal/modules/affiliate/application"
	affiliatecontract "github.com/dujiao-next/internal/modules/affiliate/contract"
	affiliatedomain "github.com/dujiao-next/internal/modules/affiliate/domain"
	affiliategormstore "github.com/dujiao-next/internal/modules/affiliate/infrastructure/gormstore"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/testkit/memorysettings"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"

	"github.com/dujiao-next/internal/constants"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestResolveOrderAffiliateSnapshotPreferLatestVisitorClick(t *testing.T) {
	svc, db, _ := setupAffiliateServiceTest(t)

	promoterA := createAffiliateTestUser(t, db, "affiliate-a@example.com")
	promoterB := createAffiliateTestUser(t, db, "affiliate-b@example.com")
	profileA := createAffiliateTestProfile(t, db, promoterA.ID, "AFFA0001", constants.AffiliateProfileStatusActive)
	profileB := createAffiliateTestProfile(t, db, promoterB.ID, "AFFB0002", constants.AffiliateProfileStatusActive)

	visitorKey := "visitor-key-priority"
	now := time.Now()
	createAffiliateTestClick(t, db, profileA.ID, visitorKey, now.Add(-2*time.Hour))
	createAffiliateTestClick(t, db, profileB.ID, visitorKey, now.Add(-1*time.Hour))

	profileID, code, err := svc.ResolveOrderAffiliateSnapshot(0, profileA.AffiliateCode, visitorKey)
	if err != nil {
		t.Fatalf("resolve snapshot failed: %v", err)
	}
	if profileID == nil || *profileID != profileB.ID {
		t.Fatalf("expected latest clicked profile %d, got %+v", profileB.ID, profileID)
	}
	if code != profileB.AffiliateCode {
		t.Fatalf("expected latest clicked code %s, got %s", profileB.AffiliateCode, code)
	}
}

func TestResolveOrderAffiliateSnapshotFallbackToCodeWhenNoVisitorClick(t *testing.T) {
	svc, db, _ := setupAffiliateServiceTest(t)

	promoter := createAffiliateTestUser(t, db, "affiliate-fallback@example.com")
	profile := createAffiliateTestProfile(t, db, promoter.ID, "AFFF0003", constants.AffiliateProfileStatusActive)

	profileID, code, err := svc.ResolveOrderAffiliateSnapshot(0, profile.AffiliateCode, "visitor-key-not-found")
	if err != nil {
		t.Fatalf("resolve snapshot failed: %v", err)
	}
	if profileID == nil || *profileID != profile.ID {
		t.Fatalf("expected fallback profile %d, got %+v", profile.ID, profileID)
	}
	if code != profile.AffiliateCode {
		t.Fatalf("expected fallback code %s, got %s", profile.AffiliateCode, code)
	}
}

func TestResolveOrderAffiliateSnapshotRejectSelfByVisitorClick(t *testing.T) {
	svc, db, _ := setupAffiliateServiceTest(t)

	promoter := createAffiliateTestUser(t, db, "affiliate-self@example.com")
	profile := createAffiliateTestProfile(t, db, promoter.ID, "AFFS0004", constants.AffiliateProfileStatusActive)
	createAffiliateTestClick(t, db, profile.ID, "visitor-key-self", time.Now().Add(-10*time.Minute))

	profileID, code, err := svc.ResolveOrderAffiliateSnapshot(promoter.ID, "AFFF9999", "visitor-key-self")
	if err != nil {
		t.Fatalf("resolve snapshot failed: %v", err)
	}
	if profileID != nil || code != "" {
		t.Fatalf("expected self-order attribution ignored, got profile=%+v code=%q", profileID, code)
	}
}

func TestUpdateAffiliateProfileStatus(t *testing.T) {
	svc, db, _ := setupAffiliateServiceTest(t)

	user := createAffiliateTestUser(t, db, "affiliate-status@example.com")
	profile := createAffiliateTestProfile(t, db, user.ID, "AFFST001", constants.AffiliateProfileStatusActive)

	disabled, err := svc.UpdateAffiliateProfileStatus(profile.ID, constants.AffiliateProfileStatusDisabled)
	if err != nil {
		t.Fatalf("disable profile failed: %v", err)
	}
	if disabled == nil || disabled.Status != constants.AffiliateProfileStatusDisabled {
		t.Fatalf("expected disabled status, got %+v", disabled)
	}

	enabled, err := svc.UpdateAffiliateProfileStatus(profile.ID, constants.AffiliateProfileStatusActive)
	if err != nil {
		t.Fatalf("enable profile failed: %v", err)
	}
	if enabled == nil || enabled.Status != constants.AffiliateProfileStatusActive {
		t.Fatalf("expected active status, got %+v", enabled)
	}
}

func TestBatchUpdateAffiliateProfileStatus(t *testing.T) {
	svc, db, store := setupAffiliateServiceTest(t)

	userA := createAffiliateTestUser(t, db, "affiliate-batch-a@example.com")
	userB := createAffiliateTestUser(t, db, "affiliate-batch-b@example.com")
	profileA := createAffiliateTestProfile(t, db, userA.ID, "AFFBT001", constants.AffiliateProfileStatusActive)
	profileB := createAffiliateTestProfile(t, db, userB.ID, "AFFBT002", constants.AffiliateProfileStatusActive)

	updated, err := svc.BatchUpdateAffiliateProfileStatus([]uint{profileA.ID, profileB.ID}, constants.AffiliateProfileStatusDisabled)
	if err != nil {
		t.Fatalf("batch disable failed: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected updated 2, got %d", updated)
	}

	reloadedA, err := store.GetProfileByID(profileA.ID)
	if err != nil || reloadedA == nil {
		t.Fatalf("reload profileA failed: %v", err)
	}
	reloadedB, err := store.GetProfileByID(profileB.ID)
	if err != nil || reloadedB == nil {
		t.Fatalf("reload profileB failed: %v", err)
	}
	if reloadedA.Status != constants.AffiliateProfileStatusDisabled || reloadedB.Status != constants.AffiliateProfileStatusDisabled {
		t.Fatalf("unexpected statuses after batch disable: %s, %s", reloadedA.Status, reloadedB.Status)
	}
}

// 看板必须带出推广账号状态，前台据此隐藏停用账号的推广入口。
func TestGetUserDashboardExposesProfileStatus(t *testing.T) {
	svc, db, _ := setupAffiliateServiceTest(t)
	if err := db.AutoMigrate(&affiliatedomain.Commission{}, &affiliatedomain.Click{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	for _, status := range []string{constants.AffiliateProfileStatusActive, constants.AffiliateProfileStatusDisabled} {
		user := createAffiliateTestUser(t, db, status+"@example.com")
		createAffiliateTestProfile(t, db, user.ID, "ST"+status[:3], status)
		dashboard, err := svc.GetUserDashboard(user.ID)
		if err != nil {
			t.Fatalf("GetUserDashboard(%s): %v", status, err)
		}
		if !dashboard.Opened || dashboard.Status != status {
			t.Fatalf("status %s: opened=%v status=%q", status, dashboard.Opened, dashboard.Status)
		}
	}
}

// 停用的推广账号不能提现，且不能因此冻结或消耗已有的可提现佣金；启用账号作为对照必须成功。
func TestApplyWithdrawRejectsDisabledProfile(t *testing.T) {
	svc, db, _ := setupAffiliateServiceTest(t)
	if err := db.AutoMigrate(&affiliatedomain.Commission{}, &affiliatedomain.WithdrawRequest{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	seed := func(email, code, status string, orderID uint) (userID uint, commissionID uint) {
		user := createAffiliateTestUser(t, db, email)
		profile := createAffiliateTestProfile(t, db, user.ID, code, status)
		commission := affiliatedomain.Commission{
			AffiliateProfileID: profile.ID, OrderID: orderID, CommissionType: constants.AffiliateCommissionTypeOrder,
			CommissionAmount: money.FromDecimal(decimal.NewFromInt(50)),
			Status:           constants.AffiliateCommissionStatusAvailable,
		}
		if err := db.Create(&commission).Error; err != nil {
			t.Fatalf("seed commission: %v", err)
		}
		return user.ID, commission.ID
	}
	input := affiliateapp.WithdrawApplyInput{Amount: decimal.NewFromInt(10), Channel: "usdt", Account: "acct"}

	disabledUser, disabledCommission := seed("disabled-w@example.com", "WDDIS001", constants.AffiliateProfileStatusDisabled, 1)
	if _, err := svc.ApplyWithdraw(disabledUser, input); !errors.Is(err, affiliateapp.ErrNotOpened) {
		t.Fatalf("disabled profile withdraw: err=%v, want ErrNotOpened", err)
	}
	var locked affiliatedomain.Commission
	if err := db.First(&locked, disabledCommission).Error; err != nil {
		t.Fatalf("reload commission: %v", err)
	}
	if locked.WithdrawRequestID != nil || locked.Status != constants.AffiliateCommissionStatusAvailable || locked.CommissionAmount.String() != "50.00" {
		t.Fatalf("disabled withdraw must leave commission untouched: %+v", locked)
	}
	var requests int64
	if err := db.Model(&affiliatedomain.WithdrawRequest{}).Count(&requests).Error; err != nil || requests != 0 {
		t.Fatalf("disabled withdraw created %d requests (err=%v)", requests, err)
	}

	activeUser, _ := seed("active-w@example.com", "WDACT001", constants.AffiliateProfileStatusActive, 2)
	if _, err := svc.ApplyWithdraw(activeUser, input); err != nil {
		t.Fatalf("active profile withdraw should succeed: %v", err)
	}
}

func setupAffiliateServiceTest(t *testing.T) (*affiliateapp.Service, *gorm.DB, affiliatecontract.Store) {
	t.Helper()

	dsn := fmt.Sprintf("file:affiliate_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &affiliatedomain.Profile{}, &affiliatedomain.Click{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	settingRepo := memorysettings.New()
	settingSvc := settingsapp.NewService(settingRepo)
	if _, err := settingSvc.UpdateAffiliateSetting(settingsintegration.AffiliateSetting{
		Enabled:        true,
		CommissionRate: 20,
	}); err != nil {
		t.Fatalf("init affiliate setting failed: %v", err)
	}

	affiliateRepo := affiliategormstore.New(db)
	return affiliateapp.NewService(affiliateRepo, userstore.New(db), nil, nil, nil, settingSvc), db, affiliateRepo
}

func createAffiliateTestUser(t *testing.T, db *gorm.DB, email string) userdomain.User {
	t.Helper()

	row := userdomain.User{
		Email:        email,
		PasswordHash: "hash",
		DisplayName:  "tester",
		Status:       constants.UserStatusActive,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	return row
}

func createAffiliateTestProfile(t *testing.T, db *gorm.DB, userID uint, code, status string) affiliatedomain.Profile {
	t.Helper()

	row := affiliatedomain.Profile{
		UserID:        userID,
		AffiliateCode: code,
		Status:        status,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create affiliate profile failed: %v", err)
	}
	return row
}

func createAffiliateTestClick(t *testing.T, db *gorm.DB, profileID uint, visitorKey string, createdAt time.Time) {
	t.Helper()

	row := affiliatedomain.Click{
		AffiliateProfileID: profileID,
		VisitorKey:         visitorKey,
		LandingPath:        "/",
		CreatedAt:          createdAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("create affiliate click failed: %v", err)
	}
}
