package publicconfigwiring

import (
	"context"
	"time"

	paymentapp "github.com/dujiao-next/internal/modules/payment/application"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	captchaapp "github.com/dujiao-next/internal/modules/captcha/application"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	resellerapplication "github.com/dujiao-next/internal/modules/reseller/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type publicConfigCacheAdapter struct{}

func (publicConfigCacheAdapter) CacheKey(resellerID *uint) string {
	return cache.PublicConfigCacheKey(resellerID)
}

func (publicConfigCacheAdapter) GetJSON(ctx context.Context, key string, dest *map[string]interface{}) (bool, error) {
	return cache.GetJSON(ctx, key, dest)
}

func (publicConfigCacheAdapter) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return cache.SetJSON(ctx, key, value, ttl)
}

type publicConfigSettingsAdapter struct {
	settings *settingsapp.Service
	cfg      *config.Config
}

func (a publicConfigSettingsAdapter) GetConfig(defaults map[string]interface{}) (map[string]interface{}, error) {
	return a.settings.GetConfig(defaults)
}

func (a publicConfigSettingsAdapter) GetWalletRechargeChannelIDs() []uint {
	if a.settings == nil {
		return nil
	}
	return a.settings.GetWalletRechargeChannelIDs()
}

func (a publicConfigSettingsAdapter) GetWalletOnlyPayment() bool {
	return a.settings != nil && a.settings.GetWalletOnlyPayment()
}

func (a publicConfigSettingsAdapter) GetAffiliateSettingMap() (map[string]interface{}, error) {
	setting, err := a.settings.GetAffiliateSetting()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}(settingsintegration.EncodeAffiliateSetting(setting)), nil
}

func (a publicConfigSettingsAdapter) GetSMTPEnabled() bool {
	var emailCfg config.EmailConfig
	if a.cfg != nil {
		emailCfg = a.cfg.Email
	}
	smtpSetting, _ := a.settings.GetSMTPSetting(emailCfg)
	return smtpSetting.Enabled
}

func (a publicConfigSettingsAdapter) GetRegistrationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetRegistrationEnabled(defaultValue)
}

func (a publicConfigSettingsAdapter) GetEmailVerificationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetEmailVerificationEnabled(defaultValue)
}

func (a publicConfigSettingsAdapter) GetRegistrationEmailDomainPolicy() (bool, []string, error) {
	policy, err := a.settings.GetRegistrationEmailDomainPolicy()
	if err != nil {
		return false, nil, err
	}
	return policy.Enabled, policy.AllowedDomains, nil
}

func (a publicConfigSettingsAdapter) GetByKey(key string) (interface{}, error) {
	value, err := a.settings.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return value, nil
}

func (a publicConfigSettingsAdapter) GetActiveHomeAnnouncement() (jsonmap.JSON, bool) {
	return a.settings.GetActiveHomeAnnouncement()
}

func (a publicConfigSettingsAdapter) GetActiveHomepageAd() (jsonmap.JSON, bool) {
	return a.settings.GetActiveHomepageAd()
}

type publicConfigPaymentAdapter struct {
	payments *paymentapp.PaymentService
}

func (a publicConfigPaymentAdapter) GetOrderPaymentChannels() ([]map[string]interface{}, error) {
	return a.payments.GetAvailableChannels(paymentapp.AvailablePaymentChannelFilter{
		PaymentType: constants.PaymentTypeOrder,
	})
}

type publicConfigCaptchaAdapter struct {
	svc *captchaapp.Service
}

func (a publicConfigCaptchaAdapter) GetPublicSetting() (jsonmap.JSON, error) {
	return a.svc.GetPublicSetting()
}

type publicConfigTelegramAdapter struct {
	svc *telegramauthapp.Service
}

func (a publicConfigTelegramAdapter) PublicConfig() map[string]interface{} {
	return a.svc.PublicConfig()
}

type publicConfigGoogleAdapter struct {
	svc *googleauthapp.Service
}

func (a publicConfigGoogleAdapter) PublicConfig() map[string]interface{} {
	return a.svc.PublicConfig()
}

type publicConfigResellerOverlayAdapter struct {
	svc *resellerapplication.SiteConfigService
}

func (a publicConfigResellerOverlayAdapter) ApplyPublicConfigOverlay(ctx context.Context, tenant resellercontract.TenantContext, base map[string]interface{}) (map[string]interface{}, error) {
	return a.svc.ApplyPublicConfigOverlay(ctx, tenant, base)
}
