package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	resellergormstore "github.com/dujiao-next/internal/modules/reseller/infrastructure/gormstore"

	resellerapplication "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/money"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/config"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func openResellerManagementServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:reseller_management_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &admindomain.Admin{}, &resellerdomain.Profile{}, &resellerdomain.Domain{}, &resellerdomain.SiteConfig{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	return db
}

func seedResellerManagementUser(t *testing.T, db *gorm.DB, email string) userdomain.User {
	t.Helper()
	user := userdomain.User{Email: email, PasswordHash: "hash", DisplayName: email}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	return user
}

func newResellerManagementServiceForTest(db *gorm.DB) *ResellerManagementService {
	return NewResellerManagementService(resellergormstore.New(db), config.ResellerConfig{
		Enabled:          true,
		SelfApplyEnabled: true,
		SubdomainBase:    "shop.example.test",
		MainHosts:        []string{"main.example.test"},
	})
}

func TestResellerManagementUpdateUserDefaultMarkup(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	svc := newResellerManagementServiceForTest(db)
	owner := seedResellerManagementUser(t, db, "markup-owner@example.test")
	other := seedResellerManagementUser(t, db, "markup-other@example.test")
	pending := seedResellerManagementUser(t, db, "markup-pending@example.test")
	reviewer := uint(17)
	profiles := []resellerdomain.Profile{
		{UserID: owner.ID, Status: resellerdomain.ProfileStatusActive, MaxMarkupPercent: money.FromDecimal(decimal.NewFromInt(25)), SettlementStatus: resellerdomain.SettlementStatusFrozen, ReviewedBy: &reviewer},
		{UserID: other.ID, Status: resellerdomain.ProfileStatusActive, DefaultMarkupPercent: money.FromDecimal(decimal.NewFromInt(7))},
		{UserID: pending.ID, Status: resellerdomain.ProfileStatusPendingReview},
	}
	if err := db.Create(&profiles).Error; err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		userID uint
		value  string
		want   error
	}{
		{owner.ID, "25.01", resellercontract.ErrMarkupExceeded},
		{owner.ID, "-1", resellercontract.ErrDefaultMarkupInvalid},
		{owner.ID, "2.123", resellercontract.ErrDefaultMarkupInvalid},
		{owner.ID, "1e3", resellercontract.ErrDefaultMarkupInvalid},
		{owner.ID, "100000000", resellercontract.ErrDefaultMarkupInvalid},
		{pending.ID, "20", resellercontract.ErrProfileInactive},
		{pending.ID + 1, "20", resellercontract.ErrNotOpened},
	} {
		if _, err := svc.UpdateUserDefaultMarkup(tc.userID, tc.value); !errors.Is(err, tc.want) {
			t.Fatalf("user %d markup %q: got %v, want %v", tc.userID, tc.value, err, tc.want)
		}
	}

	profile, err := svc.UpdateUserDefaultMarkup(owner.ID, "25.00")
	if err != nil {
		t.Fatal(err)
	}
	if profile.DefaultMarkupPercent.String() != "25.00" || profile.MaxMarkupPercent.String() != "25.00" ||
		profile.SettlementStatus != resellerdomain.SettlementStatusFrozen || profile.ReviewedBy == nil || *profile.ReviewedBy != reviewer {
		t.Fatalf("unexpected updated profile: %+v", profile)
	}
	unit, _, err := resellerapplication.ResolveUnitAmount(profile, nil, nil, decimal.NewFromInt(100))
	if err != nil || !unit.Equal(decimal.NewFromInt(125)) {
		t.Fatalf("expected updated default to price at 125, got %s, %v", unit, err)
	}
	otherProfile, err := resellergormstore.New(db).GetProfileByUserID(other.ID)
	if err != nil || otherProfile.DefaultMarkupPercent.String() != "7.00" {
		t.Fatalf("other reseller was affected: %+v, %v", otherProfile, err)
	}
	if profile, err := svc.UpdateUserDefaultMarkup(other.ID, "42.75"); err != nil || profile.DefaultMarkupPercent.String() != "42.75" {
		t.Fatalf("zero cap should allow a positive markup: %+v, %v", profile, err)
	}
	if profile, err := svc.UpdateUserDefaultMarkup(owner.ID, "0"); err != nil || profile.DefaultMarkupPercent.String() != "0.00" {
		t.Fatalf("zero should reset markup: %+v, %v", profile, err)
	}
}

