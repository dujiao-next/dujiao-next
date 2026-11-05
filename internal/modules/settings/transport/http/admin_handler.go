package settingshttp

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台通用设置端口。
type AdminService interface {
	GetByKey(key string) (jsonmap.JSON, error)
	UpdateWithEffects(key string, value map[string]interface{}) (settingsapp.UpdateResult, error)
	InvalidateCallbackRoutesCache()
}

// AdminHandler 处理后台通用设置请求。
type AdminHandler struct {
	settings AdminService
}

func NewAdminHandler(settings AdminService) *AdminHandler {
	if settings == nil {
		panic("settings admin handler: settings is nil")
	}
	return &AdminHandler{settings: settings}
}

type updateRequest struct {
	Key   string                 `json:"key" binding:"required"`
	Value map[string]interface{} `json:"value" binding:"required"`
}

// Get 获取设置。
func (h *AdminHandler) Get(c *gin.Context) {
	key := c.DefaultQuery("key", constants.SettingKeySiteConfig)

	value, err := h.settings.GetByKey(key)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	if value == nil {
		response.Success(c, gin.H{})
		return
	}

	response.Success(c, value)
}

// settingFieldErrorKeys 复用分销站同名字段的文案，两边校验规则一致。
var settingFieldErrorKeys = map[string]string{
	"contact_telegram": "error.reseller_support_telegram_invalid",
	"contact_whatsapp": "error.reseller_support_whatsapp_invalid",
}

// dedicatedSettingEndpoints 有专用接口的设置：专用接口负责校验、归一化与运行时生效（如 SMTP、登录方式），
// 走通用接口会跳过这些步骤，导致配置不生效或写入未校验的值。
var dedicatedSettingEndpoints = map[string]string{
	constants.SettingKeySMTPConfig:               "/admin/settings/smtp",
	constants.SettingKeyCaptchaConfig:            "/admin/settings/captcha",
	constants.SettingKeyTelegramAuthConfig:       "/admin/settings/telegram-auth",
	constants.SettingKeyGoogleAuthConfig:         "/admin/settings/google-auth",
	constants.SettingKeyAffiliateConfig:          "/admin/settings/affiliate",
	constants.SettingKeyOrderEmailTemplateConfig: "/admin/settings/order-email-template",
	constants.SettingKeyTelegramBotConfig:        "/admin/settings/telegram-bot",
	constants.SettingKeyNotificationCenterConfig: "/admin/settings/notification-center",
}

// Update 更新设置。
func (h *AdminHandler) Update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	key := strings.TrimSpace(req.Key)
	if endpoint, ok := dedicatedSettingEndpoints[key]; ok {
		ginutil.RespondErrorWithMsg(
			c,
			response.CodeBadRequest,
			key+" must be updated through "+endpoint,
			nil,
		)
		return
	}

	result, err := h.settings.UpdateWithEffects(req.Key, req.Value)
	if err != nil {
		var fieldErr *settingsapp.SettingFieldError
		if errors.As(err, &fieldErr) {
			if key, ok := settingFieldErrorKeys[fieldErr.Field]; ok {
				ginutil.RespondError(c, response.CodeBadRequest, key, nil)
				return
			}
		}
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}

	if result.HasEffect(settingsapp.EffectInvalidatePublicConfigCache) {
		_ = cache.DelAllPublicConfig(c.Request.Context())
	}
	if result.HasEffect(settingsapp.EffectInvalidateCallbackRoutesCache) {
		h.settings.InvalidateCallbackRoutesCache()
	}
	response.Success(c, result.Value)
}

// respondSecuritySetting 返回脱敏后的安全类设置（验证码 / Google / Telegram 登录）。
func respondSecuritySetting[S any, M any](c *gin.Context, get func() (S, error), mask func(S) M) {
	setting, err := get()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, mask(setting))
}

// updateSecuritySetting 校验并保存安全类设置，随后热更新运行时并清公共配置缓存。
func updateSecuritySetting[P any, S any, M any](c *gin.Context, patch func(P) (S, error), apply func(S), mask func(S) M, invalid error) {
	var req P
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	setting, err := patch(req)
	if err != nil {
		if errors.Is(err, invalid) {
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	apply(setting)
	_ = cache.DelAllPublicConfig(c.Request.Context())
	response.Success(c, mask(setting))
}
