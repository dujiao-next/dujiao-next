package integrationtest

import (
	"context"
	"fmt"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	"testing"
	"time"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestLoginWithTelegramMiniAppCreatesUserIdentityAndToken(t *testing.T) {
	dsn := fmt.Sprintf("file:user_auth_service_miniapp_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}, &emailverificationdomain.Code{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	cfg := &config.Config{
		UserJWT: config.JWTConfig{
			SecretKey:   "user-jwt-test-secret",
			ExpireHours: 24,
		},
	}
	telegramSvc := telegramauthapp.NewService(config.TelegramAuthConfig{
		Enabled:            true,
		BotToken:           "test-bot-token",
		LoginExpireSeconds: 300,
		ReplayTTLSeconds:   300,
	}, telegramauthapp.WithReplaySetNX(func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
		return true, nil
	}))

	svc := userauthapp.NewService(
		cfg,
		userstore.New(db),
		externalidentitystore.New(db),
		emailverificationstore.New(db),
		nil,
		nil,
		telegramSvc,
	)
	svc.SetAuthUnitOfWork(userauthgormstore.New(db))

	initData := buildUserAuthTestTelegramMiniAppInitData(t, "test-bot-token", time.Now().Unix(), `{"id":987654,"first_name":"Mini","last_name":"Buyer","username":"mini_buyer"}`)
	res, err := svc.LoginWithTelegramMiniApp(userauthapp.LoginWithTelegramMiniAppInput{
		InitData: initData,
		Context:  context.Background(),
	})
	if err != nil {
		t.Fatalf("LoginWithTelegramMiniApp returned error: %v", err)
	}
	user := res.User
	if user == nil {
		t.Fatalf("expected user")
	}
	if user.Email != "telegram_987654@login.local" {
		t.Fatalf("user email mismatch: %s", user.Email)
	}
	if user.Status != constants.UserStatusActive {
		t.Fatalf("user status want active got %s", user.Status)
	}
	if res.Token == "" {
		t.Fatalf("expected token")
	}
	if res.ExpiresAt.Before(time.Now()) {
		t.Fatalf("expected expiresAt in future")
	}

	claims, err := svc.ParseUserJWT(res.Token)
	if err != nil {
		t.Fatalf("ParseUserJWT returned error: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("claims user id want %d got %d", user.ID, claims.UserID)
	}

	var identity externalidentitydomain.Identity
	if err := db.Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderTelegram, "987654").First(&identity).Error; err != nil {
		t.Fatalf("load identity failed: %v", err)
	}
	if identity.UserID != user.ID {
		t.Fatalf("identity user id want %d got %d", user.ID, identity.UserID)
	}
	if identity.Username != "mini_buyer" {
		t.Fatalf("identity username mismatch: %s", identity.Username)
	}
}
