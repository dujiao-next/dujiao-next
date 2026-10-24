package application

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/telegramidentity"

	"github.com/golang-jwt/jwt/v5"
)

// InviteTicketTTL 第三方身份已验证、等待用户补填邀请码的有效期。
const InviteTicketTTL = 10 * time.Minute

const (
	inviteTicketAudience       = "invite_registration"
	inviteTicketProviderGoogle = "google"
	inviteTicketProviderTG     = "telegram"
)

// InviteCodeChecker 只读校验邀请码是否可用（不核销）。
type InviteCodeChecker interface {
	IsUsable(code string) (bool, error)
}

// SetInviteCodeChecker 注入邀请码只读校验，用于建号前的提前预检。
func (s *Service) SetInviteCodeChecker(checker InviteCodeChecker) {
	s.inviteCodes = checker
}

// registrationPolicy 是建号前从设置读出的快照（必须在事务外读取，避免 SQLite 自锁）。
type registrationPolicy struct {
	Enabled     bool
	InviteOnly  bool
	EmailDomain settingsapp.RegistrationEmailDomainPolicy
	InviteCode  string
}

func (s *Service) loadRegistrationPolicy(inviteCode string) (registrationPolicy, error) {
	policy := registrationPolicy{Enabled: true, InviteCode: strings.TrimSpace(inviteCode)}
	if s == nil || s.settingService == nil {
		return policy, nil
	}
	var err error
	if policy.Enabled, err = s.settingService.GetRegistrationEnabled(true); err != nil {
		return policy, err
	}
	if policy.InviteOnly, err = s.settingService.GetRegistrationInviteOnly(); err != nil {
		return policy, err
	}
	if policy.EmailDomain, err = s.settingService.GetRegistrationEmailDomainPolicy(); err != nil {
		return policy, err
	}
	return policy, nil
}

// precheckInviteCode 在消耗邮箱验证码等一次性凭证之前给出明确错误；最终以事务内核销为准。
func (s *Service) precheckInviteCode(policy registrationPolicy) error {
	if !policy.InviteOnly {
		return nil
	}
	if policy.InviteCode == "" {
		return ErrInviteCodeRequired
	}
	if s.inviteCodes == nil {
		return nil
	}
	usable, err := s.inviteCodes.IsUsable(policy.InviteCode)
	if err != nil {
		return err
	}
	if !usable {
		return ErrInviteCodeInvalid
	}
	return nil
}

// createRegisteredUser 是所有新用户落库的唯一入口（邮箱注册、Google、Telegram 登录与 Bot 渠道）。
// 注册开关、邀请码核销与建号在同一事务内完成：建号失败则邀请码不被消耗。
func createRegisteredUser(tx AuthTransaction, user *userdomain.User, policy registrationPolicy) error {
	if !policy.Enabled {
		return ErrRegistrationDisabled
	}
	if policy.InviteOnly {
		if policy.InviteCode == "" {
			return ErrInviteCodeRequired
		}
		inviteCodeID, err := tx.ConsumeInviteCode(policy.InviteCode, time.Now())
		if err != nil {
			return err
		}
		if inviteCodeID == 0 {
			return ErrInviteCodeInvalid
		}
		user.InviteCodeID = &inviteCodeID
	}
	return tx.CreateUser(user)
}

func (s *Service) createUserInTransaction(ctx context.Context, user *userdomain.User, policy registrationPolicy) error {
	if s.authUnitOfWork == nil {
		return errors.New("user auth unit of work is not configured")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.authUnitOfWork.WithinTransaction(ctx, func(tx AuthTransaction) error {
		return createRegisteredUser(tx, user, policy)
	})
}

type inviteTicketClaims struct {
	Provider string                            `json:"provider"`
	Google   *googleauthapp.VerifiedIdentity   `json:"google,omitempty"`
	Telegram *telegramauthapp.IdentityVerified `json:"telegram,omitempty"`
	jwt.RegisteredClaims
}

// inviteTicketKey 由 JWT 密钥派生独立密钥，票据不可能被当作访问令牌或 2FA 挑战令牌使用。
func (s *Service) inviteTicketKey() []byte {
	sum := sha256.Sum256([]byte("dujiao-invite-ticket:" + s.cfg.UserJWT.SecretKey))
	return sum[:]
}

func (s *Service) inviteRequiredResult(claims inviteTicketClaims) (*UserLoginResult, error) {
	now := time.Now()
	claims.RegisteredClaims = jwt.RegisteredClaims{
		Audience:  jwt.ClaimStrings{inviteTicketAudience},
		ExpiresAt: jwt.NewNumericDate(now.Add(InviteTicketTTL)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	ticket, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.inviteTicketKey())
	if err != nil {
		return nil, err
	}
	return &UserLoginResult{InviteRequired: true, InviteTicket: ticket}, nil
}

func (s *Service) parseInviteTicket(ticket string) (*inviteTicketClaims, error) {
	claims := &inviteTicketClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithAudience(inviteTicketAudience),
		jwt.WithExpirationRequired(),
	)
	parsed, err := parser.ParseWithClaims(strings.TrimSpace(ticket), claims, func(*jwt.Token) (interface{}, error) {
		return s.inviteTicketKey(), nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrInviteTicketInvalid
	}
	return claims, nil
}

// CompleteInviteRegistration 用第三方登录时签发的票据 + 邀请码完成建号。
// 票据只能用于"新建账号"：对应账号一旦存在即失效，因此它不会成为一张可重放的登录凭证。
func (s *Service) CompleteInviteRegistration(ctx context.Context, ticket, inviteCode string) (*UserLoginResult, error) {
	if strings.TrimSpace(inviteCode) == "" {
		return nil, ErrInviteCodeRequired
	}
	claims, err := s.parseInviteTicket(ticket)
	if err != nil {
		return nil, err
	}
	switch {
	case claims.Provider == inviteTicketProviderGoogle && claims.Google != nil:
		verified := claims.Google
		identity, err := s.userOAuthIdentityRepo.GetByProviderUserID(constants.UserOAuthProviderGoogle, verified.Sub)
		if err != nil {
			return nil, err
		}
		existing, err := s.userRepo.GetByEmail(verified.Email)
		if err != nil {
			return nil, err
		}
		if identity != nil || existing != nil {
			return nil, ErrInviteTicketInvalid
		}
		return s.loginVerifiedGoogle(ctx, verified, inviteCode)
	case claims.Provider == inviteTicketProviderTG && claims.Telegram != nil:
		verified := claims.Telegram
		identity, err := s.getTelegramIdentityByVerifiedID(verified)
		if err != nil {
			return nil, err
		}
		existing, err := s.userRepo.GetByEmail(telegramidentity.BuildPlaceholderEmail(verified.ProviderUserID))
		if err != nil {
			return nil, err
		}
		if identity != nil || existing != nil {
			return nil, ErrInviteTicketInvalid
		}
		return s.loginVerifiedTelegram(verified, inviteCode)
	default:
		return nil, ErrInviteTicketInvalid
	}
}
