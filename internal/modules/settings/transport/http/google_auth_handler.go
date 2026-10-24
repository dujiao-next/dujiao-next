package settingshttp

import (
	"github.com/gin-gonic/gin"

	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

// GoogleAuthAdminService 是后台 Google Identity Services 登录设置端口。
type GoogleAuthAdminService interface {
	GetGoogleAuthSetting() (settingssecurity.GoogleAuthSetting, error)
	PatchGoogleAuthSetting(patch settingssecurity.GoogleAuthSettingPatch) (settingssecurity.GoogleAuthSetting, error)
	ApplyRuntime(setting settingssecurity.GoogleAuthSetting)
}

// GoogleAuthHandler 处理后台 Google Identity Services 登录设置请求。
type GoogleAuthHandler struct {
	googleAuth GoogleAuthAdminService
}

func NewGoogleAuthHandler(googleAuth GoogleAuthAdminService) *GoogleAuthHandler {
	if googleAuth == nil {
		panic("settings google auth handler: googleAuth is nil")
	}
	return &GoogleAuthHandler{googleAuth: googleAuth}
}

// GetGoogleAuth 获取配置（脱敏）。
func (h *GoogleAuthHandler) GetGoogleAuth(c *gin.Context) {
	respondSecuritySetting(c, h.googleAuth.GetGoogleAuthSetting, settingssecurity.MaskGoogleAuthSettingForAdmin)
}

// UpdateGoogleAuth 更新配置并热更新运行时。
func (h *GoogleAuthHandler) UpdateGoogleAuth(c *gin.Context) {
	updateSecuritySetting(c, h.googleAuth.PatchGoogleAuthSetting, h.googleAuth.ApplyRuntime, settingssecurity.MaskGoogleAuthSettingForAdmin, settingssecurity.ErrGoogleAuthConfigInvalid)
}
