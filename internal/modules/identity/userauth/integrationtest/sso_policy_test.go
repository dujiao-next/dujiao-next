package integrationtest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	ssoauthapp "github.com/dujiao-next/internal/modules/identity/ssoauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
)

func setupOIDCPolicyProvider(t *testing.T, svc *userauthapp.Service, email, subject string, verified bool) *ssoauthapp.Service {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			http.SetCookie(w, &http.Cookie{Name: "casdoor_session_id", Value: "session"})
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": "kano/buyer"})
		case "/api/get-account":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok", "data": map[string]interface{}{"id": subject, "name": "buyer", "email": email, "emailVerified": verified}})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	provider := ssoauthapp.NewService(config.SSOAuthConfig{Enabled: true, OnlyEnabled: true, Issuer: server.URL, ApplicationID: "admin/store", Organization: "kano"})
	svc.SetSSOAuthService(provider)
	return provider
}

func TestOIDCVerifiedEmailAssociation(t *testing.T) {
	for _, test := range []struct {
		name         string
		existing     bool
		bound        bool
		verified     bool
		email        string
		conflict     bool
		wantVerified bool
		wantErr      error
	}{
		{name: "new verified", verified: true, email: " Buyer@Example.com ", wantVerified: true},
		{name: "new unverified", email: "buyer@example.com", wantErr: userauthapp.ErrEmailNotVerified},
		{name: "invalid verified", verified: true, email: "bad-email", wantErr: userauthapp.ErrEmailNotVerified},
		{name: "legacy verified association", existing: true, verified: true, email: "buyer@example.com", wantVerified: true},
		{name: "legacy unverified denied", existing: true, email: "buyer@example.com", wantErr: userauthapp.ErrEmailNotVerified},
		{name: "different subject denied", existing: true, conflict: true, verified: true, email: "buyer@example.com", wantErr: userauthapp.ErrUserOAuthIdentityExists},
		{name: "bound verified marker sync", existing: true, bound: true, verified: true, email: " BUYER@EXAMPLE.COM ", wantVerified: true},
		{name: "bound unverified no sync", existing: true, bound: true, email: "buyer@example.com"},
		{name: "bound changed email no sync", existing: true, bound: true, verified: true, email: "different@example.com"},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, settings, db := setupTelegramOAuthTestService(t)
			if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{constants.SettingFieldRegistrationEnabled: false}); err != nil {
				t.Fatal(err)
			}
			now := time.Now()
			original := &userdomain.User{Email: "buyer@example.com", PasswordHash: "unchanged", PasswordSetupRequired: true, Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now}
			if test.existing {
				if err := db.Create(original).Error; err != nil {
					t.Fatal(err)
				}
				if test.bound || test.conflict {
					subject := "subject"
					if test.conflict {
						subject = "other-subject"
					}
					if err := db.Create(&externalidentitydomain.Identity{UserID: original.ID, Provider: constants.UserOAuthProviderOIDC, ProviderUserID: subject, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			setupOIDCPolicyProvider(t, svc, test.email, "subject", test.verified)
			result, err := svc.LoginWithSSOPassword(userauthapp.SSOPasswordLoginInput{Account: "buyer", Password: "password", Context: context.Background()})
			if test.wantErr != nil {
				if !errors.Is(err, test.wantErr) {
					t.Fatalf("error=%v, want %v", err, test.wantErr)
				}
				if result != nil {
					t.Fatal("denied identity received a result")
				}
				return
			}
			if err != nil || result == nil || result.Login == nil || result.Login.Token == "" {
				t.Fatalf("result=%+v error=%v", result, err)
			}
			user := result.Login.User
			if user.Email != "buyer@example.com" || (user.EmailVerifiedAt != nil) != test.wantVerified {
				t.Fatalf("email=%s verified=%v", user.Email, user.EmailVerifiedAt)
			}
			if test.existing && (user.ID != original.ID || user.PasswordHash != original.PasswordHash) {
				t.Fatal("existing business identity or password changed")
			}
			var users, identities int64
			db.Model(&userdomain.User{}).Count(&users)
			db.Model(&externalidentitydomain.Identity{}).Count(&identities)
			if users != 1 || identities != 1 {
				t.Fatalf("users=%d identities=%d", users, identities)
			}
		})
	}
}

func TestUnifiedPolicyCoreAndOldChallenges(t *testing.T) {
	svc, _, _ := setupTelegramOAuthTestService(t)
	var challenges = map[string]string{}
	for _, source := range []string{constants.LoginLogSourceWeb, constants.LoginLogSourceGoogle, constants.LoginLogSourceTelegram, constants.LoginLogSourceOIDC} {
		token, _, _, err := svc.IssueUserChallengeTokenForSource(1, false, source)
		if err != nil {
			t.Fatal(err)
		}
		challenges[source] = token
	}
	provider := setupOIDCPolicyProvider(t, svc, "buyer@example.com", "subject", true)
	checks := []func() error{
		func() error { _, err := svc.LoginStep1("buyer@example.com", "password", false); return err },
		func() error {
			_, _, _, err := svc.Register("buyer@example.com", "password", "code", "", true, true)
			return err
		},
		func() error { return svc.SendVerifyCode(context.Background(), "buyer@example.com", "reset", "en-US", "") },
		func() error { return svc.ResetPassword("buyer@example.com", "code", "password") },
		func() error { return svc.ChangePassword(1, "old", "new") },
		func() error {
			return svc.SendChangeEmailCode(context.Background(), 1, "new", "buyer@example.com", "en-US")
		},
		func() error { _, err := svc.ChangeEmail(1, "buyer@example.com", "old", "new"); return err },
		func() error { _, err := svc.LoginVerifiedGoogle(nil); return err },
		func() error { _, err := svc.LoginVerifiedTelegram(nil); return err },
		func() error {
			_, err := svc.StartTelegramOIDC(userauthapp.StartTelegramOIDCInput{Intent: "login"})
			return err
		},
	}
	for i, check := range checks {
		if err := check(); !errors.Is(err, userauthapp.ErrUnifiedAuthRequired) {
			t.Errorf("check %d: %v", i, err)
		}
	}
	for source, token := range challenges {
		claims, err := svc.ParseUserChallengeToken(token)
		if source == constants.LoginLogSourceOIDC {
			if err != nil || claims.LoginSource != source {
				t.Fatalf("OIDC source lost: %v", err)
			}
		} else if !errors.Is(err, userauthapp.ErrUnifiedAuthRequired) {
			t.Fatalf("old %s challenge accepted: %v", source, err)
		}
	}
	provider.SetConfig(config.SSOAuthConfig{OnlyEnabled: true})
	if err := checks[0](); !errors.Is(err, userauthapp.ErrUnifiedAuthRequired) {
		t.Fatal("unavailable IdP reopened local login")
	}
	provider.SetConfig(config.SSOAuthConfig{})
	if _, err := svc.ParseUserChallengeToken(challenges[constants.LoginLogSourceWeb]); err != nil {
		t.Fatal("optional policy rejected old challenge", err)
	}
}
