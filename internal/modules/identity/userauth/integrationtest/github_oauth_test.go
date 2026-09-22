package integrationtest

import (
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	githubauthapp "github.com/dujiao-next/internal/modules/identity/githubauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
)

func TestLoginVerifiedGitHubCreatesAndRepeatsVerifiedUser(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)
	identity := &githubauthapp.VerifiedIdentity{ID: "12345", Email: "github@example.com", Login: "octocat", Name: "Octo", AuthAt: time.Now()}
	first, err := svc.LoginVerifiedGitHub(identity)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.LoginVerifiedGitHub(identity)
	if err != nil {
		t.Fatal(err)
	}
	if first.User.ID != second.User.ID {
		t.Fatalf("duplicate login users: %d %d", first.User.ID, second.User.ID)
	}
	var count int64
	if err := db.Model(&externalidentitydomain.Identity{}).Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderGitHub, "12345").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("identity count=%d", count)
	}
}

func TestLoginVerifiedGitHubNeverMergesUnverifiedEmail(t *testing.T) {
	svc, _, _ := setupTelegramOAuthTestService(t)
	_, err := svc.LoginVerifiedGitHub(&githubauthapp.VerifiedIdentity{ID: "99", Email: "", Login: "bad"})
	if err == nil {
		t.Fatal("unverified/missing email accepted")
	}
}

func TestLoginVerifiedGitHubRespectsRegistrationAndEmailDomainSettings(t *testing.T) {
	t.Run("registration disabled", func(t *testing.T) {
		svc, settings, _ := setupTelegramOAuthTestService(t)
		if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
			constants.SettingFieldRegistrationEnabled: false,
		}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.LoginVerifiedGitHub(&githubauthapp.VerifiedIdentity{ID: "disabled", Email: "new@example.com", Login: "new"})
		if !errors.Is(err, userauthapp.ErrRegistrationDisabled) {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("email domain allowlist", func(t *testing.T) {
		svc, settings, _ := setupTelegramOAuthTestService(t)
		if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
			constants.SettingFieldRegistrationEnabled:         true,
			constants.SettingFieldEmailDomainAllowlistEnabled: true,
			constants.SettingFieldAllowedEmailDomains:         []interface{}{"allowed.example"},
		}); err != nil {
			t.Fatal(err)
		}
		_, err := svc.LoginVerifiedGitHub(&githubauthapp.VerifiedIdentity{ID: "domain", Email: "new@example.com", Login: "new"})
		if !errors.Is(err, settingsapp.ErrEmailDomainNotAllowed) {
			t.Fatalf("error=%v", err)
		}
	})
}
