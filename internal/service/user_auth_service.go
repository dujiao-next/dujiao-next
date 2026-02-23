package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// UserAuthService 用户认证服务
type UserAuthService struct {
	cfg                   *config.Config
	userRepo              repository.UserRepository
	userOAuthIdentityRepo repository.UserOAuthIdentityRepository
	codeRepo              repository.EmailVerifyCodeRepository
	emailService          *EmailService
	telegramAuthService   *TelegramAuthService
	settingService        *SettingService
}

// NewUserAuthService 创建用户认证服务
func NewUserAuthService(
	cfg *config.Config,
	userRepo repository.UserRepository,
	userOAuthIdentityRepo repository.UserOAuthIdentityRepository,
	codeRepo repository.EmailVerifyCodeRepository,
	emailService *EmailService,
	telegramAuthService *TelegramAuthService,
	settingService *SettingService,
) *UserAuthService {
	return &UserAuthService{
		cfg:                   cfg,
		userRepo:              userRepo,
		userOAuthIdentityRepo: userOAuthIdentityRepo,
		codeRepo:              codeRepo,
		emailService:          emailService,
		telegramAuthService:   telegramAuthService,
		settingService:        settingService,
	}
}

// UserJWTClaims 用户 JWT 声明
type UserJWTClaims struct {
	UserID       uint   `json:"user_id"`
	Email        string `json:"email"`
	TokenVersion uint64 `json:"token_version"`
	jwt.RegisteredClaims
}

const (
	// EmailChangeModeBindOnly 表示仅需校验新邮箱验证码（用于 Telegram 虚拟邮箱账号）
	EmailChangeModeBindOnly = "bind_only"
	// EmailChangeModeChangeWithOldAndNew 表示需要旧邮箱 + 新邮箱双验证码
	EmailChangeModeChangeWithOldAndNew = "change_with_old_and_new"
	// PasswordChangeModeSetWithoutOld 表示首次设置密码，不需要旧密码
	PasswordChangeModeSetWithoutOld = "set_without_old"
	// PasswordChangeModeChangeWithOld 表示修改密码，需要旧密码
	PasswordChangeModeChangeWithOld = "change_with_old"
	telegramPlaceholderEmailPrefix  = "telegram_"
	telegramPlaceholderEmailDomain  = "@login.local"
)

// GenerateUserJWT 生成用户 JWT Token
func (s *UserAuthService) GenerateUserJWT(user *models.User, expireHours int) (string, time.Time, error) {
	resolvedHours := expireHours
	if resolvedHours <= 0 {
		resolvedHours = resolveUserJWTExpireHours(s.cfg.UserJWT)
	}
	expiresAt := time.Now().Add(time.Duration(resolvedHours) * time.Hour)
	claims := UserJWTClaims{
		UserID:       user.ID,
		Email:        user.Email,
		TokenVersion: user.TokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.UserJWT.SecretKey))
	if err != nil {
		return "", time.Time{}, err
	}
	return tokenString, expiresAt, nil
}

// ParseUserJWT 解析用户 JWT Token
func (s *UserAuthService) ParseUserJWT(tokenString string) (*UserJWTClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	claims := &UserJWTClaims{}
	token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.UserJWT.SecretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*UserJWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("无效的 token")
}

// SendVerifyCode 发送邮箱验证码
func (s *UserAuthService) SendVerifyCode(email, purpose, locale string) error {
	if s.emailService == nil {
		return ErrEmailServiceNotConfigured
	}
	normalized, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if !isVerifyPurposeSupported(purpose) {
		return ErrInvalidVerifyPurpose
	}

	if purpose == constants.VerifyPurposeRegister {
		exist, err := s.userRepo.GetByEmail(normalized)
		if err != nil {
			return err
		}
		if exist != nil {
			return ErrEmailExists
		}
	}

	if purpose == constants.VerifyPurposeReset {
		user, err := s.userRepo.GetByEmail(normalized)
		if err != nil {
			return err
		}
		if user == nil {
			return ErrNotFound
		}
		if strings.TrimSpace(user.Locale) != "" {
			locale = user.Locale
		}
	}

	return s.sendVerifyCode(normalized, strings.ToLower(purpose), locale)
}

