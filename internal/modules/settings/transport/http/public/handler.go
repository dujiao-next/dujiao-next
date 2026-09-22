package publicconfighttp

import (
	"context"
	"strings"
	"time"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/version"

	"github.com/gin-gonic/gin"
)

const publicConfigCacheTTL = 60 * time.Second

// ConfigCache 公开配置缓存端口。
type ConfigCache interface {
	CacheKey(resellerID *uint) string
	GetJSON(ctx context.Context, key string, dest *map[string]interface{}) (bool, error)
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// Settings 公开配置所需的设置端口。
type Settings interface {
	GetConfig(defaults map[string]interface{}) (map[string]interface{}, error)
	GetWalletRechargeChannelIDs() []uint
	GetWalletOnlyPayment() bool
	GetAffiliateSettingMap() (map[string]interface{}, error)
	GetSMTPEnabled() bool
	GetRegistrationEnabled(defaultValue bool) (bool, error)
	GetEmailVerificationEnabled(defaultValue bool) (bool, error)
	GetRegistrationEmailDomainPolicy() (enabled bool, allowedDomains []string, err error)
	GetByKey(key string) (interface{}, error)
	GetActiveHomeAnnouncement() (jsonmap.JSON, bool)
	GetActiveHomepageAd() (jsonmap.JSON, bool)
}

// PaymentChannels 公开支付渠道端口。
type PaymentChannels interface {
	GetOrderPaymentChannels() ([]map[string]interface{}, error)
}

// CaptchaPublic 公开验证码配置端口。
type CaptchaPublic interface {
	GetPublicSetting() (jsonmap.JSON, error)
}

// TelegramAuthPublic Telegram 登录公开配置端口。
type TelegramAuthPublic interface {
	PublicConfig() map[string]interface{}
}

// TelegramAuthFallback 无 TelegramAuthService 时的配置回退。
type TelegramAuthFallback struct {
	Enabled     bool
	BotUsername string
	MiniAppURL  string
}

// GoogleAuthPublic Google Identity Services 登录公开配置端口。
type GoogleAuthPublic interface {
	PublicConfig() map[string]interface{}
}

// GoogleAuthFallback 无 GoogleAuthService 时的配置回退。
type GoogleAuthFallback struct {
	Enabled  bool
	ClientID string
}

type GitHubAuthPublic interface {
	PublicConfig() map[string]interface{}
}

type GitHubAuthFallback struct {
	Enabled  bool
	ClientID string
}

// ResellerOverlay 分销站配置叠加端口。
type ResellerOverlay interface {
	ApplyPublicConfigOverlay(ctx context.Context, tenant reseller.TenantContext, base map[string]interface{}) (map[string]interface{}, error)
}

// Handler 处理公开站点配置 HTTP 请求。
type Handler struct {
	cache          ConfigCache
	settings       Settings
	payments       PaymentChannels
	captcha        CaptchaPublic
	telegram       TelegramAuthPublic
	fallback       TelegramAuthFallback
	google         GoogleAuthPublic
	googleFallback GoogleAuthFallback
	github         GitHubAuthPublic
	githubFallback GitHubAuthFallback
	overlay        ResellerOverlay
}

func NewHandler(
	cache ConfigCache,
	settings Settings,
	payments PaymentChannels,
	captcha CaptchaPublic,
	telegram TelegramAuthPublic,
	fallback TelegramAuthFallback,
	google GoogleAuthPublic,
	googleFallback GoogleAuthFallback,
	github GitHubAuthPublic,
	githubFallback GitHubAuthFallback,
	overlay ResellerOverlay,
) *Handler {
	if cache == nil {
		panic("public config handler: cache is nil")
	}
	if settings == nil {
		panic("public config handler: settings is nil")
	}
	if payments == nil {
		panic("public config handler: payments is nil")
	}
	return &Handler{
		cache:          cache,
		settings:       settings,
		payments:       payments,
		captcha:        captcha,
		telegram:       telegram,
		fallback:       fallback,
		google:         google,
		googleFallback: googleFallback,
		github:         github,
		githubFallback: githubFallback,
		overlay:        overlay,
	}
}

// GetConfig 获取全局配置。
func (h *Handler) GetConfig(c *gin.Context) {
	defaults := map[string]interface{}{
		"languages":                              append([]string(nil), constants.SupportedLocales...),
		constants.SettingFieldSiteCurrency:       constants.SiteCurrencyDefault,
		constants.SettingFieldStorefrontTemplate: constants.StorefrontTemplateDefault,
		"contact": map[string]interface{}{
			"telegram": "https://telegram.me/dujiaoka",
			"whatsapp": "https://wa.me/1234567890",
		},
		"scripts": make([]interface{}, 0),
	}

	tenant, _ := reseller.TenantFromContext(c.Request.Context())
	cacheKey := h.cache.CacheKey(tenant.ResellerID)

	var cached map[string]interface{}
	if hit, err := h.cache.GetJSON(c.Request.Context(), cacheKey, &cached); err == nil && hit {
		cached["server_time"] = time.Now().UnixMilli()
		cached["app_version"] = version.Version
		response.Success(c, cached)
		return
	}

	data, err := h.settings.GetConfig(defaults)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	publicChannels, err := h.payments.GetOrderPaymentChannels()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	data["payment_channels"] = publicChannels

	walletRechargeChannelIDs := h.settings.GetWalletRechargeChannelIDs()
	if len(walletRechargeChannelIDs) > 0 {
		data["wallet_recharge_channel_ids"] = walletRechargeChannelIDs
	}
	if h.settings.GetWalletOnlyPayment() {
		data["wallet_only_payment"] = true
	}

	if h.captcha != nil {
		publicCaptcha, captchaErr := h.captcha.GetPublicSetting()
		if captchaErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", captchaErr)
			return
		}
		data["captcha"] = publicCaptcha
	}
	telegramAuthConfig := map[string]interface{}{
		"enabled":      false,
		"bot_username": "",
		"mini_app_url": "",
	}
	if h.telegram != nil {
		telegramAuthConfig = h.telegram.PublicConfig()
	} else {
		telegramAuthConfig["enabled"] = h.fallback.Enabled
		telegramAuthConfig["bot_username"] = strings.TrimSpace(h.fallback.BotUsername)
		telegramAuthConfig["mini_app_url"] = strings.TrimSpace(h.fallback.MiniAppURL)
	}
	data["telegram_auth"] = telegramAuthConfig

	data["google_auth"] = resolveGoogleAuthPublicConfig(h.google, h.googleFallback)
	data["github_auth"] = resolveGitHubAuthPublicConfig(h.github, h.githubFallback)

	affiliateSetting, err := h.settings.GetAffiliateSettingMap()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}
	data["affiliate"] = affiliateSetting

	data["smtp_enabled"] = h.settings.GetSMTPEnabled()
	registrationEnabled, _ := h.settings.GetRegistrationEnabled(true)
	emailVerificationEnabled, _ := h.settings.GetEmailVerificationEnabled(true)
	data["registration_enabled"] = registrationEnabled
	data["email_verification_enabled"] = emailVerificationEnabled
	enabled, allowedDomains, policyErr := h.settings.GetRegistrationEmailDomainPolicy()
	if policyErr != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", policyErr)
		return
	}
	data["email_domain_allowlist_enabled"] = enabled
	data["allowed_email_domains"] = allowedDomains

	navConfigVal, _ := h.settings.GetByKey(constants.SettingKeyNavConfig)
	if navConfigVal != nil {
		data["nav_config"] = navConfigVal
	} else {
		data["nav_config"] = map[string]interface{}{
			"builtin":      map[string]interface{}{"blog": true, "notice": true, "about": true},
			"custom_items": make([]interface{}, 0),
		}
	}

	if announcement, ok := h.settings.GetActiveHomeAnnouncement(); ok {
		data["announcement"] = announcement
	}
	if ad, ok := h.settings.GetActiveHomepageAd(); ok {
		data["homepage_ad"] = ad
	}

	if h.overlay != nil {
		overlaid, overlayErr := h.overlay.ApplyPublicConfigOverlay(c.Request.Context(), tenant, data)
		if overlayErr != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", overlayErr)
			return
		}
		data = overlaid
	} else if tenant.ResellerID == nil {
		data["tenant"] = map[string]interface{}{"mode": "main", "host": tenant.Host}
	}

	_ = h.cache.SetJSON(c.Request.Context(), cacheKey, data, publicConfigCacheTTL)
	data["server_time"] = time.Now().UnixMilli()
	data["app_version"] = version.Version
	response.Success(c, data)
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func resolveGoogleAuthPublicConfig(source GoogleAuthPublic, fallback GoogleAuthFallback) map[string]interface{} {
	enabled := fallback.Enabled
	clientID := strings.TrimSpace(fallback.ClientID)
	if source != nil {
		publicGoogle := source.PublicConfig()
		clientID = strings.TrimSpace(stringValue(publicGoogle["client_id"]))
		enabled, _ = publicGoogle["enabled"].(bool)
	}
	return map[string]interface{}{
		"enabled":   enabled && clientID != "",
		"client_id": clientID,
	}
}

func resolveGitHubAuthPublicConfig(source GitHubAuthPublic, fallback GitHubAuthFallback) map[string]interface{} {
	enabled := fallback.Enabled
	clientID := strings.TrimSpace(fallback.ClientID)
	if source != nil {
		public := source.PublicConfig()
		enabled, _ = public["enabled"].(bool)
		clientID = strings.TrimSpace(stringValue(public["client_id"]))
	}
	return map[string]interface{}{"enabled": enabled && clientID != "", "client_id": clientID}
}
