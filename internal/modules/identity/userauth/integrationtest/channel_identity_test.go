package integrationtest

import (
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
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupUserAuthServiceChannelIdentityTest(t *testing.T) (*userauthapp.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:user_auth_service_channel_identity_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}, &emailverificationdomain.Code{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	userRepo := userstore.New(db)
	identityRepo := externalidentitystore.New(db)

	svc := userauthapp.NewService(&config.Config{}, userRepo, identityRepo, emailverificationstore.New(db), nil, nil, nil)
	svc.SetAuthUnitOfWork(userauthgormstore.New(db))
	return svc, db
}

func TestResolveTelegramChannelIdentityReturnsBoundUser(t *testing.T) {
	svc, db := setupUserAuthServiceChannelIdentityTest(t)
	now := time.Now().Add(-time.Hour)

	user := &userdomain.User{
		Email:                 "telegram_123456@login.local",
		PasswordHash:          "hash",
		PasswordSetupRequired: true,
		DisplayName:           "Old Name",
		Status:                constants.UserStatusActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user failed: %v", err)
	}

	identity := &externalidentitydomain.Identity{
		UserID:         user.ID,
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "123456",
		Username:       "old_username",
		AvatarURL:      "https://old.example/avatar.png",
		AuthAt:         &now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(identity).Error; err != nil {
		t.Fatalf("create identity failed: %v", err)
	}

	resolvedUser, resolvedIdentity, err := svc.ResolveTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: "123456",
		Username:      "new_username",
		AvatarURL:     "https://new.example/avatar.png",
	})
	if err != nil {
		t.Fatalf("ResolveTelegramChannelIdentity returned error: %v", err)
	}
	if resolvedUser == nil || resolvedIdentity == nil {
		t.Fatalf("expected bound user and identity")
	}
	if resolvedUser.ID != user.ID {
		t.Fatalf("resolved user id want %d got %d", user.ID, resolvedUser.ID)
	}
	if resolvedIdentity.Username != "new_username" {
		t.Fatalf("resolved username want new_username got %s", resolvedIdentity.Username)
	}
	if resolvedIdentity.AvatarURL != "https://new.example/avatar.png" {
		t.Fatalf("resolved avatar_url mismatch: %s", resolvedIdentity.AvatarURL)
	}

	var refreshed externalidentitydomain.Identity
	if err := db.First(&refreshed, identity.ID).Error; err != nil {
		t.Fatalf("reload identity failed: %v", err)
	}
	if refreshed.Username != "new_username" {
		t.Fatalf("stored username want new_username got %s", refreshed.Username)
	}
}

func TestProvisionTelegramChannelIdentityCreatesUserAndIdentity(t *testing.T) {
	svc, db := setupUserAuthServiceChannelIdentityTest(t)

	user, identity, created, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{
		ChannelUserID: "987654",
		Username:      "demo_user",
		FirstName:     "Demo",
		LastName:      "User",
		AvatarURL:     "https://example.com/avatar.png",
	})
	if err != nil {
		t.Fatalf("ProvisionTelegramChannelIdentity returned error: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true")
	}
	if user == nil || identity == nil {
		t.Fatalf("expected created user and identity")
	}
	if user.Email != "telegram_987654@login.local" {
		t.Fatalf("user email mismatch: %s", user.Email)
	}
	if user.DisplayName != "Demo User" {
		t.Fatalf("display name want Demo User got %s", user.DisplayName)
	}
	if !user.PasswordSetupRequired {
		t.Fatalf("expected password_setup_required=true")
	}
	if identity.Provider != constants.UserOAuthProviderTelegram {
		t.Fatalf("identity provider want telegram got %s", identity.Provider)
	}
	if identity.ProviderUserID != "987654" {
		t.Fatalf("identity provider_user_id want 987654 got %s", identity.ProviderUserID)
	}

	var userCount int64
	if err := db.Model(&userdomain.User{}).Count(&userCount).Error; err != nil {
		t.Fatalf("count users failed: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("user count want 1 got %d", userCount)
	}

	var identityCount int64
	if err := db.Model(&externalidentitydomain.Identity{}).Count(&identityCount).Error; err != nil {
		t.Fatalf("count identities failed: %v", err)
	}
	if identityCount != 1 {
		t.Fatalf("identity count want 1 got %d", identityCount)
	}
}

func TestBindTelegramChannelByEmailCodeRebindsPlaceholderIdentity(t *testing.T) {
	svc, db := setupUserAuthServiceChannelIdentityTest(t)
	now := time.Now()

	placeholderUser := &userdomain.User{
		Email:                 "telegram_456789@login.local",
		PasswordHash:          "hash",
		PasswordSetupRequired: true,
		DisplayName:           "telegram_456789",
		Status:                constants.UserStatusActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := db.Create(placeholderUser).Error; err != nil {
		t.Fatalf("create placeholder user failed: %v", err)
	}

	targetUser := &userdomain.User{
		Email:        "existing@example.com",
		PasswordHash: "hash",
		DisplayName:  "Existing User",
		Status:       constants.UserStatusActive,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(targetUser).Error; err != nil {
		t.Fatalf("create target user failed: %v", err)
	}

	identity := &externalidentitydomain.Identity{
		UserID:         placeholderUser.ID,
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: "456789",
		Username:       "legacy_user",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := db.Create(identity).Error; err != nil {
		t.Fatalf("create identity failed: %v", err)
	}

	verifyCode := &emailverificationdomain.Code{
		Email:     targetUser.Email,
		Purpose:   constants.VerifyPurposeTelegramBind,
		Code:      "123456",
		ExpiresAt: now.Add(10 * time.Minute),
		SentAt:    now,
		CreatedAt: now,
	}
	if err := db.Create(verifyCode).Error; err != nil {
		t.Fatalf("create verify code failed: %v", err)
	}

	boundUser, boundIdentity, previousUserID, err := svc.BindTelegramChannelByEmailCode(userauthapp.BindTelegramChannelByEmailCodeInput{
		Identity: userauthapp.TelegramChannelIdentityInput{
			ChannelUserID: "456789",
			Username:      "bound_user",
		},
		Email: targetUser.Email,
		Code:  "123456",
	})
	if err != nil {
		t.Fatalf("BindTelegramChannelByEmailCode returned error: %v", err)
	}
	if boundUser == nil || boundIdentity == nil {
		t.Fatalf("expected bound user and identity")
	}
	if boundUser.ID != targetUser.ID {
		t.Fatalf("bound user id want %d got %d", targetUser.ID, boundUser.ID)
	}
	if previousUserID != placeholderUser.ID {
		t.Fatalf("previous user id want %d got %d", placeholderUser.ID, previousUserID)
	}
	if boundIdentity.UserID != targetUser.ID {
		t.Fatalf("identity user id want %d got %d", targetUser.ID, boundIdentity.UserID)
	}
	if boundIdentity.Username != "bound_user" {
		t.Fatalf("identity username want bound_user got %s", boundIdentity.Username)
	}

	var refreshedCode emailverificationdomain.Code
	if err := db.First(&refreshedCode, verifyCode.ID).Error; err != nil {
		t.Fatalf("reload verify code failed: %v", err)
	}
	if refreshedCode.VerifiedAt == nil {
		t.Fatalf("expected verify code verified_at to be set")
	}
}
