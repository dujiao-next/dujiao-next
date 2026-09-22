package publicconfigwiring

import (
	"github.com/dujiao-next/internal/app/container"
	publicconfigtransport "github.com/dujiao-next/internal/modules/settings/transport/http/public"
)

func NewHandler(c *container.Container) *publicconfigtransport.Handler {
	var captcha publicconfigtransport.CaptchaPublic
	if c.CaptchaService != nil {
		captcha = publicConfigCaptchaAdapter{svc: c.CaptchaService}
	}
	var telegram publicconfigtransport.TelegramAuthPublic
	if c.TelegramAuthService != nil {
		telegram = publicConfigTelegramAdapter{svc: c.TelegramAuthService}
	}
	var google publicconfigtransport.GoogleAuthPublic
	if c.GoogleAuthService != nil {
		google = publicConfigGoogleAdapter{svc: c.GoogleAuthService}
	}
	var github publicconfigtransport.GitHubAuthPublic
	if c.GitHubAuthService != nil {
		github = publicConfigGitHubAdapter{svc: c.GitHubAuthService}
	}
	var overlay publicconfigtransport.ResellerOverlay
	if c.ResellerSiteConfigService != nil {
		overlay = publicConfigResellerOverlayAdapter{svc: c.ResellerSiteConfigService}
	}
	fallback := publicconfigtransport.TelegramAuthFallback{}
	googleFallback := publicconfigtransport.GoogleAuthFallback{}
	githubFallback := publicconfigtransport.GitHubAuthFallback{}
	if c.Config != nil {
		fallback = publicconfigtransport.TelegramAuthFallback{
			Enabled:     c.Config.TelegramAuth.Enabled,
			BotUsername: c.Config.TelegramAuth.BotUsername,
			MiniAppURL:  c.Config.TelegramAuth.MiniAppURL,
		}
		googleFallback = publicconfigtransport.GoogleAuthFallback{
			Enabled:  c.Config.GoogleAuth.Enabled,
			ClientID: c.Config.GoogleAuth.ClientID,
		}
		githubFallback = publicconfigtransport.GitHubAuthFallback{
			Enabled: c.Config.GitHubAuth.Enabled, ClientID: c.Config.GitHubAuth.ClientID,
		}
	}
	return publicconfigtransport.NewHandler(
		publicConfigCacheAdapter{},
		publicConfigSettingsAdapter{settings: c.SettingService, cfg: c.Config},
		publicConfigPaymentAdapter{payments: c.PaymentService},
		captcha,
		telegram,
		fallback,
		google,
		googleFallback,
		github,
		githubFallback,
		overlay,
	)
}
