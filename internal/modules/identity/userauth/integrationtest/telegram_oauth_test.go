package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	memberlevelapp "github.com/dujiao-next/internal/modules/memberlevel/application"
	memberlevelgormstore "github.com/dujiao-next/internal/modules/memberlevel/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/telegramidentity"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestBindTelegramMiniAppConcurrentDifferentIdentitiesReturnsStableConflict(t *testing.T) {
	botToken := "concurrent-bind-bot-token"
	telegramService := telegramauthapp.NewService(
		config.TelegramAuthConfig{
			Enabled:            true,
			BotToken:           botToken,
			LoginExpireSeconds: 300,
			ReplayTTLSeconds:   300,
		},
		telegramauthapp.WithReplaySetNX(func(context.Context, string, interface{}, time.Duration) (bool, error) {
			return true, nil
		}),
	)
	svc, _, db := setupTelegramOAuthTestService(t, telegramService)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	now := time.Now()
	user := &userdomain.User{
		Email: "telegram-concurrent@example.com", PasswordHash: "not-used", DisplayName: "Concurrent",
		Status: constants.UserStatusActive, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	initData := []string{
		buildUserAuthTestTelegramMiniAppInitData(
			t,
			botToken,
			now.Unix(),
			`{"id":1001001,"first_name":"First","username":"first"}`,
		),
		buildUserAuthTestTelegramMiniAppInitData(
			t,
			botToken,
			now.Unix(),
			`{"id":1001002,"first_name":"Second","username":"second"}`,
		),
	}
	start := make(chan struct{})
	results := make(chan error, len(initData))
	var waitGroup sync.WaitGroup
	for _, credential := range initData {
		credential := credential
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, bindErr := svc.BindTelegramMiniApp(userauthapp.BindTelegramMiniAppInput{
				UserID:   user.ID,
				InitData: credential,
				Context:  context.Background(),
			})
			results <- bindErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for resultErr := range results {
		switch {
		case resultErr == nil:
			successes++
		case errors.Is(resultErr, userauthapp.ErrUserOAuthAlreadyBound):
			conflicts++
		default:
			t.Fatalf("unexpected bind error: %v", resultErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("results successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
}

func setupTelegramOAuthTestService(t *testing.T, telegramService ...*telegramauthapp.Service) (*userauthapp.Service, *settingsapp.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:user_auth_service_oauth_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}, &emailverificationdomain.Code{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	if err := db.Exec(
		"CREATE UNIQUE INDEX idx_user_oauth_identity_user_provider ON user_oauth_identities(user_id, provider)",
	).Error; err != nil {
		t.Fatalf("create user/provider identity unique index: %v", err)
	}

	cfg := &config.Config{
		UserJWT: config.JWTConfig{
			SecretKey:   "user-jwt-test-secret",
			ExpireHours: 24,
		},
	}
	settingSvc := settingsapp.NewService(settingsstore.New(db))
	var verifier *telegramauthapp.Service
	if len(telegramService) > 0 {
		verifier = telegramService[0]
	}
	svc := userauthapp.NewService(
		cfg,
		userstore.New(db),
		externalidentitystore.New(db),
		emailverificationstore.New(db),
		settingSvc,
		nil,
		verifier,
	)
	svc.SetAuthUnitOfWork(userauthgormstore.New(db))
	svc.SetGoogleAuthService(googleauthapp.NewService(config.GoogleAuthConfig{
		Enabled:  true,
		ClientID: "google-client-id",
	}))
	return svc, settingSvc, db
}

func TestFindOrCreateTelegramUserRespectsRegistrationSetting(t *testing.T) {
	svc, settings, db := setupTelegramOAuthTestService(t)

	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled: false,
	}); err != nil {
		t.Fatalf("disable registration failed: %v", err)
	}

	user, _, _, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: "10001",
		Username:      "tg_new_user",
	})
	if !errors.Is(err, userauthapp.ErrRegistrationDisabled) {
		t.Fatalf("expected userauthapp.ErrRegistrationDisabled, got user=%v err=%v", user, err)
	}

	var count int64
	if err := db.Model(&userdomain.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no users created, got %d", count)
	}
}

func TestFindOrCreateTelegramUserIgnoresEmailDomainAllowlist(t *testing.T) {
	svc, settings, db := setupTelegramOAuthTestService(t)
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled:         true,
		constants.SettingFieldEmailDomainAllowlistEnabled: true,
		constants.SettingFieldAllowedEmailDomains:         []interface{}{"qq.com"},
	}); err != nil {
		t.Fatalf("update registration config failed: %v", err)
	}

	user, _, _, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: "allowlist_tg_10001",
		Username:      "allowlist_tg",
	})
	if err != nil {
		t.Fatalf("telegram user creation should ignore email domain allowlist: %v", err)
	}
	if user == nil || !telegramidentity.IsPlaceholderEmail(user.Email) {
		t.Fatalf("expected telegram placeholder email user, got %+v", user)
	}

	var count int64
	if err := db.Model(&userdomain.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one telegram user, got %d", count)
	}
}

