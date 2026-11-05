package settingsapp

import (
	"github.com/dujiao-next/internal/constants"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// defaultSettingRegistry 显式登记通用 Update API 支持的归一化策略。
// 每个定义只负责把所属领域的既有解析/默认值/序列化函数接入 Registry，
// 避免 SettingService 再通过全局 switch 了解所有配置类型。
var defaultSettingRegistry = MustNewRegistry(
	Definition{
		Key:       constants.SettingKeyDashboardConfig,
		Normalize: settingsstorefront.NormalizeDashboardSettingJSON,
	},
	Definition{
		Key: constants.SettingKeyOrderConfig,
		Normalize: func(value jsonmap.JSON) jsonmap.JSON {
			return OrderConfigToMap(orderConfigFromJSON(value, DefaultOrderConfig()))
		},
	},
	Definition{
		Key:       constants.SettingKeySiteConfig,
		Normalize: func(value jsonmap.JSON) jsonmap.JSON { return normalizeSiteSetting(value) },
		Validate:  validateSiteSetting,
		Effects:   []Effect{EffectInvalidatePublicConfigCache},
	},
	Definition{
		Key:       constants.SettingKeyTelegramAuthConfig,
		Normalize: settingssecurity.NormalizeTelegramAuthSettingJSON,
	},
	Definition{
		Key:       constants.SettingKeyGoogleAuthConfig,
		Normalize: settingssecurity.NormalizeGoogleAuthSettingJSON,
		Effects:   []Effect{EffectInvalidatePublicConfigCache},
	},
	Definition{
		Key: constants.SettingKeyNotificationCenterConfig,
		Normalize: func(value jsonmap.JSON) jsonmap.JSON {
			setting := settingsmessaging.DecodeNotificationCenterSetting(value, settingsmessaging.NotificationCenterDefaultSetting())
			return jsonmap.JSON(settingsmessaging.NotificationCenterSettingToMap(setting))
		},
	},
	Definition{
		Key:       constants.SettingKeyAffiliateConfig,
		Normalize: settingsintegration.NormalizeAffiliateSettingJSON,
	},
	Definition{
		Key:       constants.SettingKeyTelegramBotConfig,
		Normalize: settingsmessaging.NormalizeTelegramBotConfigJSON,
	},
	Definition{
		Key:       constants.SettingKeyNavConfig,
		Normalize: func(value jsonmap.JSON) jsonmap.JSON { return normalizeNavConfig(value) },
		Effects:   []Effect{EffectInvalidatePublicConfigCache},
	},
	Definition{
		Key:       constants.SettingKeyRegistrationConfig,
		Normalize: func(value jsonmap.JSON) jsonmap.JSON { return normalizeRegistrationSetting(value) },
		Effects:   []Effect{EffectInvalidatePublicConfigCache},
	},
	Definition{
		Key:       constants.SettingKeyOrderRiskControlConfig,
		Normalize: settingssecurity.NormalizeOrderRiskControlConfigJSON,
	},
	Definition{
		Key:       constants.SettingKeyUpstreamSyncConfig,
		Normalize: settingsintegration.NormalizeUpstreamSyncConfigJSON,
	},
	Definition{
		Key:       constants.SettingKeyCallbackRoutesConfig,
		Normalize: settingsintegration.NormalizeCallbackRoutesSettingJSON,
		Effects:   []Effect{EffectInvalidateCallbackRoutesCache},
	},
	Definition{
		Key:       constants.SettingKeyHomeAnnouncement,
		Normalize: settingsstorefront.NormalizeHomeAnnouncementJSON,
		Effects:   []Effect{EffectInvalidatePublicConfigCache},
	},
	Definition{
		Key:     constants.SettingKeyWalletConfig,
		Effects: []Effect{EffectInvalidatePublicConfigCache},
	},
	Definition{
		Key:       constants.SettingKeyPaymentConfig,
		Normalize: NormalizePaymentFeeConfig,
	},
)
