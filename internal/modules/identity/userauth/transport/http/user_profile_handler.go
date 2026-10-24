package userauthhttp

import (
	"encoding/json"
	"errors"
	"github.com/dujiao-next/internal/i18n"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
)

var (
	ErrProfileEmpty        = errors.New("user profile empty")
	ErrNicknameInvalid     = errors.New("user nickname invalid")
	ErrUserNotFound        = errors.New("user not found")
	ErrUnifiedAuthRequired = errors.New("unified authentication required")
)

// UserProfileService 是用户资料端点所需的最小端口。
type UserProfileService interface {
	GetUserByID(id uint) (*userdomain.User, error)
	ResolveEmailChangeMode(user *userdomain.User) (string, error)
	ResolvePasswordChangeMode(user *userdomain.User) (string, error)
	UpdateProfile(userID uint, nickname, locale *string) (*userdomain.User, error)
	CheckLocalIdentityManagement() error
	GetProfileAvatar(userID uint) (string, error)
}

// UserProfileHandler 处理当前用户资料 HTTP 请求。
type UserProfileHandler struct {
	service UserProfileService
}

func NewUserProfileHandler(service UserProfileService) *UserProfileHandler {
	if service == nil {
		panic("user profile handler: service is nil")
	}
	return &UserProfileHandler{service: service}
}

// UserProfileUpdateRequest 更新资料请求。
type UserProfileUpdateRequest struct {
	Nickname *string `json:"nickname"`
	Locale   *string `json:"locale"`
}

// GetCurrentUser 获取当前用户信息。
func (h *UserProfileHandler) GetCurrentUser(c *gin.Context) {
	id, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	user, err := h.service.GetUserByID(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if user == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		return
	}

	profile, err := h.userProfileResponse(user)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	response.Success(c, profile)
}

func (h *UserProfileHandler) userProfileResponse(user *userdomain.User) (userpresenter.UserProfileResp, error) {
	emailMode, err := h.service.ResolveEmailChangeMode(user)
	if err != nil {
		return userpresenter.UserProfileResp{}, err
	}
	passwordMode, err := h.service.ResolvePasswordChangeMode(user)
	if err != nil {
		return userpresenter.UserProfileResp{}, err
	}
	profile := userpresenter.NewUserProfileResp(user, emailMode, passwordMode)
	profile.AvatarURL, err = h.service.GetProfileAvatar(user.ID)
	if err != nil {
		return userpresenter.UserProfileResp{}, err
	}
	return profile, nil
}

// UpdateUserProfile 更新用户资料。
func (h *UserProfileHandler) UpdateUserProfile(c *gin.Context) {
	id, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req UserProfileUpdateRequest
	if err := c.ShouldBindBodyWith(&req, binding.JSON); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	var fields map[string]json.RawMessage
	if err := c.ShouldBindBodyWith(&fields, binding.JSON); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if ginutil.HasJSONField(fields, "nickname", "display_name", "username", "email", "password", "avatar", "avatar_url", "email_verified", "email_verified_at", "auth_bindings", "auth_identities", "oauth_identities", "identities", "identity_bindings") {
		if err := h.service.CheckLocalIdentityManagement(); err != nil {
			if errors.Is(err, ErrUnifiedAuthRequired) {
				response.ErrorWithHTTPStatus(c, response.CodeForbidden, response.CodeForbidden, i18n.T(i18n.ResolveLocale(c), "error.unified_auth_required"))
			} else {
				ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
			}
			return
		}
	}

	user, err := h.service.UpdateProfile(id, req.Nickname, req.Locale)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnifiedAuthRequired):
			response.ErrorWithHTTPStatus(c, response.CodeForbidden, response.CodeForbidden, i18n.T(i18n.ResolveLocale(c), "error.unified_auth_required"))
		case errors.Is(err, ErrProfileEmpty):
			ginutil.RespondError(c, response.CodeBadRequest, "error.profile_empty", nil)
		case errors.Is(err, ErrNicknameInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.nickname_invalid", nil)
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}

	profile, err := h.userProfileResponse(user)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		return
	}
	response.Success(c, profile)
}
