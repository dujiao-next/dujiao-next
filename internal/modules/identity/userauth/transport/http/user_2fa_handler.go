package userauthhttp

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/constants"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

const (
	userChallengeMaxFailures = 5
)

var (
	ErrTOTPAlreadyEnabled  = errors.New("totp already enabled")
	ErrTOTPNotEnabled      = errors.New("totp not enabled")
	ErrTOTPPendingExpired  = errors.New("totp pending secret expired")
	ErrTOTPCodeInvalid     = errors.New("totp code invalid")
	ErrTOTPRecoveryInvalid = errors.New("recovery code invalid or used")
	ErrTOTPTooManyAttempts = errors.New("too many failed attempts")
)

// UserTOTPStatus 用户 2FA 状态。
type UserTOTPStatus struct {
	Enabled                bool       `json:"enabled"`
	EnabledAt              *time.Time `json:"enabled_at,omitempty"`
	RecoveryCodesRemaining int        `json:"recovery_codes_remaining"`
	RecoveryCodesTotal     int        `json:"recovery_codes_total"`
}

// UserTOTPSetupResult /me/2fa/setup 响应。
type UserTOTPSetupResult struct {
	Secret     string    `json:"secret"`
	OtpauthURL string    `json:"otpauth_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// UserTOTPEnableResult /me/2fa/enable 响应。
type UserTOTPEnableResult struct {
	EnabledAt     time.Time `json:"enabled_at"`
	RecoveryCodes []string  `json:"recovery_codes"`
	Token         string    `json:"token"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// UserChallengeClaims 是 transport 层挑战 token 视图。
type UserChallengeClaims struct {
	UserID      uint
	JTI         string
	RememberMe  bool
	LoginSource string
}

// User2FAChallengeStore 管理挑战失败计数与撤销。
type User2FAChallengeStore interface {
	IsRevoked(ctx context.Context, jti string) bool
	BumpFails(ctx context.Context, jti string) int64
	Revoke(ctx context.Context, jti string)
}

// User2FATOTPService 是 2FA 绑定管理端口。
type User2FATOTPService interface {
	GetStatus(userID uint) (*UserTOTPStatus, error)
	Setup(userID uint) (*UserTOTPSetupResult, error)
	Enable(userID uint, code string) (*UserTOTPEnableResult, error)
	Disable(userID uint, code string, isRecoveryCode bool) error
	RegenerateRecoveryCodes(userID uint, code string) ([]string, error)
	VerifyChallengeCode(userID uint, code string) error
	VerifyChallengeRecoveryCode(userID uint, code string) error
}

// User2FAAuthService 是 2FA 登录完成端口。
type User2FAAuthService interface {
	ParseUserChallengeToken(tokenString string) (*UserChallengeClaims, error)
	CompleteLoginAfter2FA(ctx context.Context, userID uint, rememberMe bool) (*AuthLoginResult, error)
	GetUserEmail(userID uint) (string, error)
}

// User2FAHandler 处理用户 2FA 管理与挑战验证 HTTP 请求。
type User2FAHandler struct {
	totp       User2FATOTPService
	auth       User2FAAuthService
	challenges User2FAChallengeStore
	recorder   LoginRecorder
}

func NewUser2FAHandler(totp User2FATOTPService, auth User2FAAuthService, challenges User2FAChallengeStore, recorder LoginRecorder) *User2FAHandler {
	if totp == nil {
		panic("user 2fa handler: totp is nil")
	}
	if auth == nil {
		panic("user 2fa handler: auth is nil")
	}
	return &User2FAHandler{totp: totp, auth: auth, challenges: challenges, recorder: recorder}
}

func (h *User2FAHandler) recordLogin(c *gin.Context, email string, userID uint, status, failReason, source string) {
	if h != nil {
		recordLoginEvent(h.recorder, c, email, userID, status, failReason, source)
	}
}

// GetUser2FAStatus 当前用户 2FA 状态。
func (h *User2FAHandler) GetUser2FAStatus(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	st, err := h.totp.GetStatus(uid)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		return
	}
	response.Success(c, st)
}

