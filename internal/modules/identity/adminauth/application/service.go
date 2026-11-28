package application

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/modules/identity/adminauth/challenge"
	"github.com/dujiao-next/internal/modules/identity/jwttoken"
	"github.com/dujiao-next/internal/shared/passwordpolicy"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Service 认证服务
type Service struct {
	cfg       *config.Config
	adminRepo admincontract.Store
}

// NewService 创建认证服务实例
func NewService(cfg *config.Config, adminRepo admincontract.Store) *Service {
	return &Service{
		cfg:       cfg,
		adminRepo: adminRepo,
	}
}

// HashPassword 使用 bcrypt 加密密码
func (s *Service) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword 验证密码
func (s *Service) VerifyPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

// ValidatePassword 校验密码是否符合策略
func (s *Service) ValidatePassword(password string) error {
	if s == nil || s.cfg == nil {
		return nil
	}
	return passwordpolicy.Validate(s.cfg.Security.PasswordPolicy.ValidationPolicy(), password)
}

// JWTClaims JWT 声明
type JWTClaims struct {
	AdminID      uint   `json:"admin_id"`
	Username     string `json:"username"`
	TokenVersion uint64 `json:"token_version"`
	Typ          string `json:"typ,omitempty"`
	// SessionStartedAt 会话首次登录时间（Unix 秒）。滑动续期时保持不变，
	// 用于限制整段会话的绝对生命周期；0 表示续期前的旧 token。
	SessionStartedAt int64 `json:"sst,omitempty"`
	jwt.RegisteredClaims
}

// ChallengeClaims 2FA 挑战 token 的 JWT claims
//
// 注：Typ 字段同时占用与 JWTClaims 兼容的 typ 键，写入 "2fa_challenge"，
// 防止挑战 token 在被错误地解析为 JWTClaims 时通过中间件的 typ 校验。
type ChallengeClaims struct {
	AdminID uint   `json:"admin_id"`
	JTI     string `json:"jti"`
	Purpose string `json:"purpose"`
	Typ     string `json:"typ"`
	jwt.RegisteredClaims
}

// LoginResult 登录第一步结果
type LoginResult struct {
	RequiresTOTP       bool
	Admin              *admindomain.Admin
	Token              string
	ExpiresAt          time.Time
	ChallengeToken     string
	ChallengeJTI       string
	ChallengeExpiresAt time.Time
}

// GenerateJWT 生成 JWT Token
func (s *Service) GenerateJWT(admin *admindomain.Admin) (string, time.Time, error) {
	return s.generateJWT(admin, time.Now().Unix())
}

// RefreshJWT 在不超出会话绝对生命周期的前提下续签管理员 token。
//
// sessionStartedAt 取自已通过校验的 token（sst 声明）；为 0 表示续期机制上线前
// 签发的旧 token，此时以当前时间为会话起点。MaxSessionHours <= 0 表示不限制。
func (s *Service) RefreshJWT(admin *admindomain.Admin, sessionStartedAt int64) (string, time.Time, error) {
	if s == nil || s.cfg == nil || admin == nil {
		return "", time.Time{}, ErrNotFound
	}

	now := time.Now()
	if sessionStartedAt <= 0 || sessionStartedAt > now.Unix() {
		sessionStartedAt = now.Unix()
	}
	if maxHours := s.cfg.JWT.MaxSessionHours; maxHours > 0 {
		deadline := time.Unix(sessionStartedAt, 0).Add(time.Duration(maxHours) * time.Hour)
		if !now.Before(deadline) {
			return "", time.Time{}, jwttoken.ErrSessionExpired
		}
	}

	return s.generateJWT(admin, sessionStartedAt)
}

func (s *Service) generateJWT(admin *admindomain.Admin, sessionStartedAt int64) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(s.cfg.JWT.ExpireHours) * time.Hour)

	claims := JWTClaims{
		AdminID:          admin.ID,
		Username:         admin.Username,
		TokenVersion:     admin.TokenVersion,
		Typ:              jwttoken.TypeAccess,
		SessionStartedAt: sessionStartedAt,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.cfg.JWT.SecretKey))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// ParseJWT 解析 JWT Token
func (s *Service) ParseJWT(tokenString string) (*JWTClaims, error) {
	parser := jwttoken.NewHS256Parser()
	token, err := parser.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWT.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("无效的 token")
}

