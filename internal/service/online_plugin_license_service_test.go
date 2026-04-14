package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newOnlinePluginLicenseServiceForTest(t *testing.T) (*OnlinePluginLicenseService, repository.PluginLicensePlatformRepository, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:online_plugin_license_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.PluginMarketCatalogPlugin{},
		&models.PluginMarketPlan{},
		&models.PluginLicense{},
		&models.PluginLicenseActivation{},
		&models.PluginLicenseHeartbeat{},
	); err != nil {
		t.Fatalf("auto migrate plugin license tables failed: %v", err)
	}

	repo := repository.NewPluginLicensePlatformRepository(db)
	svc := NewOnlinePluginLicenseService(repo)
	return svc, repo, db
}

func seedOnlinePluginLicenseCatalog(t *testing.T, db *gorm.DB, pluginID, planCode, licenseMode string, durationDays *int) {
	t.Helper()
	if err := db.Create(&models.PluginMarketCatalogPlugin{
		PluginID:    pluginID,
		Slug:        pluginID,
		PublisherID: 1,
		Name:        "测试插件",
		PluginType:  "feature",
		BillingMode: licenseMode,
		LicenseMode: licenseMode,
		Status:      "published",
		IsPublic:    true,
	}).Error; err != nil {
		t.Fatalf("seed plugin failed: %v", err)
	}
	if err := db.Create(&models.PluginMarketPlan{
		PluginID:       pluginID,
		PlanCode:       planCode,
		PlanName:       "默认套餐",
		BillingMode:    licenseMode,
		LicenseMode:    licenseMode,
		DurationDays:   durationDays,
		MaxSites:       1,
		MaxActivations: 1,
		Status:         "active",
	}).Error; err != nil {
		t.Fatalf("seed plan failed: %v", err)
	}
}

func TestOnlinePluginLicenseCreateActivateAndHeartbeat(t *testing.T) {
	svc, repo, db := newOnlinePluginLicenseServiceForTest(t)
	duration := 365
	seedOnlinePluginLicenseCatalog(t, db, "telegram-suite", "annual-basic", "annual", &duration)

	license, err := svc.CreateLicense(OnlinePluginLicenseAdminInput{
		PluginID:    "telegram-suite",
		PlanCode:    "annual-basic",
		LicenseMode: "annual",
		Status:      "pending",
	})
	if err != nil {
		t.Fatalf("create license failed: %v", err)
	}
	if license.ExpireAt == nil {
		t.Fatalf("annual license should auto fill expire_at")
	}

	activated, err := svc.ActivateLicense(OnlinePluginLicenseActivateInput{
		PluginID:        "telegram-suite",
		LicenseKey:      license.LicenseKey,
		InstallID:       "host-01",
		HostFingerprint: "fp-01",
		PrimaryDomain:   "shop.example.com",
		ServerIP:        "1.2.3.4",
		CurrentVersion:  "1.0.0",
	})
	if err != nil {
		t.Fatalf("activate license failed: %v", err)
	}
	if activated.ActivationToken == "" {
		t.Fatalf("activation token should not be empty")
	}
	if activated.EnforcementMode != "ok" {
		t.Fatalf("activation enforcement want ok got %s", activated.EnforcementMode)
	}
	if !activated.MatchedDomain || !activated.MatchedServerIP {
		t.Fatalf("activation should bind and match domain/ip")
	}

	heartbeat, err := svc.ReportHeartbeat(OnlinePluginLicenseHeartbeatInput{
		PluginID:        "telegram-suite",
		ActivationToken: activated.ActivationToken,
		InstallID:       "host-01",
		PrimaryDomain:   "shop.example.com",
		ServerIP:        "1.2.3.4",
		CurrentVersion:  "1.0.0",
		RuntimeLoaded:   true,
	})
	if err != nil {
		t.Fatalf("report heartbeat failed: %v", err)
	}
	if heartbeat.LicenseStatus != "active" {
		t.Fatalf("heartbeat license status want active got %s", heartbeat.LicenseStatus)
	}
	if heartbeat.EnforcementMode != "ok" {
		t.Fatalf("heartbeat enforcement want ok got %s", heartbeat.EnforcementMode)
	}

	detail, err := svc.GetLicenseDetail(license.LicenseID)
	if err != nil {
		t.Fatalf("get detail failed: %v", err)
	}
	if detail == nil || len(detail.RecentHeartbeats) < 2 {
		t.Fatalf("expected activation + heartbeat records, got %#v", detail)
	}
	if len(detail.Activations) != 1 {
		t.Fatalf("expected one activation, got %d", len(detail.Activations))
	}

	loaded, err := repo.GetLicenseByLicenseID(license.LicenseID)
	if err != nil {
		t.Fatalf("reload license failed: %v", err)
	}
	if loaded == nil || loaded.BoundDomain != "shop.example.com" || loaded.BoundServerIP != "1.2.3.4" {
		t.Fatalf("expected bound domain/ip saved, got %#v", loaded)
	}
}

