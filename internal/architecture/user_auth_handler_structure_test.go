package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTelegramAuthLivesInIdentityModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	identityRoot := filepath.Join(repositoryRoot, "internal", "modules", "identity")
	moduleRoot := filepath.Join(identityRoot, "telegramauth")
	applicationRoot := filepath.Join(moduleRoot, "application")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{
		"Service", "LoginPayload", "IdentityVerified", "Option",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{
		"NewService", "WithReplaySetNX", "WithOIDCStateStore",
	})
	assertDirectoryGoFileBudget(t, identityRoot, 0)
	assertDirectoryGoFileBudget(t, moduleRoot, 0)
	assertDirectoryGoFileBudget(t, applicationRoot, 5)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "service", "telegram_auth_service.go"),
		filepath.Join(repositoryRoot, "internal", "service", "telegram_auth_service_test.go"),
		filepath.Join(repositoryRoot, "internal", "service", "telegram_oidc.go"),
		filepath.Join(repositoryRoot, "internal", "service", "telegram_oidc_test.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy Telegram auth path must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy Telegram auth path: %v", err)
		}
	}
}

func TestGoogleAuthLivesInIdentityModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	applicationRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "identity", "googleauth", "application",
	)
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{
		"Service", "VerifiedIdentity", "Option",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{
		"NewService", "WithJWKSURL", "WithHTTPClient", "WithClock",
	})
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
}

func TestEmailVerificationLivesInIdentityModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "identity", "emailverification")

	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "domain", "code.go"), []string{"Code"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "contract", "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"New"})
	assertDirectoryGoFileBudget(t, moduleRoot, 0)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "infrastructure", "gormstore"), 2)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "models", "email_verify_code.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "email_verify_code_repository.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy email verification path must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy email verification path: %v", err)
		}
	}
}

func TestExternalIdentityLivesInIdentityModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "identity", "externalidentity")

	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "domain", "identity.go"), []string{"Identity"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "contract", "store.go"), []string{
		"Store", "TelegramUserFilter", "TelegramUser",
	})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"New"})
	assertDirectoryGoFileBudget(t, moduleRoot, 0)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "infrastructure", "gormstore"), 2)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "models", "user_oauth_identity.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "user_oauth_identity_repository.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "user_oauth_identity_repository_test.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy external identity path must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy external identity path: %v", err)
		}
	}
}

func TestUserAccountPersistenceLivesInIdentityModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "identity", "user")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("user identity module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "domain", "user.go"), []string{"User"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "contract", "store.go"), []string{"Store", "ListFilter"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"New"})
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "domain"), 1)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "contract"), 1)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "infrastructure", "gormstore"), 2)
}

func TestSharedTOTPApplicationLivesInIdentityModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "identity", "totp")
	applicationRoot := filepath.Join(moduleRoot, "application")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("TOTP module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "enable.go"), []string{
		"EnableInput", "EnableResult", "EnableSubject", "EnableStore",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "enable.go"), []string{"Enable", "PrepareEnable"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "recovery_codes.go"), []string{"RecoveryCode"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "recovery_codes.go"), []string{
		"GenerateRecoveryCodes", "DecodeRecoveryCodes", "MatchAndConsumeRecoveryCode",
	})
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
}

func TestUserProfileHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "identity", "userauth", "transport", "http",
	)
	presenterRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "identity", "userauth", "transport", "presenter",
	)

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterUserProfileRoutes",
		"RegisterUserEmailRoutes",
		"RegisterUserPasswordAuthRoutes",
		"RegisterUserPasswordRoutes",
		"RegisterUserVerifyAuthRoutes",
		"RegisterUserRegisterAuthRoutes",
		"RegisterUserLoginAuthRoutes",
		"RegisterUser2FAAuthRoutes",
		"RegisterUser2FARoutes",
		"RegisterUserTelegramOIDCAuthRoutes",
		"RegisterUserTelegramOIDCRoutes",
		"RegisterUserTelegramAuthRoutes",
		"RegisterUserTelegramRoutes",
		"RegisterUserGoogleAuthRoutes",
		"RegisterUserGoogleRoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_profile_handler.go"), []string{
		"UserProfileService", "UserProfileHandler", "UserProfileUpdateRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_profile_handler.go"), []string{
		"NewUserProfileHandler", "GetCurrentUser", "userProfileResponse", "UpdateUserProfile",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_email_handler.go"), []string{
		"UserEmailService", "UserEmailHandler", "ChangeEmailSendCodeRequest", "ChangeEmailRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_email_handler.go"), []string{
		"NewUserEmailHandler", "SendChangeEmailCode", "ChangeEmail", "changeEmailProfileResponse",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_password_handler.go"), []string{
		"UserPasswordService", "UserPasswordHandler", "UserResetPasswordRequest", "ChangeUserPasswordRequest", "WeakPasswordError",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_password_handler.go"), []string{
		"NewUserPasswordHandler", "UserForgotPassword", "ChangeUserPassword", "NewWeakPasswordError", "respondWeakPassword",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_verify_handler.go"), []string{
		"UserVerifySettings", "UserVerifyAuth", "UserVerifyHandler", "UserSendVerifyCodeRequest", "CaptchaVerifier",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_verify_handler.go"), []string{
		"NewUserVerifyHandler", "SendUserVerifyCode", "respondCaptchaError",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_telegram_oidc_handler.go"), []string{
		"UserTelegramOIDCService", "UserTelegramOIDCHandler", "AuthLoginResult", "LoginRecorder",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_telegram_oidc_handler.go"), []string{
		"NewUserTelegramOIDCHandler", "StartTelegramOIDCLogin", "TelegramOIDCLoginCallback",
		"StartTelegramOIDCBind", "TelegramOIDCBindCallback", "respondTelegramOIDCError",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_telegram_handler.go"), []string{
		"UserTelegramService", "UserTelegramHandler", "TelegramAuthPayload",
		"UserTelegramLoginRequest", "UserTelegramMiniAppAuthRequest", "UserBindTelegramRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_telegram_handler.go"), []string{
		"NewUserTelegramHandler", "UserTelegramLogin", "UserTelegramMiniAppLogin",
		"GetMyTelegramBinding", "BindMyTelegram", "BindMyTelegramMiniApp", "UnbindMyTelegram",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_google_handler.go"), []string{
		"UserGoogleService", "UserGoogleHandler", "UserGoogleCredentialRequest", "GoogleBindingResult",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_google_handler.go"), []string{
		"NewUserGoogleHandler", "UserGoogleLogin", "GetMyGoogleBinding", "BindMyGoogle", "UnbindMyGoogle",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_login_handler.go"), []string{
		"UserLoginSettings", "UserLoginAuth", "UserLoginHandler", "UserRegisterRequest", "UserLoginRequest",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_login_handler.go"), []string{
		"NewUserLoginHandler", "UserRegister", "UserLogin",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_2fa_handler.go"), []string{
		"User2FATOTPService", "User2FAAuthService", "User2FAChallengeStore", "User2FAHandler",
		"UserTOTPStatus", "UserTOTPSetupResult", "UserTOTPEnableResult", "UserChallengeClaims",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_2fa_handler.go"), []string{
		"NewUser2FAHandler", "GetUser2FAStatus", "SetupUser2FA", "EnableUser2FA",
		"DisableUser2FA", "RegenerateUser2FARecoveryCodes", "VerifyUser2FA",
	})
	assertFileDeclaresTypes(t, filepath.Join(presenterRoot, "user.go"), []string{
		"UserProfileResp", "TelegramBindingResp", "GoogleBindingResp", "UserAuthBriefResp",
	})
	assertFileDeclaresFunctions(t, filepath.Join(presenterRoot, "user.go"), []string{
		"NewUserProfileResp", "NewTelegramBindingResp", "NewGoogleBindingResp", "NewUserAuthBriefResp",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 15)
	assertDirectoryGoFileBudget(t, presenterRoot, 2)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_profile.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_email.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_auth_password.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_auth_verify.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_auth_telegram_oidc.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_auth_telegram.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_auth_login.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_2fa.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_auth_login_record.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy userauth handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy userauth handler: %v", err)
		}
	}

	for _, legacy := range []string{"userauth_adapter.go", "giftcard_captcha_adapter.go"} {
		if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", legacy)); err == nil {
			t.Fatalf("%s belongs in internal/bootstrap/userauth, not internal/router", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy router adapter %s: %v", legacy, err)
		}
	}
	userAuthWiringRoot := filepath.Join(repositoryRoot, "internal", "bootstrap", "userauth")
	for _, file := range []string{"wiring.go", "adapters.go"} {
		if _, err := os.Stat(filepath.Join(userAuthWiringRoot, file)); err != nil {
			t.Fatalf("userauth wiring file %s missing: %v", file, err)
		}
	}
	assertDirectoryGoFileBudget(t, userAuthWiringRoot, 4)
	captchaWiringRoot := filepath.Join(repositoryRoot, "internal", "wiring", "captcha")
	if _, err := os.Stat(captchaWiringRoot); err == nil {
		t.Fatalf("captcha wiring must stay moved into the captcha module/bootstrap boundary: %s", captchaWiringRoot)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat retired captcha wiring: %v", err)
	}
}
