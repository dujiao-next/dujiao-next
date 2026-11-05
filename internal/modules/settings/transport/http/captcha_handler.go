package settingshttp

import (
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"

	"github.com/gin-gonic/gin"
)

// CaptchaAdminService 是后台验证码设置端口。
type CaptchaAdminService interface {
	GetCaptchaSetting() (settingssecurity.CaptchaSetting, error)
	PatchCaptchaSetting(patch settingssecurity.CaptchaSettingPatch) (settingssecurity.CaptchaSetting, error)
	ApplyRuntime(setting settingssecurity.CaptchaSetting)
}

// CaptchaHandler 处理后台验证码设置请求。
type CaptchaHandler struct {
	captcha CaptchaAdminService
}

func NewCaptchaHandler(captcha CaptchaAdminService) *CaptchaHandler {
	if captcha == nil {
		panic("settings captcha handler: captcha is nil")
	}
	return &CaptchaHandler{captcha: captcha}
}

// GetCaptcha 获取配置（脱敏）。
func (h *CaptchaHandler) GetCaptcha(c *gin.Context) {
	respondSecuritySetting(c, h.captcha.GetCaptchaSetting, settingssecurity.MaskCaptchaSettingForAdmin)
}

// UpdateCaptcha 更新配置并热更新运行时。
func (h *CaptchaHandler) UpdateCaptcha(c *gin.Context) {
	updateSecuritySetting(c, h.captcha.PatchCaptchaSetting, h.captcha.ApplyRuntime, settingssecurity.MaskCaptchaSettingForAdmin, captcha.ErrConfigInvalid)
}
