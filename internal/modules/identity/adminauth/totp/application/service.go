package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/crypto"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	totpapplication "github.com/dujiao-next/internal/modules/identity/totp/application"

	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
)

const (
	totpIssuerDefault     = "Dujiao-Next"
	totpPendingTTL        = 10 * time.Minute
	totpEnableMaxFailures = 5
	RecoveryCodeCount     = 10
	totpDigits            = 6
	totpPeriod            = 30
	totpSkew              = 1
)

// Service TOTP 业务服务
//
// 注：审计日志（admin_login_log）由 handler 层在调用前后写入，service 不直接持有 logRepo。
type Service struct {
	cfg       *config.Config
	encKey    []byte
	adminRepo admincontract.Store
	redis     *redis.Client
	now       func() time.Time
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
func NewService(cfg *config.Config, adminRepo admincontract.Store, rds *redis.Client, options ...Option) *Service {
	service := &Service{
		cfg:       cfg,
		encKey:    crypto.DeriveKey(cfg.App.SecretKey),
		adminRepo: adminRepo,
		redis:     rds,
		now:       time.Now,
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// Status 当前账号 2FA 状态
type Status struct {
	Enabled                bool       `json:"enabled"`
	EnabledAt              *time.Time `json:"enabled_at,omitempty"`
	RecoveryCodesRemaining int        `json:"recovery_codes_remaining"`
	RecoveryCodesTotal     int        `json:"recovery_codes_total"`
}

// GetStatus 查询状态
func (s *Service) GetStatus(adminID uint) (*Status, error) {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, ErrNotFound
	}
	st := &Status{Enabled: admin.TOTPEnabledAt != nil, EnabledAt: admin.TOTPEnabledAt}
	if admin.RecoveryCodes != "" {
		entries, err := totpapplication.DecodeRecoveryCodes(admin.RecoveryCodes)
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

// SetupResult /2fa/setup 响应
type SetupResult struct {
	Secret     string    `json:"secret"`
	OtpauthURL string    `json:"otpauth_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// Setup 生成 pending secret + otpauth URL
func (s *Service) Setup(adminID uint) (*SetupResult, error) {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, ErrNotFound
	}
	if admin.TOTPEnabledAt != nil {
		return nil, totpapplication.ErrAlreadyEnabled
	}
	issuer := totpIssuerDefault
	if s.cfg != nil && strings.TrimSpace(s.cfg.App.TOTPIssuer) != "" {
		issuer = strings.TrimSpace(s.cfg.App.TOTPIssuer)
	}
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: admin.Username,
		Period:      totpPeriod,
		Digits:      totpDigits,
	})
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}
	encSecret, err := crypto.Encrypt(s.encKey, key.Secret())
	if err != nil {
		return nil, fmt.Errorf("encrypt secret: %w", err)
	}
	expiresAt := s.now().Add(totpPendingTTL)
	if err := s.adminRepo.UpdateTOTPPending(adminID, encSecret, expiresAt); err != nil {
		return nil, err
	}
	if s.redis != nil {
		_ = s.redis.Del(context.Background(), enableFailKey(adminID)).Err()
	}
	return &SetupResult{
		Secret:     key.Secret(),
		OtpauthURL: key.URL(),
		ExpiresAt:  expiresAt,
	}, nil
}

// EnableResult /2fa/enable 响应
type EnableResult struct {
	EnabledAt     time.Time `json:"enabled_at"`
	RecoveryCodes []string  `json:"recovery_codes"`
}

// Enable 校验首次 code，启用 2FA，生成恢复码
func (s *Service) Enable(adminID uint, code string) (*EnableResult, error) {
	prepared, err := totpapplication.Enable(s, totpapplication.EnableInput{
		AccountID:         adminID,
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

// Disable 关闭 2FA：使用 TOTP code 或恢复码二次确认
func (s *Service) Disable(adminID uint, code string, isRecoveryCode bool) error {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return err
	}
	if admin == nil {
		return ErrNotFound
	}
	if admin.TOTPEnabledAt == nil {
		return totpapplication.ErrNotEnabled
	}
	if isRecoveryCode {
		if err := s.consumeRecoveryCode(admin, code); err != nil {
			return err
		}
	} else {
		secret, err := crypto.Decrypt(s.encKey, admin.TOTPSecret)
		if err != nil {
			return fmt.Errorf("decrypt secret: %w", err)
		}
		if !s.verifyCode(secret, code) {
			return totpapplication.ErrCodeInvalid
		}
	}
	if err := s.adminRepo.ClearTOTP(adminID); err != nil {
		return err
	}
	// 立即清除 Redis 鉴权快照，防止旧 TokenVersion 在 cache TTL 内继续放行旧 token
	_ = cache.DelAdminAuthState(context.Background(), adminID)
	return nil
}

// RegenerateRecoveryCodes 重新生成恢复码（必须当前 TOTP code，不允许用恢复码）
func (s *Service) RegenerateRecoveryCodes(adminID uint, code string) ([]string, error) {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, ErrNotFound
	}
	if admin.TOTPEnabledAt == nil {
		return nil, totpapplication.ErrNotEnabled
	}
	secret, err := crypto.Decrypt(s.encKey, admin.TOTPSecret)
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
	if err := s.adminRepo.UpdateRecoveryCodes(adminID, codesJSON); err != nil {
		return nil, err
	}
	return plaintext, nil
}

// VerifyChallengeCode 登录第二步：验证 TOTP code（不消耗恢复码）
func (s *Service) VerifyChallengeCode(adminID uint, code string) error {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return err
	}
	if admin == nil {
		return ErrNotFound
	}
	if admin.TOTPEnabledAt == nil {
		return totpapplication.ErrNotEnabled
	}
	secret, err := crypto.Decrypt(s.encKey, admin.TOTPSecret)
	if err != nil {
		return fmt.Errorf("decrypt secret: %w", err)
	}
	if !s.verifyCode(secret, code) {
		return totpapplication.ErrCodeInvalid
	}
	return nil
}

// VerifyChallengeRecoveryCode 登录第二步：用恢复码（消耗一个）
func (s *Service) VerifyChallengeRecoveryCode(adminID uint, code string) error {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return err
	}
	if admin == nil {
		return ErrNotFound
	}
	if admin.TOTPEnabledAt == nil {
		return totpapplication.ErrNotEnabled
	}
	return s.consumeRecoveryCode(admin, code)
}

// AdminReset 超管强制清空目标管理员 2FA
func (s *Service) AdminReset(operatorID, targetID uint) error {
	if operatorID == targetID {
		return ErrCannotResetSelf
	}
	target, err := s.adminRepo.GetByID(targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return ErrNotFound
	}
	if err := s.adminRepo.ClearTOTP(targetID); err != nil {
		return err
	}
	// 直接删除缓存，强制下次请求从 DB 加载新的 TokenVersion 并失效旧 token
	_ = cache.DelAdminAuthState(context.Background(), targetID)
	_ = target // target 仅用于上面的 nil 检查
	return nil
}

// ---- 内部辅助 ----

func (s *Service) verifyCode(secret, code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	valid, _ := totp.ValidateCustom(code, secret, s.now(), totp.ValidateOpts{
		Period: totpPeriod,
		Skew:   totpSkew,
		Digits: totpDigits,
	})
	return valid
}

func (s *Service) consumeRecoveryCode(admin *admindomain.Admin, code string) error {
	js, err := totpapplication.MatchAndConsumeRecoveryCode(admin.RecoveryCodes, code, s.now())
	if err != nil {
		return err
	}
	return s.adminRepo.UpdateRecoveryCodes(admin.ID, js)
}

func (s *Service) LoadEnableSubject(adminID uint) (totpapplication.EnableSubject, error) {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil || admin == nil {
		return totpapplication.EnableSubject{}, err
	}
	return totpapplication.EnableSubject{
		Exists:           true,
		EnabledAt:        admin.TOTPEnabledAt,
		PendingSecret:    admin.TOTPPendingSecret,
		PendingExpiresAt: admin.TOTPPendingExpiresAt,
	}, nil
}

func (s *Service) SaveEnabled(adminID uint, result *totpapplication.EnableResult) error {
	if err := s.adminRepo.UpdateTOTPEnabled(adminID, result.EncryptedSecret, result.EnabledAt, result.RecoveryCodesJSON); err != nil {
		return err
	}
	// 启用会递增 token_version 让其他设备下线，需同步清掉鉴权缓存，否则旧令牌在缓存期内仍有效
	_ = cache.DelAdminAuthState(context.Background(), adminID)
	return nil
}

func (s *Service) ClearEnableFailures(adminID uint) {
	if s.redis != nil {
		_ = s.redis.Del(context.Background(), enableFailKey(adminID)).Err()
	}
}

func enableFailKey(adminID uint) string {
	return fmt.Sprintf("2fa:enable:%d:fails", adminID)
}

func (s *Service) CheckEnableFailures(adminID uint) error {
	if s.redis == nil {
		return nil
	}
	v, err := s.redis.Get(context.Background(), enableFailKey(adminID)).Int()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil
	}
	if v >= totpEnableMaxFailures {
		_ = s.adminRepo.UpdateTOTPPending(adminID, "", time.Time{})
		_ = s.redis.Del(context.Background(), enableFailKey(adminID)).Err()
		return totpapplication.ErrTooManyAttempts
	}
	return nil
}

func (s *Service) BumpEnableFailure(adminID uint) {
	if s.redis == nil {
		return
	}
	ctx := context.Background()
	cnt, err := s.redis.Incr(ctx, enableFailKey(adminID)).Result()
	if err == nil && cnt == 1 {
		_ = s.redis.Expire(ctx, enableFailKey(adminID), totpPendingTTL).Err()
	}
}

func (s *Service) VerifyEnableCode(secret, code string) bool {
	return s.verifyCode(secret, code)
}

// 辅助：在 ChallengeToken jti 维度记录失败 / 检查 / revoke
