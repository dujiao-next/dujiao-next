package userauthhttp

import (
	"context"
	"errors"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrAgreementRequired  = errors.New("agreement required")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
)

// UserLoginSettings 是注册端点所需的设置端口。
type UserLoginSettings interface {
	GetRegistrationEnabled(defaultValue bool) (bool, error)
	GetEmailVerificationEnabled(defaultValue bool) (bool, error)
}

// UserLoginAuth 是注册/登录端点所需的认证端口。
type UserLoginAuth interface {
	Register(ctx context.Context, email, password, code, inviteCode string, agreementAccepted, emailVerificationEnabled bool) (*userdomain.User, string, time.Time, error)
	LoginStep1(ctx context.Context, email, password string, rememberMe bool) (*AuthLoginResult, error)
}

// UserLoginHandler 处理公开的注册与登录 HTTP 请求。
type UserLoginHandler struct {
	settings UserLoginSettings
	auth     UserLoginAuth
	captcha  CaptchaVerifier
	recorder LoginRecorder
}

func NewUserLoginHandler(settings UserLoginSettings, auth UserLoginAuth, captcha CaptchaVerifier, recorder LoginRecorder) *UserLoginHandler {
	if settings == nil {
		panic("user login handler: settings is nil")
	}
	if auth == nil {
		panic("user login handler: auth is nil")
	}
	return &UserLoginHandler{settings: settings, auth: auth, captcha: captcha, recorder: recorder}
}

// UserRegisterRequest 注册请求。
type UserRegisterRequest struct {
	Email             string `json:"email" binding:"required"`
	Password          string `json:"password" binding:"required"`
	Code              string `json:"code"`
	InviteCode        string `json:"invite_code"`
	AgreementAccepted bool   `json:"agreement_accepted"`
}

// UserLoginRequest 登录请求。
type UserLoginRequest struct {
	Email          string                            `json:"email" binding:"required"`
	Password       string                            `json:"password" binding:"required"`
	RememberMe     bool                              `json:"remember_me"`
	CaptchaPayload captchahttp.CaptchaPayloadRequest `json:"captcha_payload"`
}

func (h *UserLoginHandler) recordLogin(c *gin.Context, email string, userID uint, status, failReason, source string) {
	if h != nil {
		recordLoginEvent(h.recorder, c, email, userID, status, failReason, source)
	}
}

