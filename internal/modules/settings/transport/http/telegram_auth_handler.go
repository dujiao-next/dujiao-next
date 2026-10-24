package settingshttp

import (
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"

	"github.com/gin-gonic/gin"
)

// TelegramAuthAdminService 是后台 Telegram 登录设置端口。
type TelegramAuthAdminService interface {
	GetTelegramAuthSetting() (settingssecurity.TelegramAuthSetting, error)
	PatchTelegramAuthSetting(patch settingssecurity.TelegramAuthSettingPatch) (settingssecurity.TelegramAuthSetting, error)
	ApplyRuntime(setting settingssecurity.TelegramAuthSetting)
}

// TelegramAuthHandler 处理后台 Telegram 登录设置请求。
type TelegramAuthHandler struct {
	telegramAuth TelegramAuthAdminService
}

func NewTelegramAuthHandler(telegramAuth TelegramAuthAdminService) *TelegramAuthHandler {
	if telegramAuth == nil {
		panic("settings telegram auth handler: telegramAuth is nil")
	}
	return &TelegramAuthHandler{telegramAuth: telegramAuth}
}

// GetTelegramAuth 获取配置（脱敏）。
func (h *TelegramAuthHandler) GetTelegramAuth(c *gin.Context) {
	respondSecuritySetting(c, h.telegramAuth.GetTelegramAuthSetting, settingssecurity.MaskTelegramAuthSettingForAdmin)
}

// UpdateTelegramAuth 更新配置并热更新运行时。
func (h *TelegramAuthHandler) UpdateTelegramAuth(c *gin.Context) {
	updateSecuritySetting(c, h.telegramAuth.PatchTelegramAuthSetting, h.telegramAuth.ApplyRuntime, settingssecurity.MaskTelegramAuthSettingForAdmin, settingssecurity.ErrTelegramAuthConfigInvalid)
}
