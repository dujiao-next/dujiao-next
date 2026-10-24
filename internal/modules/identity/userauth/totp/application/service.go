package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/crypto"
	totpapplication "github.com/dujiao-next/internal/modules/identity/totp/application"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
)

const (
	userTotpIssuerDefault     = "Dujiao-Next-User"
	userTotpPendingTTL        = 10 * time.Minute
	userTotpEnableMaxFailures = 5
	RecoveryCodeCount         = 10
	userTotpDigits            = 6
	userTotpPeriod            = 30
	userTotpSkew              = 1
)

// Service 用户 TOTP 业务服务
type Service struct {
	cfg      *config.Config
	encKey   []byte
	userRepo usercontract.Store
	redis    *redis.Client
	now      func() time.Time
}

type Option func(*Service)

func WithClock(now func() time.Time) Option {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

// NewService 创建实例
func NewService(cfg *config.Config, userRepo usercontract.Store, rds *redis.Client, options ...Option) *Service {
	service := &Service{
		cfg:      cfg,
		encKey:   crypto.DeriveKey(cfg.App.SecretKey),
		userRepo: userRepo,
		redis:    rds,
		now:      time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// Status 用户 2FA 状态
type Status struct {
	Enabled                bool       `json:"enabled"`
	EnabledAt              *time.Time `json:"enabled_at,omitempty"`
	RecoveryCodesRemaining int        `json:"recovery_codes_remaining"`
	RecoveryCodesTotal     int        `json:"recovery_codes_total"`
}

// SetupResult /me/2fa/setup 响应
type SetupResult struct {
	Secret     string    `json:"secret"`
	OtpauthURL string    `json:"otpauth_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// EnableResult /me/2fa/enable 响应
type EnableResult struct {
	EnabledAt     time.Time `json:"enabled_at"`
	RecoveryCodes []string  `json:"recovery_codes"`
	// 启用 2FA 时同步 bump TokenVersion 强制其他设备下线，因此当前 session 也需要换发新的 JWT
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GetStatus 查询状态
func (s *Service) GetStatus(userID uint) (*Status, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	st := &Status{Enabled: user.TOTPEnabledAt != nil, EnabledAt: user.TOTPEnabledAt}
	if user.RecoveryCodes != "" {
		entries, err := totpapplication.DecodeRecoveryCodes(user.RecoveryCodes)
		if err == nil {
			st.RecoveryCodesTotal = len(entries)
			for _, e := range entries {
				if e.UsedAt == nil {
					st.RecoveryCodesRemaining++
				}
			}
		}
	}
	return st, nil
}

// Setup 生成 pending secret + otpauth URL
func (s *Service) Setup(userID uint) (*SetupResult, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if user.TOTPEnabledAt != nil {
		return nil, totpapplication.ErrAlreadyEnabled
	}
	issuer := userTotpIssuerDefault
	if s.cfg != nil && strings.TrimSpace(s.cfg.App.TOTPIssuer) != "" {
		issuer = strings.TrimSpace(s.cfg.App.TOTPIssuer)
	}
	accountName := user.Email
	if strings.TrimSpace(accountName) == "" {
		accountName = fmt.Sprintf("user-%d", user.ID)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: accountName,
		Period:      userTotpPeriod,
		Digits:      userTotpDigits,
	})
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	encSecret, err := crypto.Encrypt(s.encKey, key.Secret())
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	expiresAt := s.now().Add(userTotpPendingTTL)
	if err := s.userRepo.UpdateTOTPPending(userID, encSecret, expiresAt); err != nil {
		return nil, err
	}
	if s.redis != nil {
		_ = s.redis.Del(context.Background(), userEnableFailKey(userID)).Err()
	}
	return &SetupResult{
		Secret:     key.Secret(),
		OtpauthURL: key.URL(),
		ExpiresAt:  expiresAt,
	}, nil
}

// Enable 校验首次 code，启用 2FA，生成恢复码
func (s *Service) Enable(userID uint, code string) (*EnableResult, error) {
	prepared, err := totpapplication.Enable(s, totpapplication.EnableInput{
		AccountID:         userID,
		EncryptionKey:     s.encKey,
		Code:              code,
		RecoveryCodeCount: RecoveryCodeCount,
		Now:               s.now,
	})
	if err != nil {
		return nil, err
	}
	return &EnableResult{EnabledAt: prepared.EnabledAt, RecoveryCodes: prepared.RecoveryCodes}, nil
}

// Disable 关闭 2FA
func (s *Service) Disable(userID uint, code string, isRecoveryCode bool) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	if user.TOTPEnabledAt == nil {
		return totpapplication.ErrNotEnabled
	}
	if isRecoveryCode {
		if err := s.consumeRecoveryCode(user, code); err != nil {
			return err
		}
	} else {
		secret, err := crypto.Decrypt(s.encKey, user.TOTPSecret)
		if err != nil {
			return fmt.Errorf("decrypt secret: %w", err)
		}
		if !s.verifyCode(secret, code) {
			return totpapplication.ErrCodeInvalid
		}
	}
	if err := s.userRepo.ClearTOTP(userID); err != nil {
		return err
	}
	_ = cache.DelUserAuthState(context.Background(), userID)
	return nil
}

// RegenerateRecoveryCodes 重新生成恢复码（必须当前 TOTP code）
func (s *Service) RegenerateRecoveryCodes(userID uint, code string) ([]string, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if user.TOTPEnabledAt == nil {
		return nil, totpapplication.ErrNotEnabled
	}
	secret, err := crypto.Decrypt(s.encKey, user.TOTPSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt secret: %w", err)
	}
	if !s.verifyCode(secret, code) {
		return nil, totpapplication.ErrCodeInvalid
	}
	plaintext, codesJSON, err := totpapplication.GenerateRecoveryCodes(RecoveryCodeCount)
	if err != nil {
		return nil, err
	}
	if err := s.userRepo.UpdateRecoveryCodes(userID, codesJSON); err != nil {
		return nil, err
	}
	return plaintext, nil
}

// VerifyChallengeCode 登录第二步：验证 TOTP code
func (s *Service) VerifyChallengeCode(userID uint, code string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	if user.TOTPEnabledAt == nil {
		return totpapplication.ErrNotEnabled
	}
	secret, err := crypto.Decrypt(s.encKey, user.TOTPSecret)
	if err != nil {
		return fmt.Errorf("decrypt secret: %w", err)
	}
	if !s.verifyCode(secret, code) {
		return totpapplication.ErrCodeInvalid
	}
	return nil
}

// VerifyChallengeRecoveryCode 登录第二步：用恢复码（消耗一个）
func (s *Service) VerifyChallengeRecoveryCode(userID uint, code string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	if user.TOTPEnabledAt == nil {
		return totpapplication.ErrNotEnabled
	}
	return s.consumeRecoveryCode(user, code)
}

// AdminResetUser2FA 管理员强制清空目标用户 2FA。
// 使用场景：用户同时丢失 TOTP 设备与所有恢复码，向管理员申诉后由管理员协助解绑。
// 与用户自助 Disable 不同：不需要 code/recovery code，直接清空。
// 同步 bump TokenVersion 强制其他设备下线（由 ClearTOTP 完成）。
// operatorID 仅用于让调用方留痕；service 内部不依赖它，但要求非零以避免来路不明的调用绕过审计。
// 返回 (targetUser, error)：targetUser 供 handler 写审计日志（邮箱等）。
func (s *Service) AdminResetUser2FA(operatorID, targetID uint) (*userdomain.User, error) {
	if operatorID == 0 {
		return nil, fmt.Errorf("operatorID is required for audit")
	}
	user, err := s.userRepo.GetByID(targetID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if user.TOTPEnabledAt == nil {
		return nil, totpapplication.ErrNotEnabled
	}
	if err := s.userRepo.ClearTOTP(targetID); err != nil {
		return nil, err
	}
	_ = cache.DelUserAuthState(context.Background(), targetID)
	return user, nil
}

// ---- 内部辅助 ----

func (s *Service) verifyCode(secret, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	valid, _ := totp.ValidateCustom(code, secret, s.now(), totp.ValidateOpts{
		Period: userTotpPeriod,
		Skew:   userTotpSkew,
		Digits: userTotpDigits,
	})
	return valid
}

func (s *Service) consumeRecoveryCode(user *userdomain.User, code string) error {
	js, err := totpapplication.MatchAndConsumeRecoveryCode(user.RecoveryCodes, code, s.now())
	if err != nil {
		return err
	}
	return s.userRepo.UpdateRecoveryCodes(user.ID, js)
}

func (s *Service) LoadEnableSubject(userID uint) (totpapplication.EnableSubject, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil || user == nil {
		return totpapplication.EnableSubject{}, err
	}
	return totpapplication.EnableSubject{
		Exists:           true,
		EnabledAt:        user.TOTPEnabledAt,
		PendingSecret:    user.TOTPPendingSecret,
		PendingExpiresAt: user.TOTPPendingExpiresAt,
	}, nil
}

func (s *Service) SaveEnabled(userID uint, result *totpapplication.EnableResult) error {
	if err := s.userRepo.UpdateTOTPEnabled(userID, result.EncryptedSecret, result.EnabledAt, result.RecoveryCodesJSON); err != nil {
		return err
	}
	// 启用会递增 token_version 让其他设备下线，需同步清掉鉴权缓存，否则旧令牌在缓存期内仍有效
	_ = cache.DelUserAuthState(context.Background(), userID)
	return nil
}

func (s *Service) ClearEnableFailures(userID uint) {
	if s.redis != nil {
		_ = s.redis.Del(context.Background(), userEnableFailKey(userID)).Err()
	}
}

func userEnableFailKey(userID uint) string {
	return fmt.Sprintf("2fa:user:enable:%d:fails", userID)
}

func (s *Service) CheckEnableFailures(userID uint) error {
	if s.redis == nil {
		return nil
	}
	v, err := s.redis.Get(context.Background(), userEnableFailKey(userID)).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil
	}
	if v >= userTotpEnableMaxFailures {
		_ = s.userRepo.UpdateTOTPPending(userID, "", time.Time{})
		_ = s.redis.Del(context.Background(), userEnableFailKey(userID)).Err()
		return totpapplication.ErrTooManyAttempts
	}
	return nil
}

func (s *Service) BumpEnableFailure(userID uint) {
	if s.redis == nil {
		return
	}
	ctx := context.Background()
	cnt, err := s.redis.Incr(ctx, userEnableFailKey(userID)).Result()
	if err == nil && cnt == 1 {
		_ = s.redis.Expire(ctx, userEnableFailKey(userID), userTotpPendingTTL).Err()
	}
}

func (s *Service) VerifyEnableCode(secret, code string) bool {
	return s.verifyCode(secret, code)
}
