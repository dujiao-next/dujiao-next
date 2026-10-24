package settingsapp

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

func TestGoogleAuthSettingNormalizeValidateAndPublicConfig(t *testing.T) {
	setting := settingssecurity.NormalizeGoogleAuthSetting(settingssecurity.GoogleAuthSetting{
		Enabled:  true,
		ClientID: " client.apps.googleusercontent.com ",
	})
	if setting.ClientID != "client.apps.googleusercontent.com" {
		t.Fatalf("client id = %q", setting.ClientID)
	}
	if err := settingssecurity.ValidateGoogleAuthSetting(setting); err != nil {
		t.Fatalf("valid setting rejected: %v", err)
	}
	if err := settingssecurity.ValidateGoogleAuthSetting(settingssecurity.GoogleAuthSetting{Enabled: true}); !errors.Is(err, settingssecurity.ErrGoogleAuthConfigInvalid) {
		t.Fatalf("missing client id error = %v", err)
	}
}

func TestPatchGoogleAuthSettingPersistsNormalizedValue(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)
	enabled := true
	clientID := " new-client.apps.googleusercontent.com "

	setting, err := svc.PatchGoogleAuthSetting(
		config.GoogleAuthConfig{ClientID: "default.apps.googleusercontent.com"},
		settingssecurity.GoogleAuthSettingPatch{Enabled: &enabled, ClientID: &clientID},
	)
	if err != nil {
		t.Fatalf("patch google auth setting: %v", err)
	}
	if !setting.Enabled || setting.ClientID != "new-client.apps.googleusercontent.com" {
		t.Fatalf("unexpected setting: %#v", setting)
	}
	saved := repo.store[constants.SettingKeyGoogleAuthConfig]
	if saved["enabled"] != true || saved["client_id"] != "new-client.apps.googleusercontent.com" {
		t.Fatalf("unexpected saved value: %#v", saved)
	}
}

func TestPatchGoogleAuthSettingCanClearClientIDOnlyWhenDisabled(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)
	empty := ""
	if _, err := svc.PatchGoogleAuthSetting(
		config.GoogleAuthConfig{Enabled: true, ClientID: "client.apps.googleusercontent.com"},
		settingssecurity.GoogleAuthSettingPatch{ClientID: &empty},
	); !errors.Is(err, settingssecurity.ErrGoogleAuthConfigInvalid) {
		t.Fatalf("enabled clear error = %v", err)
	}

	disabled := false
	setting, err := svc.PatchGoogleAuthSetting(
		config.GoogleAuthConfig{Enabled: true, ClientID: "client.apps.googleusercontent.com"},
		settingssecurity.GoogleAuthSettingPatch{Enabled: &disabled, ClientID: &empty},
	)
	if err != nil {
		t.Fatalf("disable and clear: %v", err)
	}
	if setting.Enabled || setting.ClientID != "" {
		t.Fatalf("unexpected disabled setting: %#v", setting)
	}
}