// Login 管理员登录（第一步）
func (s *Service) Login(username, password string) (*LoginResult, error) {
	admin, err := s.adminRepo.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$dummyhashtopreventtimingattacksxxxxxxxxxxxxxxxxxx"), []byte(password))
		return nil, ErrInvalidCredentials
	}
	if err := s.VerifyPassword(admin.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	// 已启用 2FA → 仅签发挑战 token
	if admin.TOTPEnabledAt != nil {
		challenge, jti, expiresAt, err := s.IssueChallengeToken(admin.ID)
		if err != nil {
			return nil, err
		}
		return &LoginResult{
			RequiresTOTP:       true,
			Admin:              admin,
			ChallengeToken:     challenge,
			ChallengeJTI:       jti,
			ChallengeExpiresAt: expiresAt,
		}, nil
	}

	// 未启用 → 直接发正式 JWT
	token, expiresAt, err := s.GenerateJWT(admin)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	admin.LastLoginAt = &now
	if err := s.adminRepo.Update(admin); err != nil {
		return nil, err
	}
	_ = cache.SetAdminAuthState(context.Background(), cache.BuildAdminAuthState(admin))
	return &LoginResult{
		RequiresTOTP: false,
		Admin:        admin,
		Token:        token,
		ExpiresAt:    expiresAt,
	}, nil
}

// IssueChallengeToken 签发 2FA 挑战 token
func (s *Service) IssueChallengeToken(adminID uint) (token, jti string, expiresAt time.Time, err error) {
	jti = uuid.NewString()
	expiresAt = time.Now().Add(challenge.TTL)
	claims := ChallengeClaims{
		AdminID: adminID,
		JTI:     jti,
		Purpose: challenge.PurposeTwoFactor,
		Typ:     jwttoken.TypeTwoFactorChallenge,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(s.cfg.JWT.SecretKey))
	if err != nil {
		return "", "", time.Time{}, err
	}
	return signed, jti, expiresAt, nil
}

// ParseChallengeToken 解析并校验挑战 token
func (s *Service) ParseChallengeToken(tokenString string) (*ChallengeClaims, error) {
	parser := jwttoken.NewHS256Parser()
	tok, err := parser.ParseWithClaims(tokenString, &ChallengeClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.cfg.JWT.SecretKey), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := tok.Claims.(*ChallengeClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid challenge token")
	}
	if claims.Purpose != challenge.PurposeTwoFactor || claims.Typ != jwttoken.TypeTwoFactorChallenge {
		return nil, errors.New("invalid challenge purpose")
	}
	return claims, nil
}

// CompleteLoginAfter2FA 在 2FA 验证通过后完成登录：发正式 JWT、更新 last_login
func (s *Service) CompleteLoginAfter2FA(adminID uint) (*LoginResult, error) {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return nil, err
	}
	if admin == nil {
		return nil, ErrNotFound
	}
	token, expiresAt, err := s.GenerateJWT(admin)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	admin.LastLoginAt = &now
	if err := s.adminRepo.Update(admin); err != nil {
		return nil, err
	}
	_ = cache.SetAdminAuthState(context.Background(), cache.BuildAdminAuthState(admin))
	return &LoginResult{RequiresTOTP: false, Admin: admin, Token: token, ExpiresAt: expiresAt}, nil
}

// GetAdminByID returns the administrator needed by authentication transports.
func (s *Service) GetAdminByID(adminID uint) (*admindomain.Admin, error) {
	return s.adminRepo.GetByID(adminID)
}

// ChangePassword 修改管理员密码
func (s *Service) ChangePassword(adminID uint, oldPassword, newPassword string) error {
	admin, err := s.adminRepo.GetByID(adminID)
	if err != nil {
		return err
	}
	if admin == nil {
		return ErrNotFound
	}

	if err := s.VerifyPassword(admin.PasswordHash, oldPassword); err != nil {
		return ErrInvalidPassword
	}

	if err := s.ValidatePassword(newPassword); err != nil {
		return err
	}

	hashedPassword, err := s.HashPassword(newPassword)
	if err != nil {
		return err
	}

	admin.PasswordHash = hashedPassword
	now := time.Now()
	admin.TokenVersion++
	admin.TokenInvalidBefore = &now
	if err := s.adminRepo.Update(admin); err != nil {
		return err
	}
	_ = cache.SetAdminAuthState(context.Background(), cache.BuildAdminAuthState(admin))
	return nil
}
