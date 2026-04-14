package service

import (
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
)

type mockSettingRepo struct {
	store map[string]models.JSON
}

func newMockSettingRepo() *mockSettingRepo {
	return &mockSettingRepo{store: map[string]models.JSON{}}
}

func (m *mockSettingRepo) GetByKey(key string) (*models.Setting, error) {
	value, ok := m.store[key]
	if !ok {
		return nil, nil
	}
	return &models.Setting{Key: key, ValueJSON: value}, nil
}

func (m *mockSettingRepo) Upsert(key string, value models.JSON) (*models.Setting, error) {
	m.store[key] = value
	return &models.Setting{Key: key, ValueJSON: value}, nil
}

func (m *mockSettingRepo) DeleteByKey(key string) error {
	delete(m.store, key)
	return nil
}

func TestNormalizeSMTPSetting(t *testing.T) {
	setting := NormalizeSMTPSetting(SMTPSetting{})
	if setting.Port != 587 {
		t.Fatalf("expected default port 587, got %d", setting.Port)
	}
	if setting.VerifyCode.Length != 6 {
		t.Fatalf("expected default verify length 6, got %d", setting.VerifyCode.Length)
	}
	if setting.VerifyCode.ExpireMinutes != 10 {
		t.Fatalf("expected default expire minutes 10, got %d", setting.VerifyCode.ExpireMinutes)
	}
}

func TestValidateSMTPSetting(t *testing.T) {
	invalid := NormalizeSMTPSetting(SMTPSetting{
		Enabled: true,
		Host:    "smtp.example.com",
		From:    "notify@example.com",
		UseTLS:  true,
		UseSSL:  true,
	})
	if err := ValidateSMTPSetting(invalid); err == nil {
		t.Fatal("expected tls/ssl conflict validation error")
	}

	valid := NormalizeSMTPSetting(SMTPSetting{
		Enabled:  true,
		Host:     "smtp.example.com",
		Port:     587,
		From:     "notify@example.com",
		UseTLS:   true,
		UseSSL:   false,
		Password: "secret",
		VerifyCode: SMTPVerifyCodeSetting{
			ExpireMinutes:       10,
			SendIntervalSeconds: 60,
			MaxAttempts:         5,
			Length:              6,
		},
	})
	if err := ValidateSMTPSetting(valid); err != nil {
		t.Fatalf("expected valid smtp config, got error: %v", err)
	}
}

func TestPatchSMTPSettingKeepsPasswordWhenEmpty(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo)

	defaultCfg := config.EmailConfig{
		Enabled:  true,
		Host:     "smtp.default.com",
		Port:     587,
		Username: "default-user",
		Password: "default-secret",
		From:     "default@example.com",
		FromName: "Default",
		UseTLS:   true,
		UseSSL:   false,
		VerifyCode: config.VerifyCodeConfig{
			ExpireMinutes:       10,
			SendIntervalSeconds: 60,
			MaxAttempts:         5,
			Length:              6,
		},
	}

	updated, err := svc.PatchSMTPSetting(defaultCfg, SMTPSettingPatch{
		Host:     ptrString("smtp.custom.com"),
		Password: ptrString(""),
	})
	if err != nil {
		t.Fatalf("patch smtp setting failed: %v", err)
	}
	if updated.Password != "default-secret" {
		t.Fatalf("expected password keep default-secret, got %q", updated.Password)
	}

	saved, ok := repo.store[constants.SettingKeySMTPConfig]
	if !ok {
		t.Fatalf("smtp setting was not saved")
	}
	if saved["password"] != "default-secret" {
		t.Fatalf("expected saved password keep old value, got %v", saved["password"])
	}
}

func ptrString(value string) *string {
	return &value
}
