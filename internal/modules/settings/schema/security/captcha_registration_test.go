package settingssecurity

import (
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestCaptchaRegistrationSceneRoundTripsAcrossConfigAndPublicSettings(t *testing.T) {
	setting := DefaultCaptchaSetting(config.CaptchaConfig{
		Provider: constants.CaptchaProviderTurnstile,
		Scenes:   config.CaptchaSceneConfig{Register: true},
		Turnstile: config.CaptchaTurnstileConfig{
			SiteKey:   "site-key",
			SecretKey: "secret-key",
		},
	})
	if !setting.Scenes.Register || !setting.IsSceneEnabled(constants.CaptchaSceneRegister) {
		t.Fatalf("registration scene was not loaded from static config: %+v", setting.Scenes)
	}

	encoded := EncodeCaptchaSetting(setting)
	decoded := DecodeCaptchaSetting(encoded, CaptchaSetting{})
	if !decoded.Scenes.Register {
		t.Fatalf("registration scene was not preserved by encode/decode: %+v", decoded.Scenes)
	}

	publicScenes, ok := PublicCaptchaSetting(decoded)["scenes"].(map[string]interface{})
	if !ok || publicScenes["register"] != true {
		t.Fatalf("public registration scene = %#v", publicScenes)
	}

	runtimeConfig := CaptchaSettingToConfig(decoded)
	if !runtimeConfig.Scenes.Register {
		t.Fatal("registration scene was not preserved in runtime config")
	}
}

func TestApplyCaptchaSettingPatchUpdatesRegistrationScene(t *testing.T) {
	enabled := true
	updated, err := ApplyCaptchaSettingPatch(CaptchaSetting{
		Provider: constants.CaptchaProviderTurnstile,
		Turnstile: CaptchaTurnstileSetting{
			SiteKey:   "site-key",
			SecretKey: "secret-key",
		},
	}, CaptchaSettingPatch{
		Scenes: &CaptchaScenePatch{Register: &enabled},
	})
	if err != nil {
		t.Fatalf("patch registration scene: %v", err)
	}
	if !updated.Scenes.Register {
		t.Fatal("registration scene patch was not applied")
	}

	decoded := DecodeCaptchaSetting(jsonmap.JSON{
		"scenes": map[string]interface{}{"register": true},
	}, CaptchaSetting{})
	if !decoded.Scenes.Register {
		t.Fatal("registration scene was not decoded from persisted settings")
	}
}
