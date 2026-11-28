package adminauthhttp

import (
	"errors"
	"time"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	captchahttp "github.com/dujiao-next/internal/modules/captcha/transport/http"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/modules/identity/jwttoken"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrWeakPassword       = errors.New("weak password")
)

// WeakPasswordError 携带可本地化的弱密码策略详情。
type WeakPasswordError struct {
	key  string
	args []interface{}
}

func NewWeakPasswordError(key string, args ...interface{}) error {
	if key == "" {
		return ErrWeakPassword
	}
	return WeakPasswordError{key: key, args: args}
}

func (e WeakPasswordError) Error() string { return e.key }

func (e WeakPasswordError) Is(target error) bool { return target == ErrWeakPassword }

func (e WeakPasswordError) Key() string { return e.key }

func (e WeakPasswordError) Args() []interface{} { return e.args }

// CaptchaVerifier 是登录端点所需的验证码端口。
type CaptchaVerifier interface {
	Verify(scene string, payload captchahttp.CaptchaPayloadRequest, clientIP string) error
}

// LoginAuthService 是管理员登录、改密与会话续期端口。
type LoginAuthService interface {
	Login(username, password string) (*AuthLoginResult, error)
	ChangePassword(adminID uint, oldPassword, newPassword string) error
	GetAdminByID(adminID uint) (*admindomain.Admin, error)
	RefreshJWT(admin *admindomain.Admin, sessionStartedAt int64) (string, time.Time, error)
}

// AdminLoginHandler 处理管理员登录与改密 HTTP 请求。
type AdminLoginHandler struct {
	auth     LoginAuthService
	captcha  CaptchaVerifier
	recorder AdminLoginRecorder
}

func NewAdminLoginHandler(auth LoginAuthService, captcha CaptchaVerifier, recorder AdminLoginRecorder) *AdminLoginHandler {
	if auth == nil {
		panic("admin login handler: auth is nil")
	}
	return &AdminLoginHandler{auth: auth, captcha: captcha, recorder: recorder}
}

func (h *AdminLoginHandler) writeLoginLog(c *gin.Context, adminID uint, username, eventType, status, failReason string, operatorID *uint) {
	if h == nil || h.recorder == nil || c == nil {
		return
	}
	requestID := ""
	if rid, ok := c.Get("request_id"); ok {
		if value, ok := rid.(string); ok {
			requestID = value
		}
	}
	h.recorder.Record(adminID, username, eventType, status, failReason, c.ClientIP(), c.Request.UserAgent(), requestID, operatorID)
}

// LoginRequest 登录请求。
type LoginRequest struct {
	Username       string                            `json:"username" binding:"required"`
	Password       string                            `json:"password" binding:"required"`
	CaptchaPayload captchahttp.CaptchaPayloadRequest `json:"captcha_payload"`
}

// AdminLogin 管理员登录。
func (h *AdminLoginHandler) AdminLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if h.captcha != nil {
		if captchaErr := h.captcha.Verify(constants.CaptchaSceneLogin, req.CaptchaPayload, c.ClientIP()); captchaErr != nil {
			switch {
			case errors.Is(captchaErr, captcha.ErrRequired):
				ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_required", nil)
				return
			case errors.Is(captchaErr, captcha.ErrInvalid):
				ginutil.RespondError(c, response.CodeBadRequest, "error.captcha_invalid", nil)
				return
			case errors.Is(captchaErr, captcha.ErrConfigInvalid):
				ginutil.RespondError(c, response.CodeInternal, "error.captcha_config_invalid", captchaErr)
				return
			default:
				ginutil.RespondError(c, response.CodeInternal, "error.captcha_verify_failed", captchaErr)
				return
			}
		}
	}

	loginRes, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		failReason := constants.AdminLoginFailInvalidCredentials
		if !errors.Is(err, ErrInvalidCredentials) {
			failReason = constants.AdminLoginFailInternal
		}
		h.writeLoginLog(c, 0, req.Username, constants.AdminLoginEventLoginPassword, constants.AdminLoginStatusFailed, failReason, nil)
		if errors.Is(err, ErrInvalidCredentials) {
			ginutil.RespondError(c, response.CodeUnauthorized, "error.admin_login_invalid", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
		return
	}

	if loginRes.RequiresTOTP {
		h.writeLoginLog(c, loginRes.Admin.ID, loginRes.Admin.Username, constants.AdminLoginEventLoginPassword, constants.AdminLoginStatusSuccess, "", nil)
		response.Success(c, gin.H{
			"requires_totp":        true,
			"challenge_token":      loginRes.ChallengeToken,
			"challenge_expires_at": loginRes.ChallengeExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		return
	}

	h.writeLoginLog(c, loginRes.Admin.ID, loginRes.Admin.Username, constants.AdminLoginEventLoginPassword, constants.AdminLoginStatusSuccess, "", nil)
	response.Success(c, gin.H{
		"requires_totp": false,
		"token":         loginRes.Token,
		"user": gin.H{
			"id":       loginRes.Admin.ID,
			"username": loginRes.Admin.Username,
		},
		"expires_at": loginRes.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// RefreshSession 为已登录管理员续签 token。
//
// 只在会话绝对生命周期内放行：会话起点（sst）由 JWT 携带并在中间件写入上下文，
// 连续续期不会把这个起点往后推，因此被盗 token 的可用窗口仍然有上限。
func (h *AdminLoginHandler) RefreshSession(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}

	admin, err := h.auth.GetAdminByID(adminID)
	if err != nil || admin == nil {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.token_invalid", err)
		return
	}

	if value, exists := c.Get("admin_token_version"); exists {
		if tokenVersion, cast := value.(uint64); cast && tokenVersion != admin.TokenVersion {
			ginutil.RespondError(c, response.CodeUnauthorized, "error.token_revoked", nil)
			return
		}
	}

	sessionStartedAt := int64(0)
	if value, exists := c.Get("admin_session_started_at"); exists {
		if startedAt, cast := value.(int64); cast {
			sessionStartedAt = startedAt
		}
	}

	token, expiresAt, err := h.auth.RefreshJWT(admin, sessionStartedAt)
	if err != nil {
		if errors.Is(err, jwttoken.ErrSessionExpired) {
			ginutil.RespondError(c, response.CodeUnauthorized, "error.session_expired", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
		return
	}

	response.Success(c, gin.H{
		"token":      token,
		"expires_at": expiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// UpdatePasswordRequest 修改密码请求。
type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// UpdateAdminPassword 修改管理员密码。
func (h *AdminLoginHandler) UpdateAdminPassword(c *gin.Context) {
	id, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}

	var req UpdatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.auth.ChangePassword(id, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, ErrInvalidPassword):
			ginutil.RespondError(c, response.CodeBadRequest, "error.password_old_invalid", nil)
		case errors.Is(err, ErrWeakPassword):
			respondWeakPassword(c, err)
		case errors.Is(err, ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}

	response.Success(c, nil)
}

func respondWeakPassword(c *gin.Context, err error) {
	locale := i18n.ResolveLocale(c)
	if perr, ok := err.(interface {
		Key() string
		Args() []interface{}
	}); ok {
		msg := i18n.Sprintf(locale, perr.Key(), perr.Args()...)
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, msg, nil)
		return
	}
	ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", nil)
}
