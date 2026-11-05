package channelhttp

import (
	"strings"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/gin-gonic/gin"
)

type telegramIdentityRequest struct {
	ChannelUserID  string `json:"channel_user_id"`
	TelegramUserID string `json:"telegram_user_id"`
	Username       string `json:"username"`
	TelegramUser   string `json:"telegram_username"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	AvatarURL      string `json:"avatar_url"`
	Locale         string `json:"locale"`
	InviteCode     string `json:"invite_code"`
}

type telegramBindRequest struct {
	ChannelUserID  string `json:"channel_user_id"`
	TelegramUserID string `json:"telegram_user_id"`
	Username       string `json:"username"`
	TelegramUser   string `json:"telegram_username"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	AvatarURL      string `json:"avatar_url"`
	BindMode       string `json:"bind_mode"`
	Email          string `json:"email"`
	Code           string `json:"code"`
}

// ResolveTelegramIdentity POST /api/v1/channel/identities/telegram/resolve
func (h *Handler) ResolveTelegramIdentity(c *gin.Context) {
	var req telegramIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}

	input := buildTelegramChannelIdentityInput(req)
	if strings.TrimSpace(input.ChannelUserID) == "" {
		respondChannelError(c, 400, 400, "validation_error", "error.bad_request", nil)
		return
	}

	user, identity, err := h.UserAuthService.ResolveTelegramChannelIdentity(input)
	if err != nil {
		respondChannelIdentityServiceError(c, err)
		return
	}
	if identity == nil || user == nil {
		respondChannelSuccess(c, gin.H{"bound": false})
		return
	}

	respondChannelSuccess(c, buildChannelIdentityResponse(true, false, user, identity))
}

// ProvisionTelegramIdentity POST /api/v1/channel/identities/telegram/provision
func (h *Handler) ProvisionTelegramIdentity(c *gin.Context) {
	var req telegramIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}

	input := buildTelegramChannelIdentityInput(req)
	if strings.TrimSpace(input.ChannelUserID) == "" {
		respondChannelError(c, 400, 400, "validation_error", "error.bad_request", nil)
		return
	}
	input.InviteCode = strings.TrimSpace(req.InviteCode)

	user, identity, created, err := h.UserAuthService.ProvisionTelegramChannelIdentity(input)
	if err != nil {
		respondChannelIdentityServiceError(c, err)
		return
	}

	respondChannelSuccess(c, buildChannelIdentityResponse(true, created, user, identity))
}

// BindTelegramIdentity POST /api/v1/channel/identities/telegram/bind
func (h *Handler) BindTelegramIdentity(c *gin.Context) {
	var req telegramBindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}
	if mode := strings.ToLower(strings.TrimSpace(req.BindMode)); mode != "" && mode != "email_code" {
		respondChannelError(c, 400, 400, "validation_error", "error.bad_request", nil)
		return
	}

	user, identity, previousUserID, err := h.UserAuthService.BindTelegramChannelByEmailCode(BindTelegramIdentityInput{
		Identity: telegramChannelIdentityInput(
			req.ChannelUserID,
			req.TelegramUserID,
			req.Username,
			req.TelegramUser,
			req.FirstName,
			req.LastName,
			req.AvatarURL,
		),
		Email: req.Email,
		Code:  req.Code,
	})
	if err != nil {
		respondChannelIdentityServiceError(c, err)
		return
	}

	resp := buildChannelIdentityResponse(true, false, user, identity)
	resp["bound"] = true
	if previousUserID != 0 {
		resp["previous_user_id"] = previousUserID
	}
	respondChannelSuccess(c, resp)
}

// GetCurrentIdentity GET /api/v1/channel/me
func (h *Handler) GetCurrentIdentity(c *gin.Context) {
	input := TelegramIdentityInput{
		ChannelUserID: channelUserIDFromQuery(c),
		Username:      strings.TrimSpace(c.Query("username")),
		AvatarURL:     strings.TrimSpace(c.Query("avatar_url")),
	}
	if strings.TrimSpace(input.ChannelUserID) == "" {
		respondChannelError(c, 400, 400, "validation_error", "error.bad_request", nil)
		return
	}

	user, identity, err := h.UserAuthService.ResolveTelegramChannelIdentity(input)
	if err != nil {
		respondChannelIdentityServiceError(c, err)
		return
	}
	if identity == nil || user == nil {
		respondChannelSuccess(c, gin.H{"bound": false})
		return
	}

	respondChannelSuccess(c, buildChannelIdentityResponse(true, false, user, identity))
}

func buildTelegramChannelIdentityInput(req telegramIdentityRequest) TelegramIdentityInput {
	return telegramChannelIdentityInput(
		req.ChannelUserID,
		req.TelegramUserID,
		req.Username,
		req.TelegramUser,
		req.FirstName,
		req.LastName,
		req.AvatarURL,
	)
}

func telegramChannelIdentityInput(channelUserID, legacyUserID, username, legacyUsername, firstName, lastName, avatarURL string) TelegramIdentityInput {
	return TelegramIdentityInput{
		ChannelUserID: channelUserIDValue(channelUserID, legacyUserID),
		Username:      strings.TrimSpace(firstNonEmpty(username, legacyUsername)),
		FirstName:     strings.TrimSpace(firstName),
		LastName:      strings.TrimSpace(lastName),
		AvatarURL:     strings.TrimSpace(avatarURL),
	}
}

func (h *Handler) provisionTelegramChannelUserID(input TelegramIdentityInput) (uint, error) {
	return h.UserAuthService.ProvisionTelegramChannelUserID(input)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildChannelIdentityResponse(bound, created bool, user *userdomain.User, identity *externalidentitydomain.Identity) gin.H {
	resp := gin.H{"bound": bound}
	if identity != nil {
		resp["identity"] = gin.H{
			"provider":         identity.Provider,
			"provider_user_id": identity.ProviderUserID,
			"username":         identity.Username,
			"avatar_url":       identity.AvatarURL,
		}
	}
	if user != nil {
		resp["user"] = gin.H{
			"id":                      user.ID,
			"email":                   user.Email,
			"display_name":            user.DisplayName,
			"status":                  user.Status,
			"locale":                  user.Locale,
			"email_verified":          user.EmailVerifiedAt != nil,
			"password_setup_required": user.PasswordSetupRequired,
		}
	}
	if bound {
		resp["created"] = created
	}
	return resp
}
