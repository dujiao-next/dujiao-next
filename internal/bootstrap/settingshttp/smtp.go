package settingsbootstrap

import (
	"errors"

	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	notificationsmtp "github.com/dujiao-next/internal/modules/notification/infrastructure/smtp"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"

	"github.com/dujiao-next/internal/config"
	settingstransport "github.com/dujiao-next/internal/modules/settings/transport/http"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

type settingsSMTPAdapter struct {
	settings *settingsapp.Service
	cfg      *config.Config
	email    *notificationsmtp.Service
	brand    mailbrand.Resolver
}

func (a settingsSMTPAdapter) GetSMTPSetting() (settingsmessaging.SMTPSetting, error) {
	return a.settings.GetSMTPSetting(a.cfg.Email)
}

func (a settingsSMTPAdapter) PatchSMTPSetting(patch settingsmessaging.SMTPSettingPatch) (settingsmessaging.SMTPSetting, error) {
	return a.settings.PatchSMTPSetting(a.cfg.Email, patch)
}

func (a settingsSMTPAdapter) ApplyRuntime(setting settingsmessaging.SMTPSetting) {
	a.cfg.Email = settingsmessaging.SMTPSettingToConfig(setting)
	if a.email != nil {
		a.email.SetConfig(&a.cfg.Email)
	}
}

func (a settingsSMTPAdapter) SendTest(setting settingsmessaging.SMTPSetting, toEmail, subject, body string) error {
	configForSend := settingsmessaging.SMTPSettingToConfig(setting)
	configForSend.Enabled = true
	err := notificationsmtp.New(&configForSend, a.brand).SendCustomEmail(toEmail, subject, body)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, notificationcontract.ErrInvalidEmail):
		return settingstransport.ErrSMTPTestInvalidEmail
	case errors.Is(err, notificationcontract.ErrEmailRecipientRejected):
		return settingstransport.ErrSMTPTestRecipientRejected
	case errors.Is(err, notificationcontract.ErrEmailServiceDisabled):
		return settingstransport.ErrSMTPTestServiceDisabled
	case errors.Is(err, notificationcontract.ErrEmailNotConfigured):
		return settingstransport.ErrSMTPTestServiceNotConfigured
	default:
		return err
	}
}
