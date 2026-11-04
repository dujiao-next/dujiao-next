package userauthhttp

import (
	"context"
	"errors"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/i18n"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrInvalidEmail               = errors.New("invalid email")
	ErrEmailChangeInvalid         = errors.New("email change invalid")
	ErrEmailChangeExists          = errors.New("email change exists")
	ErrVerifyCodeInvalid          = errors.New("verify code invalid")
	ErrVerifyCodeExpired          = errors.New("verify code expired")
	ErrVerifyCodeTooFrequent      = errors.New("verify code too frequent")
	ErrVerifyCodeAttemptsExceeded = errors.New("verify code attempts exceeded")
	ErrEmailServiceDisabled       = errors.New("email service disabled")
	ErrEmailServiceNotConfigured  = errors.New("email service not configured")
	ErrEmailRecipientRejected     = errors.New("email recipient rejected")
)

// UserEmailService 是更换邮箱端点所需的最小端口。
type UserEmailService interface {
	SendChangeEmailCode(ctx context.Context, userID uint, kind, newEmail, locale string) error
	ChangeEmail(userID uint, newEmail, oldCode, newCode string) (*userdomain.User, error)
	ResolveEmailChangeMode(user *userdomain.User) (string, error)
	ResolvePasswordChangeMode(user *userdomain.User) (string, error)
}

// UserEmailHandler 处理当前用户更换邮箱 HTTP 请求。
type UserEmailHandler struct {
	service UserEmailService
}

func NewUserEmailHandler(service UserEmailService) *UserEmailHandler {
	if service == nil {
		panic("user email handler: service is nil")
	}
	return &UserEmailHandler{service: service}
}

// ChangeEmailSendCodeRequest 更换邮箱验证码请求。
type ChangeEmailSendCodeRequest struct {
	Kind     string `json:"kind" binding:"required"`
	NewEmail string `json:"new_email"`
}

// SendChangeEmailCode 发送更换邮箱验证码。
func (h *UserEmailHandler) SendChangeEmailCode(c *gin.Context) {
	id, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req ChangeEmailSendCodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	locale := i18n.ResolveLocale(c)
	if err := h.service.SendChangeEmailCode(c.Request.Context(), id, req.Kind, req.NewEmail, locale); err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, ErrEmailChangeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_change_invalid", nil)
		case errors.Is(err, ErrEmailChangeExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_change_exists", nil)
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

// ChangeEmailRequest 更换邮箱请求。
type ChangeEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required"`
	OldCode  string `json:"old_code"`
	NewCode  string `json:"new_code" binding:"required"`
}

// ChangeEmail 更换邮箱。
func (h *UserEmailHandler) ChangeEmail(c *gin.Context) {
	id, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req ChangeEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	user, err := h.service.ChangeEmail(id, req.NewEmail, req.OldCode, req.NewCode)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, ErrEmailChangeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_change_invalid", nil)
		case errors.Is(err, ErrEmailChangeExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_change_exists", nil)
		case errors.Is(err, ErrEmailDomainNotAllowed):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_domain_not_allowed", nil)
		case errors.Is(err, ErrVerifyCodeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_invalid", nil)
		case errors.Is(err, ErrVerifyCodeExpired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_expired", nil)
		case errors.Is(err, ErrVerifyCodeAttemptsExceeded):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_attempts_exceeded", nil)
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.email_change_failed", err)
		}
		return
	}

	profile, respErr := h.changeEmailProfileResponse(user)
	if respErr != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.email_change_failed", respErr)
		return
	}
	response.Success(c, profile)
}

func (h *UserEmailHandler) changeEmailProfileResponse(user *userdomain.User) (userpresenter.UserProfileResp, error) {
	emailMode, err := h.service.ResolveEmailChangeMode(user)
	if err != nil {
		return userpresenter.UserProfileResp{}, err
	}
	passwordMode, err := h.service.ResolvePasswordChangeMode(user)
	if err != nil {
		return userpresenter.UserProfileResp{}, err
	}
	return userpresenter.NewUserProfileResp(user, emailMode, passwordMode), nil
}
