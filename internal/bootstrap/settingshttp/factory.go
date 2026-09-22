package settingsbootstrap

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/config"
	settingstransport "github.com/dujiao-next/internal/modules/settings/transport/http"
)

func NewSMTPHandler(c *container.Container, cfg *config.Config) *settingstransport.SMTPHandler {
	return settingstransport.NewSMTPHandler(settingsSMTPAdapter{
		settings: c.SettingService, cfg: cfg, email: c.EmailSender,
	})
}

func NewCaptchaHandler(c *container.Container, cfg *config.Config) *settingstransport.CaptchaHandler {
	return settingstransport.NewCaptchaHandler(settingsCaptchaAdapter{
		settings: c.SettingService, cfg: cfg, captcha: c.CaptchaService,
	})
}

func NewTelegramAuthHandler(c *container.Container, cfg *config.Config) *settingstransport.TelegramAuthHandler {
	return settingstransport.NewTelegramAuthHandler(settingsTelegramAuthAdapter{
		settings: c.SettingService, cfg: cfg, telegramAuth: c.TelegramAuthService,
	})
}

func NewGoogleAuthHandler(c *container.Container, cfg *config.Config) *settingstransport.GoogleAuthHandler {
	return settingstransport.NewGoogleAuthHandler(settingsGoogleAuthAdapter{
		settings: c.SettingService, cfg: cfg, googleAuth: c.GoogleAuthService,
	})
}

func NewGitHubAuthHandler(c *container.Container, cfg *config.Config) *settingstransport.GitHubAuthHandler {
	return settingstransport.NewGitHubAuthHandler(settingsGitHubAuthAdapter{
		settings: c.SettingService, cfg: cfg, gitHubAuth: c.GitHubAuthService,
	})
}
