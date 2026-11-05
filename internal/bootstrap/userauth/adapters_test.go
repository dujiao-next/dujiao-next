package userauthwiring

import (
	"context"
	"fmt"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/mailbrand"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type noopVerifySender struct{}

func (noopVerifySender) SendVerifyCode(_, _, _, _ string, _ mailbrand.Brand) error { return nil }

// 在分销站注册/登录拿到的凭证必须带上该站点标识，否则中间件会把分销站用户全部拒之门外。
func TestLoginAdapterBindsTokenToRequestReseller(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:userauth_wiring_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}, &emailverificationdomain.Code{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "test-app-secret-wiring"},
		UserJWT: config.JWTConfig{SecretKey: "user-jwt-wiring-secret", ExpireHours: 24},
	}
	settings := settingsapp.NewService(settingsstore.New(db))
	auth := userauthapp.NewService(cfg, userstore.New(db), externalidentitystore.New(db), emailverificationstore.New(db), settings, noopVerifySender{}, nil)
	auth.SetAuthUnitOfWork(userauthgormstore.New(db))
	adapter := userLoginTransportAdapter{auth: auth, settings: settings}
	ctx := resellercontract.WithTenantContext(context.Background(), resellercontract.ResellerTenantContext("shop.example.test", 7, 70, "shop.example.test"))

	assertRID := func(name, token string) {
		t.Helper()
		claims, err := auth.ParseUserJWT(token)
		if err != nil || claims.ResellerID != 7 {
			t.Fatalf("%s token must carry rid=7, got claims=%+v err=%v", name, claims, err)
		}
	}
	_, token, _, err := adapter.Register(ctx, "wiring@example.com", "Secret123", "", "", true, false)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}
	assertRID("register", token)

	result, err := adapter.LoginStep1(ctx, "wiring@example.com", "Secret123", false)
	if err != nil || result == nil {
		t.Fatalf("login failed: %v", err)
	}
	assertRID("login", result.Token)
}