// Register 用户注册
func (s *UserAuthService) Register(email, password, code string, agreementAccepted bool) (*models.User, string, time.Time, error) {
	if !agreementAccepted {
		return nil, "", time.Time{}, ErrAgreementRequired
	}

	// 检查注册是否开启
	registrationEnabled := true
	requireEmailVerify := true
	if s.settingService != nil {
		if siteConfig, err := s.settingService.GetByKey(constants.SettingKeySiteConfig); err == nil && siteConfig != nil {
			if regConfig, ok := siteConfig["user_registration"].(map[string]interface{}); ok {
				if v, ok := regConfig["enabled"].(bool); ok {
					registrationEnabled = v
				}
				if v, ok := regConfig["require_email_verify"].(bool); ok {
					requireEmailVerify = v
				}
			}
		}
	}
	if !registrationEnabled {
		return nil, "", time.Time{}, ErrRegistrationDisabled
	}

	normalized, err := normalizeEmail(email)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if err := validatePassword(s.cfg.Security.PasswordPolicy, password); err != nil {
		return nil, "", time.Time{}, err
	}

	exist, err := s.userRepo.GetByEmail(normalized)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if exist != nil {
		return nil, "", time.Time{}, ErrEmailExists
	}

	if requireEmailVerify {
		if _, err := s.verifyCode(normalized, constants.VerifyPurposeRegister, code); err != nil {
			return nil, "", time.Time{}, err
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	now := time.Now()
	nickname := resolveNicknameFromEmail(normalized)
	user := &models.User{
		Email:           normalized,
		PasswordHash:    string(hashedPassword),
		DisplayName:     nickname,
		Status:          constants.UserStatusActive,
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, "", time.Time{}, err
	}

	token, expiresAt, err := s.GenerateUserJWT(user, 0)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	user.LastLoginAt = &now
	if err := s.userRepo.Update(user); err != nil {
		return nil, "", time.Time{}, err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))

	return user, token, expiresAt, nil
}

// Login 用户登录
func (s *UserAuthService) Login(email, password string) (*models.User, string, time.Time, error) {
	return s.LoginWithRememberMe(email, password, false)
}

// LoginWithRememberMe 用户登录（支持记住我）
func (s *UserAuthService) LoginWithRememberMe(email, password string, rememberMe bool) (*models.User, string, time.Time, error) {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	user, err := s.userRepo.GetByEmail(normalized)
	if err != nil {
		return nil, "", time.Time{}, err
	}
	if user == nil {
		return nil, "", time.Time{}, ErrInvalidCredentials
	}
	if strings.ToLower(user.Status) != constants.UserStatusActive {
		return nil, "", time.Time{}, ErrUserDisabled
	}
	if user.EmailVerifiedAt == nil {
		return nil, "", time.Time{}, ErrEmailNotVerified
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, "", time.Time{}, ErrInvalidCredentials
	}

	expireHours := resolveUserJWTExpireHours(s.cfg.UserJWT)
	if rememberMe {
		expireHours = resolveRememberMeExpireHours(s.cfg.UserJWT)
	}
	token, expiresAt, err := s.GenerateUserJWT(user, expireHours)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	now := time.Now()
	user.LastLoginAt = &now
	if err := s.userRepo.Update(user); err != nil {
		return nil, "", time.Time{}, err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))

	return user, token, expiresAt, nil
}

// LoginWithTelegramInput Telegram 登录输入
type LoginWithTelegramInput struct {
	Payload TelegramLoginPayload
	Context context.Context
}

// BindTelegramInput 绑定 Telegram 输入
type BindTelegramInput struct {
	UserID  uint
	Payload TelegramLoginPayload
	Context context.Context
}