// UserRegister 用户注册。
func (h *UserLoginHandler) UserRegister(c *gin.Context) {
	var req UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	registrationEnabled, err := h.settings.GetRegistrationEnabled(true)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.register_failed", err)
		return
	}
	if !registrationEnabled {
		ginutil.RespondError(c, response.CodeForbidden, "error.registration_disabled", nil)
		return
	}

	emailVerificationEnabled, err := h.settings.GetEmailVerificationEnabled(true)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.register_failed", err)
		return
	}

	user, token, expiresAt, err := h.auth.Register(c.Request.Context(), req.Email, req.Password, req.Code, req.InviteCode, req.AgreementAccepted, emailVerificationEnabled)
	if err != nil {
		if respondInviteCodeError(c, err) {
			return
		}
		switch {
		case errors.Is(err, ErrInvalidEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, ErrEmailExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_exists", nil)
		case errors.Is(err, ErrEmailDomainNotAllowed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_domain_not_allowed", nil)
		case errors.Is(err, ErrVerifyCodeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_invalid", nil)
		case errors.Is(err, ErrVerifyCodeExpired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_expired", nil)
		case errors.Is(err, ErrVerifyCodeAttemptsExceeded):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_attempts_exceeded", nil)
		case errors.Is(err, ErrAgreementRequired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.agreement_required", nil)
		case errors.Is(err, ErrWeakPassword):
			respondWeakPassword(c, err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.register_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"user":       userpresenter.NewUserAuthBriefResp(user),
		"token":      token,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UserLogin 用户登录。
func (h *UserLoginHandler) UserLogin(c *gin.Context) {
	var req UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonBadRequest, constants.LoginLogSourceWeb)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if h.captcha != nil {
		if captchaErr := h.captcha.Verify(constants.CaptchaSceneLogin, req.CaptchaPayload, c.ClientIP()); captchaErr != nil {
			switch {
			case errors.Is(captchaErr, captcha.ErrRequired):
				h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonCaptchaRequired, constants.LoginLogSourceWeb)
				ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
				return
			case errors.Is(captchaErr, captcha.ErrInvalid):
				h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonCaptchaInvalid, constants.LoginLogSourceWeb)
				ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
				return
			case errors.Is(captchaErr, captcha.ErrConfigInvalid):
				h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonCaptchaConfigInvalid, constants.LoginLogSourceWeb)
				ginutil.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
				return
			default:
				h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonCaptchaVerifyFailed, constants.LoginLogSourceWeb)
				ginutil.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
				return
			}
		}
	}

	res, err := h.auth.LoginStep1(c.Request.Context(), req.Email, req.Password, req.RememberMe)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonInvalidEmail, constants.LoginLogSourceWeb)
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, ErrInvalidCredentials):
			h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonInvalidCredentials, constants.LoginLogSourceWeb)
			ginutil.RespondError(c, response.CodeUnauthorized, "error.login_invalid", nil)
		case errors.Is(err, ErrEmailNotVerified):
			h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonEmailNotVerified, constants.LoginLogSourceWeb)
			ginutil.RespondError(c, response.CodeUnauthorized, "error.email_not_verified", nil)
		case errors.Is(err, ErrUserDisabled):
			h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonUserDisabled, constants.LoginLogSourceWeb)
			ginutil.RespondError(c, response.CodeUnauthorized, "error.user_disabled", nil)
		default:
			h.recordLogin(c, req.Email, 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonInternalError, constants.LoginLogSourceWeb)
			ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
		}
		return
	}

	if res.RequiresTOTP {
		h.recordLogin(c, res.User.Email, res.User.ID, constants.LoginLogStatusSuccess, constants.LoginLogPasswordOK2FAPending, constants.LoginLogSourceWeb)
		response.Success(c, gin.H{
			"requires_totp":        true,
			"challenge_token":      res.ChallengeToken,
			"challenge_expires_at": res.ChallengeExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		return
	}

	h.recordLogin(c, res.User.Email, res.User.ID, constants.LoginLogStatusSuccess, "", constants.LoginLogSourceWeb)
	response.Success(c, gin.H{
		"requires_totp": false,
		"user":          userpresenter.NewUserAuthBriefResp(res.User),
		"token":         res.Token,
		"expires_at":    res.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

var (
	ErrInviteCodeRequired  = errors.New("invite code required")
	ErrInviteCodeInvalid   = errors.New("invite code invalid")
	ErrInviteTicketInvalid = errors.New("invite ticket invalid")
)

// UserInviteService 是第三方登录补填邀请码端点所需的端口。
type UserInviteService interface {
	CompleteInviteRegistration(ctx context.Context, ticket, inviteCode string) (*AuthLoginResult, error)
}

// UserInviteHandler 处理第三方登录新用户补填邀请码。
type UserInviteHandler struct {
	service  UserInviteService
	recorder LoginRecorder
}

func NewUserInviteHandler(service UserInviteService, recorder LoginRecorder) *UserInviteHandler {
	if service == nil {
		panic("user invite handler: service is nil")
	}
	return &UserInviteHandler{service: service, recorder: recorder}
}

type completeInviteRegistrationRequest struct {
	Ticket     string `json:"invite_ticket" binding:"required"`
	InviteCode string `json:"invite_code" binding:"required"`
}

// respondInviteRequired 第三方身份已验证但需邀请码才能建号：返回票据，由前端弹窗补填后调用 /auth/invite/complete。
func respondInviteRequired(c *gin.Context, res *AuthLoginResult) bool {
	if res == nil || !res.InviteRequired {
		return false
	}
	response.Success(c, gin.H{
		"invite_required": true,
		"invite_ticket":   res.InviteTicket,
	})
	return true
}

// respondInviteCodeError 映射邀请码相关错误，未命中返回 false。
func respondInviteCodeError(c *gin.Context, err error) bool {
	switch {
	case errors.Is(err, ErrInviteCodeRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invite_code_required", nil)
	case errors.Is(err, ErrInviteCodeInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invite_code_invalid", nil)
	case errors.Is(err, ErrInviteTicketInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.invite_ticket_invalid", nil)
	default:
		return false
	}
	return true
}

// CompleteInviteRegistration POST /auth/invite/complete
func (h *UserInviteHandler) CompleteInviteRegistration(c *gin.Context) {
	var req completeInviteRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	res, err := h.service.CompleteInviteRegistration(c.Request.Context(), req.Ticket, req.InviteCode)
	if err != nil {
		recordLoginEvent(h.recorder, c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonBadRequest, constants.LoginLogSourceWeb)
		if respondInviteCodeError(c, err) {
			return
		}
		switch {
		case errors.Is(err, ErrRegistrationDisabled):
			ginutil.RespondError(c, response.CodeForbidden, "error.registration_disabled", nil)
		case errors.Is(err, ErrEmailDomainNotAllowed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_domain_not_allowed", nil)
		case errors.Is(err, ErrUserDisabled):
			ginutil.RespondError(c, response.CodeUnauthorized, "error.user_disabled", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
		}
		return
	}
	if res == nil || res.User == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.login_failed", errors.New("invite registration returned an empty result"))
		return
	}
	if res.RequiresTOTP {
		response.Success(c, gin.H{
			"requires_totp":        true,
			"challenge_token":      res.ChallengeToken,
			"challenge_expires_at": res.ChallengeExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		return
	}
	recordLoginEvent(h.recorder, c, res.User.Email, res.User.ID, constants.LoginLogStatusSuccess, "", constants.LoginLogSourceWeb)
	response.Success(c, gin.H{
		"requires_totp": false,
		"user":          userpresenter.NewUserAuthBriefResp(res.User),
		"token":         res.Token,
		"expires_at":    res.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}
