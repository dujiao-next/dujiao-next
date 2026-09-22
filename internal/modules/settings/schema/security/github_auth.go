package settingssecurity

import (
	"errors"
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/config"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrGitHubAuthConfigInvalid = errors.New("github auth config invalid")

type GitHubAuthSetting struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

type GitHubAuthSettingPatch struct {
	Enabled      *bool   `json:"enabled"`
	ClientID     *string `json:"client_id"`
	ClientSecret *string `json:"client_secret"`
}

func DefaultGitHubAuthSetting(cfg config.GitHubAuthConfig) GitHubAuthSetting {
	return NormalizeGitHubAuthSetting(GitHubAuthSetting{Enabled: cfg.Enabled, ClientID: cfg.ClientID, ClientSecret: cfg.ClientSecret})
}

func NormalizeGitHubAuthSetting(setting GitHubAuthSetting) GitHubAuthSetting {
	setting.ClientID = strings.TrimSpace(setting.ClientID)
	setting.ClientSecret = strings.TrimSpace(setting.ClientSecret)
	return setting
}

func ValidateGitHubAuthSetting(setting GitHubAuthSetting) error {
	setting = NormalizeGitHubAuthSetting(setting)
	if setting.Enabled && (setting.ClientID == "" || setting.ClientSecret == "") {
		return fmt.Errorf("%w: Client ID 和 Client Secret 不能为空", ErrGitHubAuthConfigInvalid)
	}
	return nil
}

func ApplyGitHubAuthSettingPatch(current GitHubAuthSetting, patch GitHubAuthSettingPatch) GitHubAuthSetting {
	next := current
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.ClientID != nil {
		next.ClientID = strings.TrimSpace(*patch.ClientID)
	}
	// Empty means "keep current" so reading a masked form and saving it cannot erase the secret.
	if patch.ClientSecret != nil && strings.TrimSpace(*patch.ClientSecret) != "" {
		next.ClientSecret = strings.TrimSpace(*patch.ClientSecret)
	}
	return next
}

func GitHubAuthSettingToConfig(setting GitHubAuthSetting) config.GitHubAuthConfig {
	setting = NormalizeGitHubAuthSetting(setting)
	return config.GitHubAuthConfig{Enabled: setting.Enabled, ClientID: setting.ClientID, ClientSecret: setting.ClientSecret}
}

func EncodeGitHubAuthSetting(setting GitHubAuthSetting) jsonmap.JSON {
	setting = NormalizeGitHubAuthSetting(setting)
	return jsonmap.JSON{"enabled": setting.Enabled, "client_id": setting.ClientID, "client_secret": setting.ClientSecret}
}

func MaskGitHubAuthSettingForAdmin(setting GitHubAuthSetting) jsonmap.JSON {
	setting = NormalizeGitHubAuthSetting(setting)
	return jsonmap.JSON{
		"enabled":                  setting.Enabled,
		"client_id":                setting.ClientID,
		"client_secret":            "",
		"client_secret_configured": setting.ClientSecret != "",
	}
}

func PublicGitHubAuthSetting(setting GitHubAuthSetting) map[string]interface{} {
	setting = NormalizeGitHubAuthSetting(setting)
	return map[string]interface{}{
		"enabled":   setting.Enabled && setting.ClientID != "" && setting.ClientSecret != "",
		"client_id": setting.ClientID,
	}
}

func DecodeGitHubAuthSetting(raw jsonmap.JSON, fallback GitHubAuthSetting) GitHubAuthSetting {
	next := fallback
	if raw == nil {
		return NormalizeGitHubAuthSetting(next)
	}
	if value, ok := raw["enabled"]; ok {
		next.Enabled = settingsvalue.ParseBool(value)
	}
	if value, ok := raw["client_id"].(string); ok {
		next.ClientID = value
	}
	if value, ok := raw["client_secret"].(string); ok {
		next.ClientSecret = value
	}
	return NormalizeGitHubAuthSetting(next)
}

func NormalizeGitHubAuthSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeGitHubAuthSetting(DecodeGitHubAuthSetting(raw, DefaultGitHubAuthSetting(config.GitHubAuthConfig{})))
}