// LoginWithTelegram Telegram 登录
func (s *UserAuthService) LoginWithTelegram(input LoginWithTelegramInput) (*models.User, string, time.Time, error) {
	if s.telegramAuthService == nil || s.userOAuthIdentityRepo == nil {
		return nil, "", time.Time{}, ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, err := s.telegramAuthService.VerifyLogin(ctx, input.Payload)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	identity, err := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	var user *models.User
	if identity != nil {
		user, err = s.getActiveUserByID(identity.UserID)
		if err != nil {
			return nil, "", time.Time{}, err
		}
		identityChanged := applyTelegramIdentity(verified, identity)
		if identityChanged {
			identity.UpdatedAt = time.Now()
			if err := s.userOAuthIdentityRepo.Update(identity); err != nil {
				return nil, "", time.Time{}, err
			}
		}
	} else {
		user, err = s.findOrCreateTelegramUser(verified)
		if err != nil {
			return nil, "", time.Time{}, err
		}
		identity = &models.UserOAuthIdentity{
			UserID:         user.ID,
			Provider:       verified.Provider,
			ProviderUserID: verified.ProviderUserID,
			Username:       verified.Username,
			AvatarURL:      verified.AvatarURL,
			AuthAt:         &verified.AuthAt,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.userOAuthIdentityRepo.Create(identity); err != nil {
			existing, getErr := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
			if getErr != nil {
				return nil, "", time.Time{}, err
			}
			if existing == nil {
				return nil, "", time.Time{}, err
			}
			identity = existing
			user, err = s.getActiveUserByID(existing.UserID)
			if err != nil {
				return nil, "", time.Time{}, err
			}
		}
	}

	token, expiresAt, err := s.GenerateUserJWT(user, 0)
	if err != nil {
		return nil, "", time.Time{}, err
	}

	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.Update(user); err != nil {
		return nil, "", time.Time{}, err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))
	return user, token, expiresAt, nil
}

// BindTelegram 绑定 Telegram
func (s *UserAuthService) BindTelegram(input BindTelegramInput) (*models.UserOAuthIdentity, error) {
	if input.UserID == 0 {
		return nil, ErrNotFound
	}
	if s.telegramAuthService == nil || s.userOAuthIdentityRepo == nil {
		return nil, ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, err := s.telegramAuthService.VerifyLogin(ctx, input.Payload)
	if err != nil {
		return nil, err
	}
	if _, err := s.getActiveUserByID(input.UserID); err != nil {
		return nil, err
	}

	occupied, err := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
	if err != nil {
		return nil, err
	}
	if occupied != nil && occupied.UserID != input.UserID {
		return nil, ErrUserOAuthIdentityExists
	}

	current, err := s.userOAuthIdentityRepo.GetByUserProvider(input.UserID, verified.Provider)
	if err != nil {
		return nil, err
	}
	if current != nil && current.ProviderUserID != verified.ProviderUserID {
		return nil, ErrUserOAuthAlreadyBound
	}
	if current == nil {
		current = &models.UserOAuthIdentity{
			UserID:         input.UserID,
			Provider:       verified.Provider,
			ProviderUserID: verified.ProviderUserID,
			Username:       verified.Username,
			AvatarURL:      verified.AvatarURL,
			AuthAt:         &verified.AuthAt,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.userOAuthIdentityRepo.Create(current); err != nil {
			return nil, err
		}
		return current, nil
	}

	if applyTelegramIdentity(verified, current) {
		current.UpdatedAt = time.Now()
		if err := s.userOAuthIdentityRepo.Update(current); err != nil {
			return nil, err
		}
	}
	return current, nil
}

// UnbindTelegram 解绑 Telegram
func (s *UserAuthService) UnbindTelegram(userID uint) error {
	if userID == 0 {
		return ErrNotFound
	}
	if s.userOAuthIdentityRepo == nil {
		return ErrTelegramAuthConfigInvalid
	}
	user, err := s.getActiveUserByID(userID)
	if err != nil {
		return err
	}
	mode, err := s.ResolveEmailChangeMode(user)
	if err != nil {
		return err
	}
	if mode == EmailChangeModeBindOnly {
		return ErrTelegramUnbindRequiresEmail
	}
	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(userID, constants.UserOAuthProviderTelegram)
	if err != nil {
		return err
	}
	if identity == nil {
		return ErrUserOAuthNotBound
	}
	return s.userOAuthIdentityRepo.DeleteByID(identity.ID)
}

// GetTelegramBinding 获取 Telegram 绑定
func (s *UserAuthService) GetTelegramBinding(userID uint) (*models.UserOAuthIdentity, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	if s.userOAuthIdentityRepo == nil {
		return nil, ErrTelegramAuthConfigInvalid
	}
	return s.userOAuthIdentityRepo.GetByUserProvider(userID, constants.UserOAuthProviderTelegram)
}

// ResetPassword 重置密码
func (s *UserAuthService) ResetPassword(email, code, newPassword string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if err := validatePassword(s.cfg.Security.PasswordPolicy, newPassword); err != nil {
		return err
	}
	user, err := s.userRepo.GetByEmail(normalized)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}

	if _, err := s.verifyCode(normalized, constants.VerifyPurposeReset, code); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hashedPassword)
	user.PasswordSetupRequired = false
	now := time.Now()
	user.UpdatedAt = now
	user.TokenVersion++
	user.TokenInvalidBefore = &now
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))
	return nil
}

