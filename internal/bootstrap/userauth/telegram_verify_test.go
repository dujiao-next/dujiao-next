package userauthwiring

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	userauthtransport "github.com/dujiao-next/internal/modules/identity/userauth/transport/http"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingscontract "github.com/dujiao-next/internal/modules/settings/contract"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/mailbrand"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type telegramVerificationSender struct {
	sent int
}

func (s *telegramVerificationSender) SendVerifyCode(_, _, _, _ string, _ mailbrand.Brand) error {
	s.sent++
	return nil
}

type unavailableTelegramSettingStore struct {
	settingscontract.Store
}

func (s unavailableTelegramSettingStore) GetByKey(key string) (jsonmap.JSON, bool, error) {
	if key == constants.SettingKeyTelegramAuthConfig {
		return nil, false, errors.New("telegram settings unavailable")
	}
	return s.Store.GetByKey(key)
}

func TestPublicVerifyCodeRespectsTelegramSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name                string
		purpose             string
		runtimeEnabled      bool
		storedSetting       string
		telegramUnavailable bool
		registered          bool
		wantCode            int
		wantSent            int
	}{
		{name: "disabled by default", purpose: constants.VerifyPurposeTelegramBind, registered: true, wantCode: response.CodeForbidden},
		{name: "disabled with unknown email", purpose: constants.VerifyPurposeTelegramBind, wantCode: response.CodeForbidden},
		{name: "persisted disable overrides runtime", purpose: constants.VerifyPurposeTelegramBind, runtimeEnabled: true, storedSetting: "disabled", registered: true, wantCode: response.CodeForbidden},
		{name: "normalized purpose cannot bypass disable", purpose: "  TELEGRAM_BIND ", registered: true, wantCode: response.CodeForbidden},
		{name: "runtime enabled", purpose: constants.VerifyPurposeTelegramBind, runtimeEnabled: true, registered: true, wantCode: response.CodeOK, wantSent: 1},
		{name: "persisted enable overrides runtime", purpose: constants.VerifyPurposeTelegramBind, storedSetting: "enabled", registered: true, wantCode: response.CodeOK, wantSent: 1},
		{name: "unreadable telegram setting", purpose: constants.VerifyPurposeTelegramBind, runtimeEnabled: true, telegramUnavailable: true, registered: true, wantCode: response.CodeInternal},
		{name: "registration remains available", purpose: constants.VerifyPurposeRegister, wantCode: response.CodeOK, wantSent: 1},
		{name: "password reset remains available", purpose: constants.VerifyPurposeReset, registered: true, wantCode: response.CodeOK, wantSent: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, err := db.DB()
			if err != nil {
				t.Fatal(err)
			}
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = sqlDB.Close() })
			if err := db.AutoMigrate(&userdomain.User{}, &emailverificationdomain.Code{}, &settingsstore.SettingRecord{}); err != nil {
				t.Fatal(err)
			}
			const email = "owner@example.com"
			if tt.registered {
				if err := db.Create(&userdomain.User{
					Email: email, PasswordHash: "unused", Status: constants.UserStatusActive,
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			var settingsRepo settingscontract.Store = settingsstore.New(db)
			if tt.telegramUnavailable {
				settingsRepo = unavailableTelegramSettingStore{Store: settingsRepo}
			}
			settings := settingsapp.NewService(settingsRepo)
			if tt.storedSetting != "" {
				if _, err := settings.Update(constants.SettingKeyTelegramAuthConfig, map[string]interface{}{
					"enabled": tt.storedSetting == "enabled",
				}); err != nil {
					t.Fatal(err)
				}
			}
			cfg := &config.Config{TelegramAuth: config.TelegramAuthConfig{Enabled: tt.runtimeEnabled}}
			sender := &telegramVerificationSender{}
			auth := userauthapp.NewService(cfg, userstore.New(db), nil, emailverificationstore.New(db), settings, sender, nil)
			adapter := userVerifyTransportAdapter{auth: auth, settings: settings}
			handler := userauthtransport.NewUserVerifyHandler(adapter, nil, adapter)
			router := gin.New()
			userauthtransport.RegisterUserVerifyAuthRoutes(router.Group("/api/v1/auth"), handler, func(c *gin.Context) { c.Next() })

			body, err := json.Marshal(map[string]string{"email": email, "purpose": tt.purpose})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/send-verify-code", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)
			var result response.Response
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.StatusCode != tt.wantCode {
				t.Errorf("status_code = %d, want %d; body=%s", result.StatusCode, tt.wantCode, recorder.Body.String())
			}
			if sender.sent != tt.wantSent {
				t.Errorf("sent = %d, want %d", sender.sent, tt.wantSent)
			}
			var codes int64
			if err := db.Model(&emailverificationdomain.Code{}).Count(&codes).Error; err != nil {
				t.Fatal(err)
			}
			if codes != int64(tt.wantSent) {
				t.Errorf("persisted codes = %d, want %d", codes, tt.wantSent)
			}
		})
	}
}