func TestOnlinePluginLicenseActivateRejectsBindingConflict(t *testing.T) {
	svc, _, db := newOnlinePluginLicenseServiceForTest(t)
	seedOnlinePluginLicenseCatalog(t, db, "telegram-suite", "perpetual-basic", "perpetual", nil)

	first, err := svc.CreateLicense(OnlinePluginLicenseAdminInput{
		PluginID:      "telegram-suite",
		PlanCode:      "perpetual-basic",
		LicenseMode:   "perpetual",
		Status:        "active",
		BoundDomain:   "shop.example.com",
		BoundServerIP: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("create first license failed: %v", err)
	}
	if _, err := svc.CreateLicense(OnlinePluginLicenseAdminInput{
		PluginID:    "telegram-suite",
		PlanCode:    "perpetual-basic",
		LicenseMode: "perpetual",
		Status:      "pending",
	}); err != nil {
		t.Fatalf("create second license failed: %v", err)
	}

	second, _, err := svc.ListLicenses(OnlinePluginLicenseListFilter{Page: 1, PageSize: 20, PluginID: "telegram-suite"})
	if err != nil {
		t.Fatalf("list licenses failed: %v", err)
	}
	var target *models.PluginLicense
	for _, item := range second {
		if item.License != nil && item.License.LicenseID != first.LicenseID {
			target = item.License
			break
		}
	}
	if target == nil {
		t.Fatalf("second license not found in list")
	}

	_, err = svc.ActivateLicense(OnlinePluginLicenseActivateInput{
		PluginID:      "telegram-suite",
		LicenseKey:    target.LicenseKey,
		InstallID:     "host-02",
		PrimaryDomain: "shop.example.com",
		ServerIP:      "1.2.3.4",
	})
	if err == nil {
		t.Fatalf("expected binding conflict error")
	}
	if err != ErrOnlinePluginLicenseConflict {
		t.Fatalf("expected conflict error got %v", err)
	}
}

func TestOnlinePluginLicenseValidateDetectsMismatch(t *testing.T) {
	svc, _, db := newOnlinePluginLicenseServiceForTest(t)
	seedOnlinePluginLicenseCatalog(t, db, "telegram-suite", "perpetual-basic", "perpetual", nil)

	license, err := svc.CreateLicense(OnlinePluginLicenseAdminInput{
		PluginID:      "telegram-suite",
		PlanCode:      "perpetual-basic",
		LicenseMode:   "perpetual",
		Status:        "active",
		BoundDomain:   "shop.example.com",
		BoundServerIP: "1.2.3.4",
	})
	if err != nil {
		t.Fatalf("create license failed: %v", err)
	}

	result, err := svc.ValidateLicense(OnlinePluginLicenseValidateInput{
		PluginID:      "telegram-suite",
		LicenseKey:    license.LicenseKey,
		PrimaryDomain: "another.example.com",
		ServerIP:      "8.8.8.8",
		RuntimeLoaded: true,
	})
	if err != nil {
		t.Fatalf("validate license failed: %v", err)
	}
	if result.EnforcementMode != "disable" {
		t.Fatalf("enforcement want disable got %s", result.EnforcementMode)
	}
	if result.MatchedDomain || result.MatchedServerIP {
		t.Fatalf("expected mismatch for domain and server ip")
	}
}