func TestResellerManagementApplyCreatesPendingAndReapplyRejected(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "apply-reseller@example.test")
	svc := newResellerManagementServiceForTest(db)

	profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "first application"})
	if err != nil {
		t.Fatalf("ApplyUserReseller failed: %v", err)
	}
	if profile.Status != resellerdomain.ProfileStatusPendingReview || profile.ApplyReason != "first application" {
		t.Fatalf("unexpected created profile: %+v", profile)
	}

	profile.Status = resellerdomain.ProfileStatusRejected
	profile.RejectReason = "missing info"
	reviewer := uint(7)
	now := time.Now()
	profile.ReviewedBy = &reviewer
	profile.ReviewedAt = &now
	if err := db.Save(profile).Error; err != nil {
		t.Fatalf("save rejected profile failed: %v", err)
	}

	reapplied, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "second application"})
	if err != nil {
		t.Fatalf("reapply failed: %v", err)
	}
	if reapplied.Status != resellerdomain.ProfileStatusPendingReview || reapplied.ApplyReason != "second application" || reapplied.RejectReason != "" || reapplied.ReviewedBy != nil || reapplied.ReviewedAt != nil {
		t.Fatalf("unexpected reapplied profile: %+v", reapplied)
	}
}

func TestResellerManagementApplyDisabledConfigRejects(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "apply-disabled@example.test")
	svc := NewResellerManagementService(resellergormstore.New(db), config.ResellerConfig{Enabled: false, SelfApplyEnabled: true})

	_, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "want access"})
	if !errors.Is(err, ErrResellerApplyDisabled) {
		t.Fatalf("expected ErrResellerApplyDisabled, got %v", err)
	}
}

func TestResellerManagementApproveActivatesProfileWithoutAutoSubdomain(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "approve-reseller@example.test")
	svc := newResellerManagementServiceForTest(db)
	profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "approve me"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	approved, err := svc.ApproveProfile(context.Background(), 9, profile.ID, ResellerApproveInput{
		DefaultMarkupPercent: decimal.NewFromInt(12),
		MaxMarkupPercent:     decimal.NewFromInt(45),
	})
	if err != nil {
		t.Fatalf("ApproveProfile failed: %v", err)
	}
	if approved.Profile.Status != resellerdomain.ProfileStatusActive || approved.Profile.ReviewedBy == nil || *approved.Profile.ReviewedBy != 9 {
		t.Fatalf("unexpected approved profile: %+v", approved.Profile)
	}
	if approved.SystemDomain != nil {
		t.Fatalf("expected approval to skip automatic system domain, got %+v", approved.SystemDomain)
	}
	var count int64
	if err := db.Model(&resellerdomain.Domain{}).Where("reseller_id = ?", profile.ID).Count(&count).Error; err != nil {
		t.Fatalf("count domains failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no domains after approval, got %d", count)
	}
}

func TestResellerManagementRejectsDefaultMarkupAboveMaxMarkup(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "markup-limit@example.test")
	svc := newResellerManagementServiceForTest(db)
	profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "approve me"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	_, err = svc.ApproveProfile(context.Background(), 9, profile.ID, ResellerApproveInput{
		DefaultMarkupPercent: decimal.NewFromInt(30),
		MaxMarkupPercent:     decimal.NewFromInt(10),
	})
	if !errors.Is(err, ErrResellerProfileStatusInvalid) {
		t.Fatalf("expected ErrResellerProfileStatusInvalid on approve, got %v", err)
	}

	approved, err := svc.ApproveProfile(context.Background(), 9, profile.ID, ResellerApproveInput{
		DefaultMarkupPercent: decimal.NewFromInt(10),
		MaxMarkupPercent:     decimal.NewFromInt(30),
	})
	if err != nil {
		t.Fatalf("approve valid profile failed: %v", err)
	}
	_, err = svc.UpdateProfileOperationalConfig(9, approved.Profile.ID, ResellerProfileUpdateInput{
		DefaultMarkupPercent: decimal.NewFromInt(40),
		MaxMarkupPercent:     decimal.NewFromInt(30),
		SettlementStatus:     resellerdomain.SettlementStatusNormal,
	})
	if !errors.Is(err, ErrResellerProfileStatusInvalid) {
		t.Fatalf("expected ErrResellerProfileStatusInvalid on update, got %v", err)
	}
}

func TestResellerManagementAssignSystemSubdomainCreatesAndEditsReadableDomain(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "assign-subdomain@example.test")
	svc := newResellerManagementServiceForTest(db)
	profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "approve me"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if _, err := svc.ApproveProfile(context.Background(), 9, profile.ID, ResellerApproveInput{}); err != nil {
		t.Fatalf("approve profile failed: %v", err)
	}

	created, err := svc.AssignSystemSubdomain(context.Background(), 9, profile.ID, ResellerSystemDomainInput{Subdomain: "Hello"})
	if err != nil {
		t.Fatalf("AssignSystemSubdomain create failed: %v", err)
	}
	if created.Domain != "hello.shop.example.test" || created.Type != resellerdomain.DomainTypeSubdomain || created.Status != resellerdomain.DomainStatusActive || created.VerificationStatus != resellerdomain.DomainVerificationVerified || !created.IsPrimary || created.VerifiedAt == nil {
		t.Fatalf("unexpected created system domain: %+v", created)
	}

	updated, err := svc.AssignSystemSubdomain(context.Background(), 9, profile.ID, ResellerSystemDomainInput{Subdomain: "brand.shop.example.test"})
	if err != nil {
		t.Fatalf("AssignSystemSubdomain update failed: %v", err)
	}
	if updated.ID != created.ID || updated.Domain != "brand.shop.example.test" || !updated.IsPrimary {
		t.Fatalf("unexpected updated system domain: created=%+v updated=%+v", created, updated)
	}
	var domains []resellerdomain.Domain
	if err := db.Where("reseller_id = ? AND type = ?", profile.ID, resellerdomain.DomainTypeSubdomain).Find(&domains).Error; err != nil {
		t.Fatalf("list system domains failed: %v", err)
	}
	if len(domains) != 1 {
		t.Fatalf("expected one editable system domain, got %+v", domains)
	}
}