func TestLoginWithTelegramAllowsExistingIdentityWhenRegistrationDisabled(t *testing.T) {
	svc, settings, db := setupTelegramOAuthTestService(t)

	now := time.Now()
	user := &userdomain.User{
		Email:        telegramidentity.BuildPlaceholderEmail("10002"),
		PasswordHash: "telegram-auto",
		DisplayName:  "TG Existing",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	identity := &externalidentitydomain.Identity{
		UserID:         user.ID,
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "10002",
		Username:       "tg_existing",
		AuthAt:         &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(identity).Error; err != nil {
		t.Fatalf("create identity failed: %v", err)
	}
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled: false,
	}); err != nil {
		t.Fatalf("disable registration failed: %v", err)
	}

	res, err := svc.LoginVerifiedTelegram(&telegramauthapp.IdentityVerified{
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "10002",
		Username:       "tg_existing",
		AuthAt:         time.Now(),
	})
	if err != nil {
		t.Fatalf("loginWithVerifiedTelegram returned error: %v", err)
	}
	if res.User == nil || res.User.ID != user.ID {
		t.Fatalf("expected existing user %d, got %+v", user.ID, res.User)
	}
	if res.Token == "" {
		t.Fatalf("expected token")
	}
	if res.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expected future expiresAt")
	}
}

func TestLoginWithTelegramMigratesOIDCSubjectIdentityToTelegramID(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)

	now := time.Now()
	user := &userdomain.User{
		Email:        telegramidentity.BuildPlaceholderEmail("1234123412341234123"),
		PasswordHash: "telegram-auto",
		DisplayName:  "OIDC Existing",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}
	identity := &externalidentitydomain.Identity{
		UserID:         user.ID,
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "1234123412341234123",
		Username:       "old_oidc",
		AuthAt:         &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(identity).Error; err != nil {
		t.Fatalf("create identity failed: %v", err)
	}

	res, err := svc.LoginVerifiedTelegram(&telegramauthapp.IdentityVerified{
		Provider:              constants.UserOAuthProviderTelegram,
		ProviderUserID:        "987654321",
		ProviderUserIDAliases: []string{"1234123412341234123"},
		Username:              "new_oidc",
		AuthAt:                time.Now(),
	})
	if err != nil {
		t.Fatalf("loginWithVerifiedTelegram returned error: %v", err)
	}
	if res.User == nil || res.User.ID != user.ID {
		t.Fatalf("expected existing user %d, got %+v", user.ID, res.User)
	}

	var migrated externalidentitydomain.Identity
	if err := db.First(&migrated, identity.ID).Error; err != nil {
		t.Fatalf("load migrated identity failed: %v", err)
	}
	if migrated.ProviderUserID != "987654321" {
		t.Fatalf("provider user id not migrated: %q", migrated.ProviderUserID)
	}
	if migrated.Username != "new_oidc" {
		t.Fatalf("username not updated: %q", migrated.Username)
	}
}

func TestTelegramMiniAppLoginReturnsRegistrationDisabledWhenCreatingNewUser(t *testing.T) {
	telegramSvc := telegramauthapp.NewService(config.TelegramAuthConfig{
		Enabled:            true,
		BotToken:           "test-bot-token",
		LoginExpireSeconds: 300,
		ReplayTTLSeconds:   300,
	}, telegramauthapp.WithReplaySetNX(func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
		return true, nil
	}))
	svc, settings, _ := setupTelegramOAuthTestService(t, telegramSvc)

	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled: false,
	}); err != nil {
		t.Fatalf("disable registration failed: %v", err)
	}

	initData := buildUserAuthTestTelegramMiniAppInitData(t, "test-bot-token", time.Now().Unix(), `{"id":10003,"first_name":"Mini","last_name":"Blocked","username":"mini_blocked"}`)
	res, err := svc.LoginWithTelegramMiniApp(userauthapp.LoginWithTelegramMiniAppInput{
		InitData: initData,
		Context:  context.Background(),
	})
	if !errors.Is(err, userauthapp.ErrRegistrationDisabled) {
		t.Fatalf("expected userauthapp.ErrRegistrationDisabled, got res=%+v err=%v", res, err)
	}
}

// TestLoginWithTelegramAssignsDefaultMemberLevel 回归测试：Telegram 一键登录创建的新用户
// 必须被分配默认会员等级，且不会被后续 Update(Save) 用零值覆盖（issue #197）。
func TestLoginWithTelegramAssignsDefaultMemberLevel(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)
	if err := db.AutoMigrate(&memberleveldomain.MemberLevel{}); err != nil {
		t.Fatalf("auto migrate member level failed: %v", err)
	}

	now := time.Now()
	defaultLevel := &memberleveldomain.MemberLevel{
		NameJSON:  jsonmap.JSON{"zh-CN": "默认等级"},
		Slug:      "default",
		IsDefault: true,
		IsActive:  true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(defaultLevel).Error; err != nil {
		t.Fatalf("create default level failed: %v", err)
	}

	svc.SetMemberLevelService(memberlevelapp.NewService(
		memberlevelgormstore.NewLevelStore(db),
		nil,
		memberlevelgormstore.NewUserStore(db),
	))

	res, err := svc.LoginVerifiedTelegram(&telegramauthapp.IdentityVerified{
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "20001",
		Username:       "tg_level_user",
		AuthAt:         time.Now(),
	})
	if err != nil {
		t.Fatalf("loginWithVerifiedTelegram returned error: %v", err)
	}
	if res.User == nil {
		t.Fatalf("expected user, got nil")
	}
	if res.User.MemberLevelID != defaultLevel.ID {
		t.Fatalf("in-memory user member level = %d, want %d", res.User.MemberLevelID, defaultLevel.ID)
	}

	// 关键断言：数据库中的等级未被登录流程末尾的 Update 覆盖
	var persisted userdomain.User
	if err := db.First(&persisted, res.User.ID).Error; err != nil {
		t.Fatalf("load persisted user failed: %v", err)
	}
	if persisted.MemberLevelID != defaultLevel.ID {
		t.Fatalf("persisted user member level = %d, want %d (被零值覆盖)", persisted.MemberLevelID, defaultLevel.ID)
	}
}
