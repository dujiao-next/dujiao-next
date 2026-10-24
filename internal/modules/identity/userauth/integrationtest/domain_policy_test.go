package integrationtest

import (
	"context"
	"errors"
	"fmt"
	userauthgormstore "github.com/dujiao-next/internal/modules/identity/userauth/infrastructure/gormstore"
	"testing"
	"time"

	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/shared/mailbrand"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type verificationEmailSenderStub struct{}

func (verificationEmailSenderStub) SendVerifyCode(_, _, _, _ string, _ mailbrand.Brand) error {
	return nil
}

type capturingVerificationEmailSender struct {
	brand mailbrand.Brand
}

func (s *capturingVerificationEmailSender) SendVerifyCode(_, _, _, _ string, brand mailbrand.Brand) error {
	s.brand = brand
	return nil
}

type verificationEmailBrandResolverStub struct {
	brand       mailbrand.Brand
	resellerID  uint
	resolverRun bool
}

func (s *verificationEmailBrandResolverStub) ResolveEmailBrand(ctx context.Context, _ mailbrand.Scope) (mailbrand.Brand, error) {
	s.resolverRun = true
	tenant, ok := resellercontract.TenantFromContext(ctx)
	if ok && tenant.ResellerID != nil {
		s.resellerID = *tenant.ResellerID
	}
	return s.brand, nil
}

func newRegistrationDomainPolicyAuthService(t *testing.T) (*userauthapp.Service, *settingsapp.Service, *gorm.DB) {
	return newRegistrationDomainPolicyAuthServiceWithSender(t, verificationEmailSenderStub{})
}

func newRegistrationDomainPolicyAuthServiceWithSender(t *testing.T, sender userauthapp.VerificationEmailSender) (*userauthapp.Service, *settingsapp.Service, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:user_auth_domain_policy_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}, &emailverificationdomain.Code{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}
	cfg := &config.Config{
		App:     config.AppConfig{SecretKey: "test-app-secret-domain-policy"},
		UserJWT: config.JWTConfig{SecretKey: "user-jwt-domain-policy-secret", ExpireHours: 24},
		Email:   config.EmailConfig{Enabled: false},
	}
	settingSvc := settingsapp.NewService(settingsstore.New(db))
	svc := userauthapp.NewService(
		cfg,
		userstore.New(db),
		externalidentitystore.New(db),
		emailverificationstore.New(db),
		settingSvc,
		sender,
		nil,
	)
	svc.SetAuthUnitOfWork(userauthgormstore.New(db))
	return svc, settingSvc, db
}

func TestRegisterRejectsEmailDomainNotAllowed(t *testing.T) {
	svc, settings, _ := newRegistrationDomainPolicyAuthService(t)
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldEmailDomainAllowlistEnabled: true,
		constants.SettingFieldAllowedEmailDomains:         []interface{}{"qq.com"},
	}); err != nil {
		t.Fatalf("update registration config failed: %v", err)
	}

	user, token, _, err := svc.Register("buyer@example.com", "secret123", "", "", true, false)
	if !errors.Is(err, settingsapp.ErrEmailDomainNotAllowed) {
		t.Fatalf("expected ErrEmailDomainNotAllowed, got user=%+v token=%q err=%v", user, token, err)
	}
}

func TestRegisterAllowsExactEmailDomain(t *testing.T) {
	svc, settings, _ := newRegistrationDomainPolicyAuthService(t)
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldEmailDomainAllowlistEnabled: true,
		constants.SettingFieldAllowedEmailDomains:         []interface{}{"qq.com"},
	}); err != nil {
		t.Fatalf("update registration config failed: %v", err)
	}

	user, token, _, err := svc.Register("buyer@qq.com", "secret123", "", "", true, false)
	if err != nil {
		t.Fatalf("register should allow qq.com: %v", err)
	}
	if user == nil || user.Email != "buyer@qq.com" || token == "" {
		t.Fatalf("unexpected register result user=%+v token=%q", user, token)
	}
}

func TestSendVerifyCodeRejectsEmailDomainBeforeEmailSend(t *testing.T) {
	svc, settings, _ := newRegistrationDomainPolicyAuthService(t)
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldEmailDomainAllowlistEnabled: true,
		constants.SettingFieldAllowedEmailDomains:         []interface{}{"qq.com"},
	}); err != nil {
		t.Fatalf("update registration config failed: %v", err)
	}

	err := svc.SendVerifyCode(context.Background(), "buyer@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, "")
	if !errors.Is(err, settingsapp.ErrEmailDomainNotAllowed) {
		t.Fatalf("expected ErrEmailDomainNotAllowed, got %v", err)
	}
}

func TestSendVerifyCodePropagatesRequestTenantBrandToSender(t *testing.T) {
	sender := &capturingVerificationEmailSender{}
	svc, _, _ := newRegistrationDomainPolicyAuthServiceWithSender(t, sender)
	resolver := &verificationEmailBrandResolverStub{brand: mailbrand.Brand{
		SiteName: "White Label Store",
		SiteURL:  "https://shop.example.test",
		FromName: "White Label Store",
	}}
	svc.SetEmailBrandResolver(resolver)
	tenant := resellercontract.ResellerTenantContext("shop.example.test", 31, 310, "shop.example.test")
	ctx := resellercontract.WithTenantContext(context.Background(), tenant)

	if err := svc.SendVerifyCode(ctx, "brand-buyer@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, ""); err != nil {
		t.Fatalf("send verify code failed: %v", err)
	}
	if !resolver.resolverRun || resolver.resellerID != 31 {
		t.Fatalf("request tenant did not reach brand resolver: %+v", resolver)
	}
	if sender.brand.SiteName != "White Label Store" || sender.brand.SiteURL != "https://shop.example.test" || sender.brand.FromName != "White Label Store" {
		t.Fatalf("resolved brand did not reach sender: %+v", sender.brand)
	}
}

type toggleVerificationEmailSender struct{ fail bool }

func (s *toggleVerificationEmailSender) SendVerifyCode(_, _, _, _ string, _ mailbrand.Brand) error {
	if s.fail {
		return errors.New("smtp timeout")
	}
	return nil
}

// 邮件没发出去时，验证码不能占用发送间隔，用户应能立即重发。
func TestSendVerifyCodeFailedSendDoesNotBlockResend(t *testing.T) {
	sender := &toggleVerificationEmailSender{fail: true}
	svc, _, _ := newRegistrationDomainPolicyAuthServiceWithSender(t, sender)

	if err := svc.SendVerifyCode(context.Background(), "resend@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, ""); err == nil {
		t.Fatal("expected send failure")
	}
	sender.fail = false
	if err := svc.SendVerifyCode(context.Background(), "resend@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, ""); err != nil {
		t.Fatalf("resend after failed send should succeed, got %v", err)
	}
}