// ChangePassword 登录态修改密码
func (s *UserAuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	if userID == 0 {
		return ErrNotFound
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	mode, err := s.ResolvePasswordChangeMode(user)
	if err != nil {
		return err
	}
	if mode == PasswordChangeModeChangeWithOld {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
			return ErrInvalidPassword
		}
	}

	if err := validatePassword(s.cfg.Security.PasswordPolicy, newPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.PasswordSetupRequired = false
	now := time.Now()
	user.UpdatedAt = now
	user.TokenVersion++
	user.TokenInvalidBefore = &now
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))
	return nil
}

// UpdateProfile 更新用户资料
func (s *UserAuthService) UpdateProfile(userID uint, nickname, locale *string) (*models.User, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	updated := false
	if nickname != nil {
		trimmed := strings.TrimSpace(*nickname)
		if trimmed != "" {
			user.DisplayName = trimmed
			updated = true
		}
	}

	if locale != nil {
		trimmed := strings.TrimSpace(*locale)
		if trimmed != "" {
			user.Locale = trimmed
			updated = true
		}
	}

	if !updated {
		return nil, ErrProfileEmpty
	}

	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

// SendChangeEmailCode 发送更换邮箱验证码
func (s *UserAuthService) SendChangeEmailCode(userID uint, kind, newEmail, locale string) error {
	if s.emailService == nil {
		return ErrEmailServiceNotConfigured
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}

	if strings.TrimSpace(user.Locale) != "" {
		locale = user.Locale
	}
	mode, err := s.ResolveEmailChangeMode(user)
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "old":
		if mode == EmailChangeModeBindOnly {
			return ErrEmailChangeInvalid
		}
		return s.sendVerifyCode(user.Email, constants.VerifyPurposeChangeEmailOld, locale)
	case "new":
		normalized, err := normalizeEmail(newEmail)
		if err != nil {
			return err
		}
		if strings.EqualFold(normalized, user.Email) {
			return ErrEmailChangeInvalid
		}
		exist, err := s.userRepo.GetByEmail(normalized)
		if err != nil {
			return err
		}
		if exist != nil {
			return ErrEmailChangeExists
		}
		return s.sendVerifyCode(normalized, constants.VerifyPurposeChangeEmailNew, locale)
	default:
		return ErrEmailChangeInvalid
	}
}

// ChangeEmail 更换邮箱（旧邮箱/新邮箱双验证）
func (s *UserAuthService) ChangeEmail(userID uint, newEmail, oldCode, newCode string) (*models.User, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	mode, err := s.ResolveEmailChangeMode(user)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeEmail(newEmail)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(normalized, user.Email) {
		return nil, ErrEmailChangeInvalid
	}
	exist, err := s.userRepo.GetByEmail(normalized)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, ErrEmailChangeExists
	}

	if mode != EmailChangeModeBindOnly {
		if _, err := s.verifyCode(user.Email, constants.VerifyPurposeChangeEmailOld, oldCode); err != nil {
			return nil, err
		}
	}
	if _, err := s.verifyCode(normalized, constants.VerifyPurposeChangeEmailNew, newCode); err != nil {
		return nil, err
	}

	now := time.Now()
	user.Email = normalized
	user.EmailVerifiedAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID 获取用户信息
func (s *UserAuthService) GetUserByID(id uint) (*models.User, error) {
	if id == 0 {
		return nil, ErrNotFound
	}
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTelegramVirtualEmailState(user); err != nil {
		return nil, err
	}
	return user, nil
}

// ResolveEmailChangeMode 返回当前用户邮箱修改模式
func (s *UserAuthService) ResolveEmailChangeMode(user *models.User) (string, error) {
	if user == nil {
		return EmailChangeModeChangeWithOldAndNew, nil
	}
	if err := s.ensureTelegramVirtualEmailState(user); err != nil {
		return "", err
	}
	if isTelegramPlaceholderEmail(user.Email) {
		return EmailChangeModeBindOnly, nil
	}
	return EmailChangeModeChangeWithOldAndNew, nil
}

