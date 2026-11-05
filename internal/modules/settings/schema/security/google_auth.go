package settingssecurity

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/config"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrGoogleAuthConfigInvalid = errors.New("google auth config invalid")

// GoogleAuthSetting Google Identity Services 登录配置实体。
type GoogleAuthSetting struct {
	Enabled  bool   `json:"enabled"`
	ClientID string `json:"client_id"`
}

// GoogleAuthSettingPatch Google Identity Services 登录配置补丁。
type GoogleAuthSettingPatch struct {
	Enabled  *bool   `json:"enabled"`
	ClientID *string `json:"client_id"`
}

// DefaultGoogleAuthSetting 根据运行时配置生成默认设置。
func DefaultGoogleAuthSetting(cfg config.GoogleAuthConfig) GoogleAuthSetting {
	return NormalizeGoogleAuthSetting(GoogleAuthSetting{
		Enabled:  cfg.Enabled,
		ClientID: cfg.ClientID,
	})
}

// NormalizeGoogleAuthSetting 归一化 Google 登录配置。
func NormalizeGoogleAuthSetting(setting GoogleAuthSetting) GoogleAuthSetting {
	setting.ClientID = strings.TrimSpace(setting.ClientID)
	return setting
}

// ValidateGoogleAuthSetting 校验 Google 登录配置合法性。
func ValidateGoogleAuthSetting(setting GoogleAuthSetting) error {
	normalized := NormalizeGoogleAuthSetting(setting)
	if normalized.Enabled && normalized.ClientID == "" {
		return fmt.Errorf("%w: Client ID 不能为空", ErrGoogleAuthConfigInvalid)
	}
	return nil
}

// ApplyGoogleAuthSettingPatch 把补丁应用到当前配置（不做校验）。
func ApplyGoogleAuthSettingPatch(current GoogleAuthSetting, patch GoogleAuthSettingPatch) GoogleAuthSetting {
	next := current
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.ClientID != nil {
		next.ClientID = strings.TrimSpace(*patch.ClientID)
	}
	return next
}

// GoogleAuthSettingToConfig 转换为运行时配置。
func GoogleAuthSettingToConfig(setting GoogleAuthSetting) config.GoogleAuthConfig {
	normalized := NormalizeGoogleAuthSetting(setting)
	return config.GoogleAuthConfig{
		Enabled:  normalized.Enabled,
		ClientID: normalized.ClientID,
	}
}

// EncodeGoogleAuthSetting 转换为 settings 存储结构。
func EncodeGoogleAuthSetting(setting GoogleAuthSetting) jsonmap.JSON {
	normalized := NormalizeGoogleAuthSetting(setting)
	return jsonmap.JSON{
		"enabled":   normalized.Enabled,
		"client_id": normalized.ClientID,
	}
}

// MaskGoogleAuthSettingForAdmin 返回后台可见配置。
// Google Identity Services 的 Client ID 是公开标识，不需要脱敏。
func MaskGoogleAuthSettingForAdmin(setting GoogleAuthSetting) jsonmap.JSON {
	return EncodeGoogleAuthSetting(setting)
}

// DecodeGoogleAuthSetting 从持久化 JSON 解码，并对缺失字段使用 fallback。
func DecodeGoogleAuthSetting(raw jsonmap.JSON, fallback GoogleAuthSetting) GoogleAuthSetting {
	next := fallback
	if raw == nil {
		return next
	}
	if value, exists := raw["enabled"]; exists {
		next.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["client_id"]; exists {
		if text, ok := value.(string); ok {
			next.ClientID = text
		}
	}
	return NormalizeGoogleAuthSetting(next)
}

// NormalizeGoogleAuthSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeGoogleAuthSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeGoogleAuthSetting(DecodeGoogleAuthSetting(raw, DefaultGoogleAuthSetting(config.GoogleAuthConfig{})))
}
