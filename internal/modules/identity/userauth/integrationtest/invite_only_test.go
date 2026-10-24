package integrationtest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	invitecodeapp "github.com/dujiao-next/internal/modules/identity/invitecode/application"
	invitecodedomain "github.com/dujiao-next/internal/modules/identity/invitecode/domain"
	invitecodestore "github.com/dujiao-next/internal/modules/identity/invitecode/infrastructure/gormstore"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"

	"gorm.io/gorm"
)

func setupInviteOnlyService(t *testing.T) (*userauthapp.Service, *gorm.DB) {
	t.Helper()
	svc, settings, db := setupTelegramOAuthTestService(t)
	if err := db.AutoMigrate(&invitecodedomain.InviteCode{}); err != nil {
		t.Fatalf("migrate invite codes: %v", err)
	}
	svc.SetInviteCodeChecker(invitecodeapp.NewService(invitecodestore.New(db)))
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled:      true,
		constants.SettingFieldEmailVerificationEnabled: false,
		constants.SettingFieldRegistrationInviteOnly:   true,
	}); err != nil {
		t.Fatalf("enable invite-only registration: %v", err)
	}
	return svc, db
}

func createInviteCode(t *testing.T, db *gorm.DB, code string, maxUses int) {
	t.Helper()
	if err := db.Create(&invitecodedomain.InviteCode{Code: code, MaxUses: maxUses, Status: invitecodedomain.StatusActive}).Error; err != nil {
		t.Fatalf("create invite code: %v", err)
	}
}

func countUsers(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&userdomain.User{}).Count(&count).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	return count
}

func tgIdentity(id string) *telegramauthapp.IdentityVerified {
	return &telegramauthapp.IdentityVerified{Provider: constants.UserOAuthProviderTelegram, ProviderUserID: id, Username: "tg_" + id, AuthAt: time.Now()}
}

// 邀请注册模式下，任何会新建用户的入口在没有邀请码时都不得落库。
func TestInviteOnlyEveryEntryRejectsNewUserWithoutCode(t *testing.T) {
	svc, db := setupInviteOnlyService(t)

	if _, _, _, err := svc.Register("new@example.com", "Secret123", "", "", true, false); !errors.Is(err, userauthapp.ErrInviteCodeRequired) {
		t.Fatalf("email register: want ErrInviteCodeRequired, got %v", err)
	}
	if _, _, _, err := svc.Register("new@example.com", "Secret123", "", "NOPE", true, false); !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("email register with bad code: want ErrInviteCodeInvalid, got %v", err)
	}
	// Google 一键登录与跳转登录、Telegram Widget/MiniApp/OIDC 最终都汇入这两个入口。
	googleRes, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("new@gmail.com", "google-invite-1", true))
	if err != nil || googleRes == nil || !googleRes.InviteRequired || googleRes.InviteTicket == "" || googleRes.Token != "" {
		t.Fatalf("google login: want invite ticket without token, got %+v err=%v", googleRes, err)
	}
	tgRes, err := svc.LoginVerifiedTelegram(tgIdentity("70001"))
	if err != nil || tgRes == nil || !tgRes.InviteRequired || tgRes.Token != "" {
		t.Fatalf("telegram login: want invite ticket without token, got %+v err=%v", tgRes, err)
	}
	// Bot 渠道（含下单、钱包、礼品卡、推广等隐式开通）统一走 ProvisionTelegramChannelIdentity。
	if _, _, _, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{ChannelUserID: "70002"}); !errors.Is(err, userauthapp.ErrInviteCodeRequired) {
		t.Fatalf("bot provision: want ErrInviteCodeRequired, got %v", err)
	}

	if n := countUsers(t, db); n != 0 {
		t.Fatalf("no user may be created without an invite code, got %d", n)
	}
	var identities int64
	db.Table("user_oauth_identities").Count(&identities)
	if identities != 0 {
		t.Fatalf("no identity may be created without an invite code, got %d", identities)
	}
}

func TestInviteOnlyTicketCompletesRegistrationOnce(t *testing.T) {
	svc, db := setupInviteOnlyService(t)
	createInviteCode(t, db, "ONCE2026", 1)

	res, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("ticket@gmail.com", "google-invite-2", true))
	if err != nil || !res.InviteRequired {
		t.Fatalf("expected invite ticket, got %+v err=%v", res, err)
	}
	ticket := res.InviteTicket

	if _, err := svc.CompleteInviteRegistration(context.Background(), ticket+"x", "ONCE2026"); !errors.Is(err, userauthapp.ErrInviteTicketInvalid) {
		t.Fatalf("tampered ticket: want ErrInviteTicketInvalid, got %v", err)
	}
	if _, err := svc.CompleteInviteRegistration(context.Background(), ticket, "WRONG"); !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("wrong code: want ErrInviteCodeInvalid, got %v", err)
	}
	// 码大小写不敏感；错误码后同一票据可重试。
	done, err := svc.CompleteInviteRegistration(context.Background(), ticket, "once2026")
	if err != nil || done == nil || done.User == nil || done.Token == "" {
		t.Fatalf("complete registration: %+v err=%v", done, err)
	}
	if done.User.InviteCodeID == nil {
		t.Fatalf("user must record its invite code")
	}
	var code invitecodedomain.InviteCode
	db.Where("code = ?", "ONCE2026").First(&code)
	if code.UsedCount != 1 || *done.User.InviteCodeID != code.ID {
		t.Fatalf("invite code not consumed correctly: %+v user=%+v", code, done.User)
	}

	// 账号建好后票据即失效，不能当作登录凭证重放。
	if _, err := svc.CompleteInviteRegistration(context.Background(), ticket, "ONCE2026"); !errors.Is(err, userauthapp.ErrInviteTicketInvalid) {
		t.Fatalf("replayed ticket: want ErrInviteTicketInvalid, got %v", err)
	}
	// 已有账号再登录无需邀请码。
	again, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("ticket@gmail.com", "google-invite-2", true))
	if err != nil || again.InviteRequired || again.Token == "" {
		t.Fatalf("existing user login must not need a code: %+v err=%v", again, err)
	}

	// 名额用完后其他新用户同码失败，且不落库。
	tgRes, _ := svc.LoginVerifiedTelegram(tgIdentity("70003"))
	if _, err := svc.CompleteInviteRegistration(context.Background(), tgRes.InviteTicket, "ONCE2026"); !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("exhausted code: want ErrInviteCodeInvalid, got %v", err)
	}
	if n := countUsers(t, db); n != 1 {
		t.Fatalf("exhausted code must not create users, got %d", n)
	}
}

