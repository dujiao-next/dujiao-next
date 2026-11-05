package userauthhttp

import (
	"context"
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrTelegramAuthSignatureInvalid = errors.New("telegram auth signature invalid")
	ErrUserOAuthNotBound            = errors.New("user oauth not bound")
	ErrTelegramUnbindRequiresEmail  = errors.New("telegram unbind requires real email")
)

// TelegramAuthPayload 是 Telegram Login Widget 鉴权载荷。
type TelegramAuthPayload struct {
	ID        int64
	FirstName string
	LastName  string
	Username  string
	PhotoURL  string
	AuthDate  int64
	Hash      string
}

// TelegramBindingResult is the transport view returned by the application adapter.
type TelegramBindingResult struct {
	Identity  *externalidentitydomain.Identity
	CanUnbind bool
}

// UserTelegramService 是 Telegram widget/MiniApp 端点所需的最小端口。
type UserTelegramService interface {
	LoginWithTelegram(ctx context.Context, payload TelegramAuthPayload) (*AuthLoginResult, error)
	LoginWithTelegramMiniApp(ctx context.Context, initData string) (*AuthLoginResult, error)
	GetTelegramBinding(userID uint) (*TelegramBindingResult, error)
	BindTelegram(ctx context.Context, userID uint, payload TelegramAuthPayload) (*externalidentitydomain.Identity, error)
	BindTelegramMiniApp(ctx context.Context, userID uint, initData string) (*externalidentitydomain.Identity, error)
	UnbindTelegram(userID uint) error
}

// UserTelegramHandler 处理 Telegram widget/MiniApp 登录与绑定 HTTP 请求。
type UserTelegramHandler struct {
	service  UserTelegramService
	recorder LoginRecorder
}

func NewUserTelegramHandler(service UserTelegramService, recorder LoginRecorder) *UserTelegramHandler {
	if service == nil {
		panic("user telegram handler: service is nil")
	}
	return &UserTelegramHandler{service: service, recorder: recorder}
}

// UserTelegramLoginRequest Telegram 登录请求。
type UserTelegramLoginRequest struct {
	ID        int64  `json:"id" binding:"required"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date" binding:"required"`
	Hash      string `json:"hash" binding:"required"`
}

// UserTelegramMiniAppAuthRequest Telegram Mini App 鉴权请求。
type UserTelegramMiniAppAuthRequest struct {
	InitData      string `json:"init_data"`
	InitDataCamel string `json:"initData"`
}