// ResolvePasswordChangeMode 返回当前用户密码修改模式
func (s *UserAuthService) ResolvePasswordChangeMode(user *models.User) (string, error) {
	if user == nil {
		return PasswordChangeModeChangeWithOld, nil
	}
	if err := s.ensureTelegramVirtualEmailState(user); err != nil {
		return "", err
	}
	if user.PasswordSetupRequired {
		return PasswordChangeModeSetWithoutOld, nil
	}
	return PasswordChangeModeChangeWithOld, nil
}

func (s *UserAuthService) ensureTelegramVirtualEmailState(user *models.User) error {
	if user == nil || !isTelegramPlaceholderEmail(user.Email) {
		return nil
	}
	updated := false
	if user.EmailVerifiedAt != nil {
		user.EmailVerifiedAt = nil
		updated = true
	}
	if !user.PasswordSetupRequired {
		user.PasswordSetupRequired = true
		updated = true
	}
	if !updated {
		return nil
	}
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(user)
}

func (s *UserAuthService) verifyCode(email, purpose, code string) (*models.EmailVerifyCode, error) {
	record, err := s.codeRepo.GetLatest(email, purpose)
	if err != nil {
		return nil, err
	}
	if record == nil {
		return nil, ErrVerifyCodeInvalid
	}
	if record.VerifiedAt != nil {
		return nil, ErrVerifyCodeInvalid
	}

	now := time.Now()
	if record.ExpiresAt.Before(now) {
		return nil, ErrVerifyCodeExpired
	}

	maxAttempts := resolveMaxAttempts(s.cfg.Email.VerifyCode)
	if maxAttempts > 0 && record.AttemptCount >= maxAttempts {
		return nil, ErrVerifyCodeAttemptsExceeded
	}

	if strings.TrimSpace(record.Code) != strings.TrimSpace(code) {
		_ = s.codeRepo.IncrementAttempt(record.ID)
		return nil, ErrVerifyCodeInvalid
	}

	if err := s.codeRepo.MarkVerified(record.ID, now); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *UserAuthService) sendVerifyCode(email, purpose, locale string) error {
	latest, err := s.codeRepo.GetLatest(email, purpose)
	if err != nil {
		return err
	}
	now := time.Now()
	if latest != nil {
		interval := time.Duration(resolveSendIntervalSeconds(s.cfg.Email.VerifyCode)) * time.Second
		if !latest.SentAt.IsZero() && now.Sub(latest.SentAt) < interval {
			return ErrVerifyCodeTooFrequent
		}
	}

	code, err := randomNumericCode(resolveCodeLength(s.cfg.Email.VerifyCode))
	if err != nil {
		return err
	}

	record := &models.EmailVerifyCode{
		Email:     email,
		Purpose:   strings.ToLower(purpose),
		Code:      code,
		ExpiresAt: now.Add(time.Duration(resolveExpireMinutes(s.cfg.Email.VerifyCode)) * time.Minute),
		SentAt:    now,
		CreatedAt: now,
	}
	if err := s.emailService.SendVerifyCode(email, code, purpose, locale); err != nil {
		return err
	}

	if err := s.codeRepo.Create(record); err != nil {
		return err
	}

	return nil
}

func (s *UserAuthService) getActiveUserByID(userID uint) (*models.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if strings.ToLower(strings.TrimSpace(user.Status)) != constants.UserStatusActive {
		return nil, ErrUserDisabled
	}
	return user, nil
}

