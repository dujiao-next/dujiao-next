package userauthhttp

import (
	"context"
	"errors"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrTelegramAuthDisabled       = errors.New("telegram auth disabled")
	ErrTelegramAuthConfigInvalid  = errors.New("telegram auth config invalid")
	ErrTelegramOIDCStateInvalid   = errors.New("telegram oidc state invalid")
	ErrTelegramOIDCTokenExchange  = errors.New("telegram oidc token exchange failed")
	ErrTelegramOIDCIDTokenInvalid = errors.New("telegram oidc id token invalid")
	ErrTelegramAuthPayloadInvalid = errors.New("telegram auth payload invalid")
	ErrTelegramAuthExpired        = errors.New("telegram auth expired")
	ErrTelegramAuthReplay         = errors.New("telegram auth replay")
	ErrUserOAuthIdentityExists    = errors.New("user oauth identity exists")
	ErrUserOAuthAlreadyBound      = errors.New("user oauth already bound")
	ErrUserDisabled               = errors.New("user disabled")
	ErrRegistrationDisabled       = errors.New("registration disabled")
)

// AuthLoginResult 是 transport 层登录结果视图。
type AuthLoginResult struct {
	RequiresTOTP       bool
	User               *userdomain.User
	Token              string
	ExpiresAt          time.Time
	ChallengeToken     string
	ChallengeExpiresAt time.Time
	InviteRequired     bool
	InviteTicket       string
}

// LoginRecorder 记录用户登录审计日志。
type LoginRecorder interface {
	Record(email string, userID uint, status, failReason, source, clientIP, userAgent, requestID string)
}

// recordLoginEvent 记录一次登录事件（附带客户端 IP、UA 与请求 ID）。
func recordLoginEvent(recorder LoginRecorder, c *gin.Context, email string, userID uint, status, failReason, source string) {
	if recorder == nil || c == nil {
		return
	}
	requestID, _ := c.Get("request_id")
	requestIDText, _ := requestID.(string)
	recorder.Record(email, userID, status, failReason, source, c.ClientIP(), c.GetHeader("User-Agent"), requestIDText)
}

// UserTelegramOIDCService 是 Telegram OIDC 端点所需的最小端口。
type UserTelegramOIDCService interface {
	StartTelegramOIDC(ctx context.Context, intent string, userID uint) (string, error)
	LoginWithTelegramOIDC(ctx context.Context, code, state string) (*AuthLoginResult, error)
	BindTelegramOIDC(ctx context.Context, userID uint, code, state string) (*externalidentitydomain.Identity, error)
}

// UserTelegramOIDCHandler 处理 Telegram OIDC 登录与绑定 HTTP 请求。
type UserTelegramOIDCHandler struct {
	service  UserTelegramOIDCService
	recorder LoginRecorder
}

func NewUserTelegramOIDCHandler(service UserTelegramOIDCService, recorder LoginRecorder) *UserTelegramOIDCHandler {
	if service == nil {
		panic("user telegram oidc handler: service is nil")
	}
	return &UserTelegramOIDCHandler{service: service, recorder: recorder}
}

type telegramOIDCCallbackRequest struct {
	Code  string `json:"code" binding:"required"`
	State string `json:"state" binding:"required"`
}

func respondTelegramOIDCError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTelegramAuthDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_disabled", nil)
	case errors.Is(err, ErrTelegramAuthConfigInvalid):
		ginutil.RespondError(c, response.CodeInternal, "error.telegram_auth_config_invalid", err)
	case errors.Is(err, ErrTelegramOIDCStateInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_oidc_state_invalid", nil)
	case errors.Is(err, ErrTelegramOIDCTokenExchange):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_oidc_token_exchange_failed", err)
	case errors.Is(err, ErrTelegramOIDCIDTokenInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_oidc_id_token_invalid", nil)
	case errors.Is(err, ErrTelegramAuthPayloadInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_payload_invalid", nil)
	case errors.Is(err, ErrTelegramAuthExpired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_expired", nil)
	case errors.Is(err, ErrTelegramAuthReplay):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_replayed", nil)
	case errors.Is(err, ErrUserOAuthIdentityExists):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_bind_conflict", nil)
	case errors.Is(err, ErrUserOAuthAlreadyBound):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_already_bound", nil)
	case errors.Is(err, ErrUserDisabled):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.user_disabled", nil)
	case errors.Is(err, ErrRegistrationDisabled):
		ginutil.RespondError(c, response.CodeForbidden, "error.registration_disabled", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
	}
}

func (h *UserTelegramOIDCHandler) recordLogin(c *gin.Context, email string, userID uint, status, failReason, source string) {
	if h != nil {
		recordLoginEvent(h.recorder, c, email, userID, status, failReason, source)
	}
}

// StartTelegramOIDCLogin 返回 Telegram OIDC 授权 URL（登录流程）。
func (h *UserTelegramOIDCHandler) StartTelegramOIDCLogin(c *gin.Context) {
	authURL, err := h.service.StartTelegramOIDC(c.Request.Context(), "login", 0)
	if err != nil {
		respondTelegramOIDCError(c, err)
		return
	}
	response.Success(c, gin.H{"auth_url": authURL})
}

// TelegramOIDCLoginCallback 处理 Telegram OIDC 回调（登录）。
func (h *UserTelegramOIDCHandler) TelegramOIDCLoginCallback(c *gin.Context) {
	var req telegramOIDCCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonBadRequest, constants.LoginLogSourceTelegram)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	res, err := h.service.LoginWithTelegramOIDC(c.Request.Context(), req.Code, req.State)
	if err != nil {
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonTelegramInvalid, constants.LoginLogSourceTelegram)
		respondTelegramOIDCError(c, err)
		return
	}
	if respondInviteRequired(c, res) {
		return
	}
	if res.RequiresTOTP {
		h.recordLogin(c, res.User.Email, res.User.ID, constants.LoginLogStatusSuccess, constants.LoginLogPasswordOK2FAPending, constants.LoginLogSourceTelegram)
		response.Success(c, gin.H{
			"requires_totp":        true,
			"challenge_token":      res.ChallengeToken,
			"challenge_expires_at": res.ChallengeExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		return
	}
	h.recordLogin(c, res.User.Email, res.User.ID, constants.LoginLogStatusSuccess, "", constants.LoginLogSourceTelegram)
	response.Success(c, gin.H{
		"requires_totp": false,
		"user":          userpresenter.NewUserAuthBriefResp(res.User),
		"token":         res.Token,
		"expires_at":    res.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// StartTelegramOIDCBind 返回 Telegram OIDC 授权 URL（绑定流程，需登录）。
func (h *UserTelegramOIDCHandler) StartTelegramOIDCBind(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	authURL, err := h.service.StartTelegramOIDC(c.Request.Context(), "bind", uid)
	if err != nil {
		respondTelegramOIDCError(c, err)
		return
	}
	response.Success(c, gin.H{"auth_url": authURL})
}

// TelegramOIDCBindCallback 处理 Telegram OIDC 回调（绑定，需登录）。
func (h *UserTelegramOIDCHandler) TelegramOIDCBindCallback(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req telegramOIDCCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	identity, err := h.service.BindTelegramOIDC(c.Request.Context(), uid, req.Code, req.State)
	if err != nil {
		respondTelegramOIDCError(c, err)
		return
	}
	response.Success(c, userpresenter.NewTelegramBindingResp(identity))
}