func TestResellerManagementSubmitCustomDomainRequiresActiveProfile(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "custom-domain@example.test")
	svc := newResellerManagementServiceForTest(db)
	if _, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "pending"}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	_, err := svc.SubmitUserCustomDomain(user.ID, "shop.customer.example")
	if !errors.Is(err, ErrResellerProfileInactive) {
		t.Fatalf("expected ErrResellerProfileInactive, got %v", err)
	}
}

func TestResellerManagementSubmitAndApproveCustomDomain(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "domain-approve@example.test")
	svc := newResellerManagementServiceForTest(db)
	profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "approve me"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if _, err := svc.ApproveProfile(context.Background(), 9, profile.ID, ResellerApproveInput{}); err != nil {
		t.Fatalf("approve profile failed: %v", err)
	}

	domain, err := svc.SubmitUserCustomDomain(user.ID, "Shop.Customer.Example:443")
	if err != nil {
		t.Fatalf("SubmitUserCustomDomain failed: %v", err)
	}
	if domain.Domain != "shop.customer.example" || domain.Type != resellerdomain.DomainTypeCustom || domain.VerificationStatus != resellerdomain.DomainVerificationPending || domain.Status != resellerdomain.DomainStatusPendingReview || domain.VerificationToken != "" {
		t.Fatalf("unexpected submitted domain: %+v", domain)
	}

	approved, err := svc.ApproveDomain(context.Background(), 9, domain.ID)
	if err != nil {
		t.Fatalf("ApproveDomain failed: %v", err)
	}
	if approved.Status != resellerdomain.DomainStatusActive || approved.VerificationStatus != resellerdomain.DomainVerificationVerified || approved.VerifiedAt == nil {
		t.Fatalf("unexpected approved domain: %+v", approved)
	}
}

func TestResellerManagementDisablePrimaryDomainPromotesNextActiveVerifiedDomain(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	user := seedResellerManagementUser(t, db, "disable-primary@example.test")
	svc := newResellerManagementServiceForTest(db)
	profile, err := svc.ApplyUserReseller(user.ID, ResellerApplyInput{Reason: "approve me"})
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	approved, err := svc.ApproveProfile(context.Background(), 9, profile.ID, ResellerApproveInput{})
	if err != nil {
		t.Fatalf("approve profile failed: %v", err)
	}
	now := time.Now()
	primary := resellerdomain.Domain{
		ResellerID:         approved.Profile.ID,
		Domain:             "primary.shop.example.test",
		Type:               resellerdomain.DomainTypeSubdomain,
		Status:             resellerdomain.DomainStatusActive,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
		IsPrimary:          true,
		VerifiedAt:         &now,
	}
	secondary := resellerdomain.Domain{
		ResellerID:         approved.Profile.ID,
		Domain:             "secondary.shop.example.test",
		Type:               resellerdomain.DomainTypeSubdomain,
		Status:             resellerdomain.DomainStatusActive,
		VerificationStatus: resellerdomain.DomainVerificationVerified,
		IsPrimary:          false,
		VerifiedAt:         &now,
	}
	if err := db.Create(&primary).Error; err != nil {
		t.Fatalf("create primary domain failed: %v", err)
	}
	if err := db.Create(&secondary).Error; err != nil {
		t.Fatalf("create secondary domain failed: %v", err)
	}

	disabled, err := svc.DisableDomain(context.Background(), 9, primary.ID)
	if err != nil {
		t.Fatalf("disable primary domain failed: %v", err)
	}
	if disabled.IsPrimary {
		t.Fatalf("disabled domain should not remain primary: %+v", disabled)
	}
	var loadedSecondary resellerdomain.Domain
	if err := db.First(&loadedSecondary, secondary.ID).Error; err != nil {
		t.Fatalf("load secondary domain failed: %v", err)
	}
	if !loadedSecondary.IsPrimary {
		t.Fatalf("expected active verified secondary domain to become primary: %+v", loadedSecondary)
	}
}