// UserBindTelegramRequest 绑定 Telegram 请求。
type UserBindTelegramRequest struct {
	ID        int64  `json:"id" binding:"required"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	PhotoURL  string `json:"photo_url"`
	AuthDate  int64  `json:"auth_date" binding:"required"`
	Hash      string `json:"hash" binding:"required"`
}

func (r UserTelegramMiniAppAuthRequest) initData() string {
	if strings.TrimSpace(r.InitData) != "" {
		return strings.TrimSpace(r.InitData)
	}
	return strings.TrimSpace(r.InitDataCamel)
}

func (r UserTelegramLoginRequest) payload() TelegramAuthPayload {
	return TelegramAuthPayload(r)
}

func (r UserBindTelegramRequest) payload() TelegramAuthPayload {
	return TelegramAuthPayload(r)
}

type telegramLoginErrorRule struct {
	target     error
	code       int
	key        string
	failReason string
	logErr     bool
}

var telegramLoginErrorRules = []telegramLoginErrorRule{
	{target: ErrTelegramAuthDisabled, code: response.CodeBadRequest, key: "error.telegram_auth_disabled", failReason: constants.LoginLogFailReasonTelegramConfig},
	{target: ErrTelegramAuthConfigInvalid, code: response.CodeInternal, key: "error.telegram_auth_config_invalid", failReason: constants.LoginLogFailReasonTelegramConfig, logErr: true},
	{target: ErrTelegramAuthPayloadInvalid, code: response.CodeBadRequest, key: "error.telegram_auth_payload_invalid", failReason: constants.LoginLogFailReasonTelegramInvalid},
	{target: ErrTelegramAuthSignatureInvalid, code: response.CodeBadRequest, key: "error.telegram_auth_signature_invalid", failReason: constants.LoginLogFailReasonTelegramInvalid},
	{target: ErrTelegramAuthExpired, code: response.CodeBadRequest, key: "error.telegram_auth_expired", failReason: constants.LoginLogFailReasonTelegramExpired},
	{target: ErrTelegramAuthReplay, code: response.CodeBadRequest, key: "error.telegram_auth_replayed", failReason: constants.LoginLogFailReasonTelegramReplayed},
	{target: ErrUserDisabled, code: response.CodeUnauthorized, key: "error.user_disabled", failReason: constants.LoginLogFailReasonUserDisabled},
	{target: ErrRegistrationDisabled, code: response.CodeForbidden, key: "error.registration_disabled", failReason: constants.LoginLogFailReasonBadRequest},
}

func (h *UserTelegramHandler) recordLogin(c *gin.Context, email string, userID uint, status, failReason, source string) {
	if h != nil {
		recordLoginEvent(h.recorder, c, email, userID, status, failReason, source)
	}
}

func (h *UserTelegramHandler) respondTelegramLoginError(c *gin.Context, err error) {
	for _, rule := range telegramLoginErrorRules {
		if errors.Is(err, rule.target) {
			h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, rule.failReason, constants.LoginLogSourceTelegram)
			var cause error
			if rule.logErr {
				cause = err
			}
			ginutil.RespondError(c, rule.code, rule.key, cause)
			return
		}
	}
	h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonInternalError, constants.LoginLogSourceTelegram)
	ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
}

func respondTelegramBindError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrTelegramAuthDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_disabled", nil)
	case errors.Is(err, ErrTelegramAuthConfigInvalid):
		ginutil.RespondError(c, response.CodeInternal, "error.telegram_auth_config_invalid", err)
	case errors.Is(err, ErrTelegramAuthPayloadInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_payload_invalid", nil)
	case errors.Is(err, ErrTelegramAuthSignatureInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_signature_invalid", nil)
	case errors.Is(err, ErrTelegramAuthExpired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_expired", nil)
	case errors.Is(err, ErrTelegramAuthReplay):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_auth_replayed", nil)
	case errors.Is(err, ErrUserOAuthIdentityExists):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_bind_conflict", nil)
	case errors.Is(err, ErrUserOAuthAlreadyBound):
		ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_already_bound", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
	}
}

func respondTelegramLoginSuccess(c *gin.Context, h *UserTelegramHandler, res *AuthLoginResult) {
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

// UserTelegramLogin Telegram 登录。
func (h *UserTelegramHandler) UserTelegramLogin(c *gin.Context) {
	var req UserTelegramLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonBadRequest, constants.LoginLogSourceTelegram)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	res, err := h.service.LoginWithTelegram(c.Request.Context(), req.payload())
	if err != nil {
		h.respondTelegramLoginError(c, err)
		return
	}
	respondTelegramLoginSuccess(c, h, res)
}

// UserTelegramMiniAppLogin Telegram Mini App 登录。
func (h *UserTelegramHandler) UserTelegramMiniAppLogin(c *gin.Context) {
	var req UserTelegramMiniAppAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.initData() == "" {
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonBadRequest, constants.LoginLogSourceTelegram)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	res, err := h.service.LoginWithTelegramMiniApp(c.Request.Context(), req.initData())
	if err != nil {
		h.respondTelegramLoginError(c, err)
		return
	}
	respondTelegramLoginSuccess(c, h, res)
}

// GetMyTelegramBinding 获取当前用户 Telegram 绑定。
func (h *UserTelegramHandler) GetMyTelegramBinding(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	binding, err := h.service.GetTelegramBinding(uid)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		}
		return
	}
	if binding == nil || binding.Identity == nil {
		response.Success(c, userpresenter.NewTelegramBindingResp(nil, false))
		return
	}
	response.Success(c, userpresenter.NewTelegramBindingResp(binding.Identity, binding.CanUnbind))
}

// BindMyTelegram 绑定当前用户 Telegram。
func (h *UserTelegramHandler) BindMyTelegram(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req UserBindTelegramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	identity, err := h.service.BindTelegram(c.Request.Context(), uid, req.payload())
	if err != nil {
		respondTelegramBindError(c, err)
		return
	}
	binding, getErr := h.service.GetTelegramBinding(uid)
	if getErr != nil || binding == nil {
		response.Success(c, userpresenter.NewTelegramBindingResp(identity, false))
		return
	}
	response.Success(c, userpresenter.NewTelegramBindingResp(binding.Identity, binding.CanUnbind))
}

// BindMyTelegramMiniApp 绑定当前用户的 Telegram Mini App 身份。
func (h *UserTelegramHandler) BindMyTelegramMiniApp(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req UserTelegramMiniAppAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.initData() == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	identity, err := h.service.BindTelegramMiniApp(c.Request.Context(), uid, req.initData())
	if err != nil {
		respondTelegramBindError(c, err)
		return
	}
	binding, getErr := h.service.GetTelegramBinding(uid)
	if getErr != nil || binding == nil {
		response.Success(c, userpresenter.NewTelegramBindingResp(identity, false))
		return
	}
	response.Success(c, userpresenter.NewTelegramBindingResp(binding.Identity, binding.CanUnbind))
}

// UnbindMyTelegram 解绑当前用户 Telegram。
func (h *UserTelegramHandler) UnbindMyTelegram(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	if err := h.service.UnbindTelegram(uid); err != nil {
		switch {
		case errors.Is(err, ErrUserOAuthNotBound):
			ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_not_bound", nil)
		case errors.Is(err, ErrTelegramUnbindRequiresEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.telegram_unbind_requires_email", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"unbound": true})
}
