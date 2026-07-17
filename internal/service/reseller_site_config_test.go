package service

import (
	"context"
	"errors"
	"testing"

	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

func TestResellerSiteConfigServiceUserUpdateRequiresActiveProfile(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	repo := repository.NewResellerRepository(db)
	user := seedResellerManagementUser(t, db, "site-config-pending@example.test")
	profile := models.ResellerProfile{UserID: user.ID, Status: models.ResellerProfileStatusPendingReview, SettlementStatus: models.ResellerSettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	svc := NewResellerSiteConfigService(repo)
	_, err := svc.UpdateUserSiteConfig(context.Background(), user.ID, ResellerSiteConfigInput{SiteName: "Pending Store"})
	if !errors.Is(err, ErrResellerProfileInactive) {
		t.Fatalf("expected inactive profile error, got %v", err)
	}
}

func TestResellerSiteConfigServiceNormalizesAndStoresSafeFields(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	repo := repository.NewResellerRepository(db)
	user := seedResellerManagementUser(t, db, "site-config-active@example.test")
	profile := models.ResellerProfile{UserID: user.ID, Status: models.ResellerProfileStatusActive, SettlementStatus: models.ResellerSettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	svc := NewResellerSiteConfigService(repo)
	row, err := svc.UpdateUserSiteConfig(context.Background(), user.ID, ResellerSiteConfigInput{
		SiteName: "  Alice Store  ",
		Logo:     "/uploads/reseller/logo.png",
		Favicon:  "https://cdn.example.test/favicon.png",
		Announcement: ResellerAnnouncementInput{
			Enabled: true,
			Type:    "info",
			Title:   LocalizedTextInput{"zh-CN": "公告", "fr-FR": "drop me"},
			Content: LocalizedTextInput{"zh-CN": "欢迎"},
		},
		Support: ResellerSupportInput{
			Telegram:   "https://t.me/alice",
			Email:      "support@example.test",
			SupportURL: "mailto:support@example.test",
		},
		SEO: ResellerSEOInput{
			Title:       LocalizedTextInput{"zh-CN": "爱丽丝商店", "zh-TW": "愛麗絲商店", "en-US": "Alice Store"},
			Description: LocalizedTextInput{"zh-CN": "精选商品", "en-US": "Curated products"},
		},
	})
	if err != nil {
		t.Fatalf("update site config failed: %v", err)
	}
	if row.SiteName != "Alice Store" || row.Logo != "/uploads/reseller/logo.png" || row.Favicon != "https://cdn.example.test/favicon.png" {
		t.Fatalf("unexpected normalized row: %+v", row)
	}
	if row.SupportJSON["telegram"] != "https://t.me/alice" || row.SupportJSON["email"] != "support@example.test" {
		t.Fatalf("unexpected support json: %+v", row.SupportJSON)
	}
	announcementTitle := row.AnnouncementJSON["title"].(models.JSON)
	if _, exists := announcementTitle["fr-FR"]; exists {
		t.Fatalf("unexpected unsupported locale retained: %+v", announcementTitle)
	}
	if len(row.ThemeJSON) != 0 {
		t.Fatalf("expected theme config to be ignored, got: %+v", row.ThemeJSON)
	}
}

func TestResellerSiteConfigServiceRejectsUnsafeURLs(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	repo := repository.NewResellerRepository(db)
	user := seedResellerManagementUser(t, db, "site-config-unsafe@example.test")
	profile := models.ResellerProfile{UserID: user.ID, Status: models.ResellerProfileStatusActive, SettlementStatus: models.ResellerSettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	svc := NewResellerSiteConfigService(repo)
	_, err := svc.UpdateUserSiteConfig(context.Background(), user.ID, ResellerSiteConfigInput{
		SiteName: "Unsafe Store",
		Support:  ResellerSupportInput{SupportURL: "javascript:alert(1)"},
	})
	if !errors.Is(err, ErrResellerSiteConfigInvalid) {
		t.Fatalf("expected invalid site config error, got %v", err)
	}
}

func TestResellerSiteConfigServiceReturnsFieldErrorForInvalidSupport(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	repo := repository.NewResellerRepository(db)
	user := seedResellerManagementUser(t, db, "site-config-field@example.test")
	profile := models.ResellerProfile{UserID: user.ID, Status: models.ResellerProfileStatusActive, SettlementStatus: models.ResellerSettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	svc := NewResellerSiteConfigService(repo)
	_, err := svc.UpdateUserSiteConfig(context.Background(), user.ID, ResellerSiteConfigInput{
		SiteName: "Field Store",
		Support:  ResellerSupportInput{WhatsApp: "https://w.me/123"},
	})
	if !errors.Is(err, ErrResellerSiteConfigInvalid) {
		t.Fatalf("expected invalid site config error, got %v", err)
	}
	var fieldErr *ResellerSiteConfigFieldError
	if !errors.As(err, &fieldErr) {
		t.Fatalf("expected field error, got %v", err)
	}
	if fieldErr.Field != "support_whatsapp" {
		t.Fatalf("expected support_whatsapp field, got %q", fieldErr.Field)
	}
}

func TestNormalizeResellerSupportAcceptsTelegramMe(t *testing.T) {
	support, err := normalizeResellerSupport(ResellerSupportInput{Telegram: "https://telegram.me/alice"})
	if err != nil {
		t.Fatalf("expected telegram.me link to be accepted, got %v", err)
	}
	if support["telegram"] != "https://telegram.me/alice" {
		t.Fatalf("unexpected support json: %+v", support)
	}
}

func TestResellerSiteConfigServiceApplyPublicConfigOverlay(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	repo := repository.NewResellerRepository(db)
	user := seedResellerManagementUser(t, db, "site-config-overlay@example.test")
	profile := models.ResellerProfile{UserID: user.ID, Status: models.ResellerProfileStatusActive, SettlementStatus: models.ResellerSettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	svc := NewResellerSiteConfigService(repo)
	if _, err := svc.UpdateUserSiteConfig(context.Background(), user.ID, ResellerSiteConfigInput{
		SiteName: "Overlay Store",
		Favicon:  "/uploads/reseller/favicon.png",
		SEO: ResellerSEOInput{
			Title: LocalizedTextInput{"zh-CN": "覆盖标题", "en-US": "Overlay Title"},
		},
	}); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	resellerID := profile.ID
	base := map[string]interface{}{
		"brand": map[string]interface{}{
			"site_name": "Main Store",
			"site_icon": "/dj.svg",
		},
		"currency": "CNY",
		"seo": map[string]interface{}{
			"title": map[string]interface{}{"zh-CN": "主站标题", "en-US": "Main Title"},
		},
		"announcement": map[string]interface{}{"enabled": true},
		"nav_config":   map[string]interface{}{"builtin": map[string]interface{}{"blog": false}},
	}
	tenant := ResellerTenantContext("shop.example.test", resellerID, user.ID, "shop.example.test")
	out, err := svc.ApplyPublicConfigOverlay(context.Background(), tenant, base)
	if err != nil {
		t.Fatalf("apply overlay failed: %v", err)
	}
	brand := out["brand"].(map[string]interface{})
	if brand["site_name"] != "Overlay Store" || brand["site_icon"] != "/uploads/reseller/favicon.png" {
		t.Fatalf("unexpected brand overlay: %+v", brand)
	}
	if out["currency"] != "CNY" {
		t.Fatalf("global inherited fields should remain, got currency=%v", out["currency"])
	}
	seo := resellerSiteConfigTestMap(out["seo"])
	title := resellerSiteConfigTestMap(seo["title"])
	if title["zh-CN"] != "覆盖标题" || title["en-US"] != "Overlay Title" {
		t.Fatalf("unexpected localized seo overlay: %+v", seo)
	}
	if _, exists := out["announcement"]; exists {
		t.Fatalf("disabled reseller announcement should remove the field, got %+v", out["announcement"])
	}
	if nav := resellerSiteConfigTestMap(out["nav_config"]); nav["builtin"] == nil {
		t.Fatalf("saved reseller config should intentionally own nav defaults, got %+v", nav)
	}
	tenantPayload := out["tenant"].(map[string]interface{})
	if tenantPayload["mode"] != "reseller" || tenantPayload["host"] != "shop.example.test" {
		t.Fatalf("unexpected tenant payload: %+v", tenantPayload)
	}
}

func TestResellerSiteConfigServiceOverlayEmitsActiveAnnouncement(t *testing.T) {
	db := openResellerManagementServiceTestDB(t)
	repo := repository.NewResellerRepository(db)
	user := seedResellerManagementUser(t, db, "site-config-announcement@example.test")
	profile := models.ResellerProfile{UserID: user.ID, Status: models.ResellerProfileStatusActive, SettlementStatus: models.ResellerSettlementStatusNormal}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create profile failed: %v", err)
	}
	svc := NewResellerSiteConfigService(repo)
	if _, err := svc.UpdateUserSiteConfig(context.Background(), user.ID, ResellerSiteConfigInput{
		SiteName: "Announcement Store",
		Announcement: ResellerAnnouncementInput{
			Enabled: true,
			Type:    "success",
			Title:   LocalizedTextInput{"zh-CN": "测试"},
			Content: LocalizedTextInput{"zh-CN": "<p>测试测试</p>"},
		},
	}); err != nil {
		t.Fatalf("save config failed: %v", err)
	}
	tenant := ResellerTenantContext("shop.example.test", profile.ID, user.ID, "shop.example.test")
	out, err := svc.ApplyPublicConfigOverlay(context.Background(), tenant, map[string]interface{}{
		"announcement": map[string]interface{}{"type": "info", "version": "main0000"},
	})
	if err != nil {
		t.Fatalf("apply overlay failed: %v", err)
	}
	announcement := resellerSiteConfigTestMap(out["announcement"])
	if announcement == nil {
		t.Fatalf("enabled reseller announcement should be emitted, got %+v", out["announcement"])
	}
	if announcement["type"] != "success" {
		t.Fatalf("unexpected announcement type: %+v", announcement)
	}
	if _, exists := announcement["enabled"]; exists {
		t.Fatalf("public announcement should not expose enabled flag, got %+v", announcement)
	}
	version, _ := announcement["version"].(string)
	if len(version) != 8 || version == "main0000" {
		t.Fatalf("expected reseller-derived version fingerprint, got %q", version)
	}
	content := resellerSiteConfigTestMap(announcement["content"])
	if content["zh-CN"] != "<p>测试测试</p>" {
		t.Fatalf("unexpected announcement content: %+v", content)
	}
}

func resellerSiteConfigTestMap(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case models.JSON:
		return map[string]interface{}(typed)
	case map[string]interface{}:
		return typed
	default:
		return nil
	}
}