// SetupUser2FA 开始绑定，返回 secret + otpauth url。
func (h *User2FAHandler) SetupUser2FA(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	res, err := h.totp.Setup(uid)
	if err != nil {
		switch {
		case errors.Is(err, ErrTOTPAlreadyEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_already_enabled", nil)
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	response.Success(c, res)
}

// EnableUser2FARequest 启用请求。
type EnableUser2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// EnableUser2FA 完成绑定。
func (h *User2FAHandler) EnableUser2FA(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req EnableUser2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	res, err := h.totp.Enable(uid, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrTOTPAlreadyEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_already_enabled", nil)
		case errors.Is(err, ErrTOTPPendingExpired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_pending_expired", nil)
		case errors.Is(err, ErrTOTPCodeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_invalid", nil)
		case errors.Is(err, ErrTOTPTooManyAttempts):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_too_many_attempts", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	loginRes, signErr := h.auth.CompleteLoginAfter2FA(c.Request.Context(), uid, false)
	if signErr == nil && loginRes != nil {
		res.Token = loginRes.Token
		res.ExpiresAt = loginRes.ExpiresAt
	}
	response.Success(c, res)
}

// DisableUser2FARequest 关闭请求。
type DisableUser2FARequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

// DisableUser2FA 关闭 2FA。
func (h *User2FAHandler) DisableUser2FA(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req DisableUser2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if req.Code == "" && req.RecoveryCode == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_required", nil)
		return
	}
	isRecovery := req.RecoveryCode != ""
	codeArg := req.Code
	if isRecovery {
		codeArg = req.RecoveryCode
	}
	if err := h.totp.Disable(uid, codeArg, isRecovery); err != nil {
		switch {
		case errors.Is(err, ErrTOTPNotEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_not_enabled", nil)
		case errors.Is(err, ErrTOTPCodeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_invalid", nil)
		case errors.Is(err, ErrTOTPRecoveryInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.recovery_code_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	response.Success(c, nil)
}

// RegenerateUser2FARecoveryCodesRequest 重新生成请求。
type RegenerateUser2FARecoveryCodesRequest struct {
	Code string `json:"code" binding:"required"`
}

// RegenerateUser2FARecoveryCodes 重新生成恢复码。
func (h *User2FAHandler) RegenerateUser2FARecoveryCodes(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req RegenerateUser2FARecoveryCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	codes, err := h.totp.RegenerateRecoveryCodes(uid, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrTOTPNotEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_not_enabled", nil)
		case errors.Is(err, ErrTOTPCodeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	response.Success(c, gin.H{"recovery_codes": codes})
}

// VerifyUser2FARequest 第二步登录请求。
type VerifyUser2FARequest struct {
	ChallengeToken string `json:"challenge_token" binding:"required"`
	Code           string `json:"code"`
	RecoveryCode   string `json:"recovery_code"`
}

// VerifyUser2FA 完成两步登录。
func (h *User2FAHandler) VerifyUser2FA(c *gin.Context) {
	var req VerifyUser2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if req.Code == "" && req.RecoveryCode == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_required", nil)
		return
	}
	claims, err := h.auth.ParseUserChallengeToken(req.ChallengeToken)
	if err != nil {
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonChallengeInvalid, constants.LoginLogSourceWeb)
		ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_challenge_invalid", nil)
		return
	}
	ctx := context.Background()
	if h.challenges != nil && h.challenges.IsRevoked(ctx, claims.JTI) {
		h.recordLogin(c, "", claims.UserID, constants.LoginLogStatusFailed, constants.LoginLogFailReasonChallengeInvalid, resolvedChallengeLoginSource(claims))
		ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_challenge_invalid", nil)
		return
	}

	verifyErr := h.verifyUserChallengeAttempt(claims.UserID, req.Code, req.RecoveryCode)
	email, _ := h.auth.GetUserEmail(claims.UserID)
	if verifyErr != nil {
		failCnt := int64(0)
		if h.challenges != nil {
			failCnt = h.challenges.BumpFails(ctx, claims.JTI)
		}
		var failReason string
		switch {
		case errors.Is(verifyErr, ErrTOTPRecoveryInvalid):
			failReason = constants.LoginLogFailReasonInvalidRecoveryCode
		case errors.Is(verifyErr, ErrTOTPCodeInvalid):
			failReason = constants.LoginLogFailReasonInvalidTOTPCode
		default:
			failReason = constants.LoginLogFailReasonInternalError
		}
		h.recordLogin(c, email, claims.UserID, constants.LoginLogStatusFailed, failReason, resolvedChallengeLoginSource(claims))
		if failCnt >= userChallengeMaxFailures {
			if h.challenges != nil {
				h.challenges.Revoke(ctx, claims.JTI)
			}
			ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_too_many_attempts", nil)
			return
		}
		switch {
		case errors.Is(verifyErr, ErrTOTPCodeInvalid):
			ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_code_invalid", nil)
		case errors.Is(verifyErr, ErrTOTPRecoveryInvalid):
			ginutil.RespondError(c, response.CodeUnauthorized, "error.recovery_code_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_challenge_invalid", nil)
		}
		return
	}
	if h.challenges != nil {
		h.challenges.Revoke(ctx, claims.JTI)
	}
	loginRes, err := h.auth.CompleteLoginAfter2FA(c.Request.Context(), claims.UserID, claims.RememberMe)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
		return
	}
	h.recordLogin(c, loginRes.User.Email, loginRes.User.ID, constants.LoginLogStatusSuccess, "", resolvedChallengeLoginSource(claims))
	response.Success(c, gin.H{
		"requires_totp": false,
		"user":          userpresenter.NewUserAuthBriefResp(loginRes.User),
		"token":         loginRes.Token,
		"expires_at":    loginRes.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func resolvedChallengeLoginSource(claims *UserChallengeClaims) string {
	if claims == nil {
		return constants.LoginLogSourceWeb
	}
	switch claims.LoginSource {
	case constants.LoginLogSourceGoogle:
		return constants.LoginLogSourceGoogle
	case constants.LoginLogSourceTelegram:
		return constants.LoginLogSourceTelegram
	default:
		return constants.LoginLogSourceWeb
	}
}

func (h *User2FAHandler) verifyUserChallengeAttempt(userID uint, code, recoveryCode string) error {
	if recoveryCode != "" {
		return h.totp.VerifyChallengeRecoveryCode(userID, recoveryCode)
	}
	return h.totp.VerifyChallengeCode(userID, code)
}
