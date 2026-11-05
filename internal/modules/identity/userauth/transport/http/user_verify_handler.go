package userauthhttp

import (
	"context"
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrInvalidVerifyPurpose  = errors.New("invalid verify purpose")
	ErrEmailExists           = errors.New("email exists")
	ErrEmailDomainNotAllowed = errors.New("email domain not allowed")
)

// CaptchaVerifier 是发送验证码所需的验证码端口。
type CaptchaVerifier interface {
	Verify(scene string, payload captchahttp.CaptchaPayloadRequest, clientIP string) error
}

// UserVerifySettings 是发送验证码所需的设置端口。
type UserVerifySettings interface {
	GetEmailVerificationEnabled(defaultValue bool) (bool, error)
	GetRegistrationEnabled(defaultValue bool) (bool, error)
}

// UserVerifyAuth 是发送邮箱验证码端口。
type UserVerifyAuth interface {
	SendVerifyCode(ctx context.Context, email, purpose, locale, inviteCode string) error
}

// UserVerifyHandler 处理公开的发送邮箱验证码 HTTP 请求。
type UserVerifyHandler struct {
	settings UserVerifySettings
	captcha  CaptchaVerifier
	auth     UserVerifyAuth
}

func NewUserVerifyHandler(settings UserVerifySettings, captcha CaptchaVerifier, auth UserVerifyAuth) *UserVerifyHandler {
	if settings == nil {
		panic("user verify handler: settings is nil")
	}
	if auth == nil {
		panic("user verify handler: auth is nil")
	}
	return &UserVerifyHandler{settings: settings, captcha: captcha, auth: auth}
}

// UserSendVerifyCodeRequest 发送验证码请求。
type UserSendVerifyCodeRequest struct {
	Email          string                            `json:"email" binding:"required"`
	Purpose        string                            `json:"purpose" binding:"required"`
	InviteCode     string                            `json:"invite_code"`
	CaptchaPayload captchahttp.CaptchaPayloadRequest `json:"captcha_payload"`
}

// SendUserVerifyCode 发送用户邮箱验证码。
func (h *UserVerifyHandler) SendUserVerifyCode(c *gin.Context) {
	var req UserSendVerifyCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	purpose := strings.ToLower(strings.TrimSpace(req.Purpose))

	emailVerificationEnabled, err := h.settings.GetEmailVerificationEnabled(true)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.send_verify_code_failed", err)
		return
	}
	if !emailVerificationEnabled {
		ginutil.RespondError(c, response.CodeForbidden, "error.email_verification_disabled", nil)
		return
	}

	if purpose == constants.VerifyPurposeRegister {
		registrationEnabled, err := h.settings.GetRegistrationEnabled(true)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.send_verify_code_failed", err)
			return
		}
		if !registrationEnabled {
			ginutil.RespondError(c, response.CodeForbidden, "error.registration_disabled", nil)
			return
		}
	}

	captchaScene := ""
	switch purpose {
	case constants.VerifyPurposeRegister:
		captchaScene = constants.CaptchaSceneRegisterSendCode
	case constants.VerifyPurposeReset:
		captchaScene = constants.CaptchaSceneResetSendCode
	}
	if captchaScene != "" && h.captcha != nil {
		if captchaErr := h.captcha.Verify(captchaScene, req.CaptchaPayload, c.ClientIP()); captchaErr != nil {
			respondCaptchaError(c, captchaErr)
			return
		}
	}

	locale := i18n.ResolveLocale(c)
	if err := h.auth.SendVerifyCode(c.Request.Context(), req.Email, req.Purpose, locale, req.InviteCode); err != nil {
		if respondInviteCodeError(c, err) {
			return
		}
		switch {
		case errors.Is(err, ErrInvalidEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, ErrInvalidVerifyPurpose):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_purpose_invalid", nil)
		case errors.Is(err, ErrEmailExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_exists", nil)
		case errors.Is(err, ErrEmailDomainNotAllowed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_domain_not_allowed", nil)
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		case errors.Is(err, ErrVerifyCodeTooFrequent):
			ginutil.RespondError(c, response.CodeTooManyRequests, "error.verify_code_too_frequent", nil)
		case errors.Is(err, ErrEmailRecipientRejected):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_recipient_not_found", nil)
		case errors.Is(err, ErrEmailServiceDisabled),
			errors.Is(err, ErrEmailServiceNotConfigured):
			ginutil.RespondError(c, response.CodeInternal, "error.email_service_not_configured", err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.send_verify_code_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"sent": true})
}

func respondCaptchaError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, captcha.ErrRequired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
	case errors.Is(err, captcha.ErrInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
	case errors.Is(err, captcha.ErrConfigInvalid):
		ginutil.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", err)
	}
}