// Telegram（Widget/MiniApp/OIDC）新用户凭票据补填邀请码后建号并绑定身份，之后登录无需再填码。
func TestInviteOnlyTelegramTicketCompletesRegistration(t *testing.T) {
	svc, db := setupInviteOnlyService(t)
	createInviteCode(t, db, "TGCODE", 1)

	res, err := svc.LoginVerifiedTelegram(tgIdentity("70006"))
	if err != nil || !res.InviteRequired || res.InviteTicket == "" {
		t.Fatalf("expected invite ticket, got %+v err=%v", res, err)
	}
	done, err := svc.CompleteInviteRegistration(context.Background(), res.InviteTicket, "TGCODE")
	if err != nil || done == nil || done.User == nil || done.Token == "" || done.User.InviteCodeID == nil {
		t.Fatalf("complete telegram registration: %+v err=%v", done, err)
	}
	var identities int64
	db.Table("user_oauth_identities").Where("provider = ? AND provider_user_id = ? AND user_id = ?", constants.UserOAuthProviderTelegram, "70006", done.User.ID).Count(&identities)
	if identities != 1 {
		t.Fatalf("telegram identity must be bound to the new user, got %d", identities)
	}
	again, err := svc.LoginVerifiedTelegram(tgIdentity("70006"))
	if err != nil || again.InviteRequired || again.Token == "" || again.User.ID != done.User.ID {
		t.Fatalf("existing telegram user login must not need a code: %+v err=%v", again, err)
	}
}

func TestInviteOnlyCodeLimitsAndOtherEntries(t *testing.T) {
	svc, db := setupInviteOnlyService(t)
	createInviteCode(t, db, "TEAM", 2)
	createInviteCode(t, db, "OFF", 0)
	db.Model(&invitecodedomain.InviteCode{}).Where("code = ?", "OFF").Update("status", invitecodedomain.StatusDisabled)

	if _, _, _, err := svc.Register("off@example.com", "Secret123", "", "OFF", true, false); !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("disabled code: want ErrInviteCodeInvalid, got %v", err)
	}
	if _, _, _, err := svc.Register("a@example.com", "Secret123", "", "TEAM", true, false); err != nil {
		t.Fatalf("email register with code: %v", err)
	}
	if _, _, created, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{ChannelUserID: "70004", InviteCode: "TEAM"}); err != nil || !created {
		t.Fatalf("bot provision with code: created=%v err=%v", created, err)
	}
	if _, _, _, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{ChannelUserID: "70005", InviteCode: "TEAM"}); !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("third use of a 2-use code: want ErrInviteCodeInvalid, got %v", err)
	}
	// 已开通的 Bot 用户后续隐式调用无需再带码。
	if _, _, created, err := svc.ProvisionTelegramChannelIdentity(userauthapp.TelegramChannelIdentityInput{ChannelUserID: "70004"}); err != nil || created {
		t.Fatalf("existing bot user: created=%v err=%v", created, err)
	}
	if n := countUsers(t, db); n != 2 {
		t.Fatalf("want 2 users, got %d", n)
	}
}

// 邀请模式下注册验证码接口先校验邀请码，防止被用来向任意邮箱发信。
func TestInviteOnlySendVerifyCodeRequiresUsableCode(t *testing.T) {
	svc, settings, db := newRegistrationDomainPolicyAuthService(t)
	if err := db.AutoMigrate(&invitecodedomain.InviteCode{}); err != nil {
		t.Fatalf("migrate invite codes: %v", err)
	}
	svc.SetInviteCodeChecker(invitecodeapp.NewService(invitecodestore.New(db)))
	if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
		constants.SettingFieldRegistrationEnabled:    true,
		constants.SettingFieldRegistrationInviteOnly: true,
	}); err != nil {
		t.Fatalf("enable invite-only registration: %v", err)
	}
	createInviteCode(t, db, "MAIL", 1)
	ctx := context.Background()
	if err := svc.SendVerifyCode(ctx, "a@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, ""); !errors.Is(err, userauthapp.ErrInviteCodeRequired) {
		t.Fatalf("no code: want ErrInviteCodeRequired, got %v", err)
	}
	if err := svc.SendVerifyCode(ctx, "a@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, "BAD"); !errors.Is(err, userauthapp.ErrInviteCodeInvalid) {
		t.Fatalf("bad code: want ErrInviteCodeInvalid, got %v", err)
	}
	if err := svc.SendVerifyCode(ctx, "a@example.com", constants.VerifyPurposeRegister, constants.LocaleZhCN, "mail"); err != nil {
		t.Fatalf("valid code: %v", err)
	}
	var code invitecodedomain.InviteCode
	db.Where("code = ?", "MAIL").First(&code)
	if code.UsedCount != 0 {
		t.Fatalf("sending a verify code must not consume the invite code")
	}
}