func (s *UserAuthService) findOrCreateTelegramUser(verified *TelegramIdentityVerified) (*models.User, error) {
	if verified == nil {
		return nil, ErrTelegramAuthPayloadInvalid
	}
	email := buildTelegramPlaceholderEmail(verified.ProviderUserID)
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		if strings.ToLower(strings.TrimSpace(user.Status)) != constants.UserStatusActive {
			return nil, ErrUserDisabled
		}
		return user, nil
	}

	randomSuffix, err := randomNumericCode(16)
	if err != nil {
		return nil, err
	}
	passwordSeed := fmt.Sprintf("tg_%s_%s", verified.ProviderUserID, randomSuffix)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordSeed), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user = &models.User{
		Email:                 email,
		PasswordHash:          string(hashedPassword),
		PasswordSetupRequired: true,
		DisplayName:           resolveTelegramDisplayName(verified),
		Status:                constants.UserStatusActive,
		LastLoginAt:           &now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

func applyTelegramIdentity(verified *TelegramIdentityVerified, identity *models.UserOAuthIdentity) bool {
	if verified == nil || identity == nil {
		return false
	}
	changed := false
	if identity.Provider == "" {
		identity.Provider = verified.Provider
		changed = true
	}
	if identity.ProviderUserID == "" {
		identity.ProviderUserID = verified.ProviderUserID
		changed = true
	}
	if identity.Username != verified.Username {
		identity.Username = verified.Username
		changed = true
	}
	if identity.AvatarURL != verified.AvatarURL {
		identity.AvatarURL = verified.AvatarURL
		changed = true
	}
	if identity.AuthAt == nil || !identity.AuthAt.Equal(verified.AuthAt) {
		authAt := verified.AuthAt
		identity.AuthAt = &authAt
		changed = true
	}
	return changed
}

func buildTelegramPlaceholderEmail(providerUserID string) string {
	normalizedID := strings.TrimSpace(providerUserID)
	if normalizedID == "" {
		normalizedID = "unknown"
	}
	return fmt.Sprintf("%s%s%s", telegramPlaceholderEmailPrefix, normalizedID, telegramPlaceholderEmailDomain)
}

func isTelegramPlaceholderEmail(email string) bool {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return false
	}
	return strings.HasPrefix(normalized, telegramPlaceholderEmailPrefix) &&
		strings.HasSuffix(normalized, telegramPlaceholderEmailDomain)
}

func resolveTelegramDisplayName(verified *TelegramIdentityVerified) string {
	if verified == nil {
		return "Telegram User"
	}
	fullName := strings.TrimSpace(strings.TrimSpace(verified.FirstName) + " " + strings.TrimSpace(verified.LastName))
	if fullName != "" {
		return fullName
	}
	if strings.TrimSpace(verified.Username) != "" {
		return verified.Username
	}
	if strings.TrimSpace(verified.ProviderUserID) != "" {
		return fmt.Sprintf("telegram_%s", strings.TrimSpace(verified.ProviderUserID))
	}
	return "Telegram User"
}

func normalizeEmail(email string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return "", ErrInvalidEmail
	}
	if _, err := mail.ParseAddress(normalized); err != nil {
		return "", ErrInvalidEmail
	}
	return normalized, nil
}

// NormalizeEmail 统一邮箱格式
func NormalizeEmail(email string) (string, error) {
	return normalizeEmail(email)
}

func isVerifyPurposeSupported(purpose string) bool {
	switch strings.ToLower(strings.TrimSpace(purpose)) {
	case constants.VerifyPurposeRegister, constants.VerifyPurposeReset, constants.VerifyPurposeChangeEmailOld, constants.VerifyPurposeChangeEmailNew:
		return true
	default:
		return false
	}
}

func resolveUserJWTExpireHours(cfg config.JWTConfig) int {
	if cfg.ExpireHours <= 0 {
		return 24
	}
	return cfg.ExpireHours
}

func resolveRememberMeExpireHours(cfg config.JWTConfig) int {
	if cfg.RememberMeExpireHours <= 0 {
		return resolveUserJWTExpireHours(cfg)
	}
	return cfg.RememberMeExpireHours
}

func resolveNicknameFromEmail(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 && strings.TrimSpace(parts[0]) != "" {
		return strings.TrimSpace(parts[0])
	}
	return email
}

func resolveExpireMinutes(cfg config.VerifyCodeConfig) int {
	if cfg.ExpireMinutes <= 0 {
		return 10
	}
	return cfg.ExpireMinutes
}

func resolveSendIntervalSeconds(cfg config.VerifyCodeConfig) int {
	if cfg.SendIntervalSeconds <= 0 {
		return 60
	}
	return cfg.SendIntervalSeconds
}

func resolveMaxAttempts(cfg config.VerifyCodeConfig) int {
	if cfg.MaxAttempts <= 0 {
		return 5
	}
	return cfg.MaxAttempts
}

func resolveCodeLength(cfg config.VerifyCodeConfig) int {
	if cfg.Length < 4 || cfg.Length > 10 {
		return 6
	}
	return cfg.Length
}

func randomNumericCode(length int) (string, error) {
	var b strings.Builder
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf("%d", n.Int64()))
	}
	return b.String(), nil
}
