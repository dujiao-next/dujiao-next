package userauthwiring

import (
	"context"
	"errors"
	"fmt"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"strings"
	"time"

	totpapplication "github.com/dujiao-next/internal/modules/identity/totp/application"
	"github.com/dujiao-next/internal/shared/passwordpolicy"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"github.com/dujiao-next/internal/cache"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/modules/identity/userauth/challenge"
	usertotpapp "github.com/dujiao-next/internal/modules/identity/userauth/totp/application"
	userauthtransport "github.com/dujiao-next/internal/modules/identity/userauth/transport/http"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
)

// userProfileTransportAdapter 将用户认证服务适配为用户资料 transport 端口。
type userProfileTransportAdapter struct {
	service *userauthapp.Service
}

func (a userProfileTransportAdapter) GetUserByID(id uint) (*userdomain.User, error) {
	user, err := a.service.GetUserByID(id)
	return user, mapUserAuthTransportError(err)
}

func (a userProfileTransportAdapter) ResolveEmailChangeMode(user *userdomain.User) (string, error) {
	mode, err := a.service.ResolveEmailChangeMode(user)
	return mode, mapUserAuthTransportError(err)
}

func (a userProfileTransportAdapter) ResolvePasswordChangeMode(user *userdomain.User) (string, error) {
	mode, err := a.service.ResolvePasswordChangeMode(user)
	return mode, mapUserAuthTransportError(err)
}

func (a userProfileTransportAdapter) UpdateProfile(userID uint, nickname, locale *string) (*userdomain.User, error) {
	user, err := a.service.UpdateProfile(userID, nickname, locale)
	return user, mapUserAuthTransportError(err)
}

// userEmailTransportAdapter 将用户认证服务适配为更换邮箱 transport 端口。
type userEmailTransportAdapter struct {
	service *userauthapp.Service
}

func (a userEmailTransportAdapter) SendChangeEmailCode(ctx context.Context, userID uint, kind, newEmail, locale string) error {
	return mapUserAuthTransportError(a.service.SendChangeEmailCode(ctx, userID, kind, newEmail, locale))
}

func (a userEmailTransportAdapter) ChangeEmail(userID uint, newEmail, oldCode, newCode string) (*userdomain.User, error) {
	user, err := a.service.ChangeEmail(userID, newEmail, oldCode, newCode)
	return user, mapUserAuthTransportError(err)
}

func (a userEmailTransportAdapter) ResolveEmailChangeMode(user *userdomain.User) (string, error) {
	mode, err := a.service.ResolveEmailChangeMode(user)
	return mode, mapUserAuthTransportError(err)
}

func (a userEmailTransportAdapter) ResolvePasswordChangeMode(user *userdomain.User) (string, error) {
	mode, err := a.service.ResolvePasswordChangeMode(user)
	return mode, mapUserAuthTransportError(err)
}

// userPasswordTransportAdapter 将用户认证/设置服务适配为密码 transport 端口。
type userPasswordTransportAdapter struct {
	auth     *userauthapp.Service
	settings *settingsapp.Service
}

func (a userPasswordTransportAdapter) GetEmailVerificationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetEmailVerificationEnabled(defaultValue)
}

func (a userPasswordTransportAdapter) ResetPassword(email, code, newPassword string) error {
	return mapUserAuthTransportError(a.auth.ResetPassword(email, code, newPassword))
}

func (a userPasswordTransportAdapter) ChangePassword(userID uint, oldPassword, newPassword string) error {
	return mapUserAuthTransportError(a.auth.ChangePassword(userID, oldPassword, newPassword))
}

// userVerifyTransportAdapter 将用户认证/设置服务适配为验证码发送端口。
type userVerifyTransportAdapter struct {
	auth     *userauthapp.Service
	settings *settingsapp.Service
}

func (a userVerifyTransportAdapter) GetEmailVerificationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetEmailVerificationEnabled(defaultValue)
}

func (a userVerifyTransportAdapter) GetRegistrationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetRegistrationEnabled(defaultValue)
}

func (a userVerifyTransportAdapter) SendVerifyCode(ctx context.Context, email, purpose, locale, inviteCode string) error {
	return mapUserAuthTransportError(a.auth.SendVerifyCode(ctx, email, purpose, locale, inviteCode))
}

// userTelegramTransportAdapter 将用户认证服务适配为 Telegram widget/MiniApp transport 端口。
type userTelegramTransportAdapter struct {
	auth *userauthapp.Service
}

// userGoogleTransportAdapter adapts Google authentication application services
// to the user HTTP transport port.
type userGoogleTransportAdapter struct {
	auth *userauthapp.Service
}

func (a userGoogleTransportAdapter) LoginWithGoogle(ctx context.Context, credential string) (*userauthtransport.AuthLoginResult, error) {
	result, err := a.auth.LoginWithGoogle(userauthapp.LoginWithGoogleInput{
		Credential: credential,
		Context:    ctx,
	})
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, result)
}

func (a userGoogleTransportAdapter) GetGoogleBinding(userID uint) (*userauthtransport.GoogleBindingResult, error) {
	result, err := a.auth.GetGoogleBinding(userID)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toGoogleTransportBindingResult(result), nil
}

func (a userGoogleTransportAdapter) BindGoogle(ctx context.Context, userID uint, credential string) (*userauthtransport.GoogleBindingResult, error) {
	result, err := a.auth.BindGoogle(userauthapp.BindGoogleInput{
		UserID:     userID,
		Credential: credential,
		Context:    ctx,
	})
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toGoogleTransportBindingResult(result), nil
}

func (a userGoogleTransportAdapter) UnbindGoogle(userID uint) error {
	return mapUserAuthTransportError(a.auth.UnbindGoogle(userID))
}

func (a userGoogleTransportAdapter) CreateGoogleRedirectIntent(
	ctx context.Context,
	flow string,
	userID uint,
	tenant userauthtransport.GoogleRedirectTenant,
) (string, error) {
	state, err := a.auth.CreateGoogleRedirectIntent(
		ctx,
		flow,
		userID,
		toGoogleRedirectApplicationTenant(tenant),
	)
	return state, mapUserAuthTransportError(err)
}

func (a userGoogleTransportAdapter) CompleteGoogleRedirect(
	ctx context.Context,
	state string,
	credential string,
	tenant userauthtransport.GoogleRedirectTenant,
) (*userauthtransport.GoogleRedirectCompletionResult, error) {
	result, err := a.auth.CompleteGoogleRedirect(
		ctx,
		state,
		credential,
		toGoogleRedirectApplicationTenant(tenant),
	)
	var transportResult *userauthtransport.GoogleRedirectCompletionResult
	if result == nil {
		return nil, mapUserAuthTransportError(err)
	}
	transportResult = &userauthtransport.GoogleRedirectCompletionResult{
		Flow:          result.Flow,
		HandoffHandle: result.HandoffHandle,
	}
	return transportResult, mapUserAuthTransportError(err)
}

func (a userGoogleTransportAdapter) ExchangeGoogleRedirectLogin(
	ctx context.Context,
	handle string,
	tenant userauthtransport.GoogleRedirectTenant,
) (*userauthtransport.AuthLoginResult, error) {
	result, err := a.auth.ExchangeGoogleRedirectLogin(
		ctx,
		handle,
		toGoogleRedirectApplicationTenant(tenant),
	)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, result)
}

func (a userGoogleTransportAdapter) ExchangeGoogleRedirectBind(
	ctx context.Context,
	handle string,
	userID uint,
	tenant userauthtransport.GoogleRedirectTenant,
) (*userauthtransport.GoogleBindingResult, error) {
	result, err := a.auth.ExchangeGoogleRedirectBind(
		ctx,
		handle,
		userID,
		toGoogleRedirectApplicationTenant(tenant),
	)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toGoogleTransportBindingResult(result), nil
}

func toGoogleRedirectApplicationTenant(
	tenant userauthtransport.GoogleRedirectTenant,
) userauthapp.GoogleRedirectTenant {
	return userauthapp.GoogleRedirectTenant{
		Host:          tenant.Host,
		IsMain:        tenant.IsMain,
		HasResellerID: tenant.HasResellerID,
		ResellerID:    tenant.ResellerID,
	}
}

func toGoogleTransportBindingResult(result *userauthapp.GoogleBinding) *userauthtransport.GoogleBindingResult {
	if result == nil {
		return nil
	}
	return &userauthtransport.GoogleBindingResult{
		Identity:    result.Identity,
		Email:       result.Email,
		DisplayName: result.DisplayName,
		CanUnbind:   result.CanUnbind,
	}
}

// toUserAuthTransportLoginResult 转换登录结果，并把访问令牌绑定到当前请求所在的分销站点。
func toUserAuthTransportLoginResult(ctx context.Context, auth *userauthapp.Service, result *userauthapp.UserLoginResult) (*userauthtransport.AuthLoginResult, error) {
	if result == nil {
		return nil, nil
	}
	token, err := auth.BindUserTokenTenant(result.Token, resellercontract.RequestResellerID(ctx))
	if err != nil {
		return nil, err
	}
	return &userauthtransport.AuthLoginResult{
		RequiresTOTP:       result.RequiresTOTP,
		User:               result.User,
		Token:              token,
		ExpiresAt:          result.ExpiresAt,
		ChallengeToken:     result.ChallengeToken,
		ChallengeExpiresAt: result.ChallengeExpiresAt,
		InviteRequired:     result.InviteRequired,
		InviteTicket:       result.InviteTicket,
	}, nil
}

// userInviteTransportAdapter 将第三方登录补填邀请码用例适配为 transport 端口。
type userInviteTransportAdapter struct {
	auth *userauthapp.Service
}

func (a userInviteTransportAdapter) CompleteInviteRegistration(ctx context.Context, ticket, inviteCode string) (*userauthtransport.AuthLoginResult, error) {
	res, err := a.auth.CompleteInviteRegistration(ctx, ticket, inviteCode)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, res)
}

func (a userTelegramTransportAdapter) toServicePayload(payload userauthtransport.TelegramAuthPayload) telegramauthapp.LoginPayload {
	return telegramauthapp.LoginPayload{
		ID:        payload.ID,
		FirstName: payload.FirstName,
		LastName:  payload.LastName,
		Username:  payload.Username,
		PhotoURL:  payload.PhotoURL,
		AuthDate:  payload.AuthDate,
		Hash:      payload.Hash,
	}
}

func (a userTelegramTransportAdapter) LoginWithTelegram(ctx context.Context, payload userauthtransport.TelegramAuthPayload) (*userauthtransport.AuthLoginResult, error) {
	res, err := a.auth.LoginWithTelegram(userauthapp.LoginWithTelegramInput{
		Payload: a.toServicePayload(payload),
		Context: ctx,
	})
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, res)
}

func (a userTelegramTransportAdapter) LoginWithTelegramMiniApp(ctx context.Context, initData string) (*userauthtransport.AuthLoginResult, error) {
	res, err := a.auth.LoginWithTelegramMiniApp(userauthapp.LoginWithTelegramMiniAppInput{
		InitData: initData,
		Context:  ctx,
	})
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, res)
}

func (a userTelegramTransportAdapter) GetTelegramBinding(userID uint) (*userauthtransport.TelegramBindingResult, error) {
	binding, err := a.auth.GetTelegramBinding(userID)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	if binding == nil {
		return nil, nil
	}
	return &userauthtransport.TelegramBindingResult{
		Identity:  binding.Identity,
		CanUnbind: binding.CanUnbind,
	}, nil
}

func (a userTelegramTransportAdapter) BindTelegram(ctx context.Context, userID uint, payload userauthtransport.TelegramAuthPayload) (*externalidentitydomain.Identity, error) {
	identity, err := a.auth.BindTelegram(userauthapp.BindTelegramInput{
		UserID:  userID,
		Payload: a.toServicePayload(payload),
		Context: ctx,
	})
	return identity, mapUserAuthTransportError(err)
}

func (a userTelegramTransportAdapter) BindTelegramMiniApp(ctx context.Context, userID uint, initData string) (*externalidentitydomain.Identity, error) {
	identity, err := a.auth.BindTelegramMiniApp(userauthapp.BindTelegramMiniAppInput{
		UserID:   userID,
		InitData: initData,
		Context:  ctx,
	})
	return identity, mapUserAuthTransportError(err)
}

func (a userTelegramTransportAdapter) UnbindTelegram(userID uint) error {
	return mapUserAuthTransportError(a.auth.UnbindTelegram(userID))
}

// userTelegramOIDCTransportAdapter 将用户认证服务适配为 Telegram OIDC transport 端口。
type userTelegramOIDCTransportAdapter struct {
	auth *userauthapp.Service
}

func (a userTelegramOIDCTransportAdapter) StartTelegramOIDC(ctx context.Context, intent string, userID uint) (string, error) {
	authURL, err := a.auth.StartTelegramOIDC(userauthapp.StartTelegramOIDCInput{
		Intent:  intent,
		UserID:  userID,
		Context: ctx,
	})
	return authURL, mapUserAuthTransportError(err)
}

func (a userTelegramOIDCTransportAdapter) LoginWithTelegramOIDC(ctx context.Context, code, state string) (*userauthtransport.AuthLoginResult, error) {
	res, err := a.auth.LoginWithTelegramOIDC(userauthapp.LoginWithTelegramOIDCInput{
		Code:    code,
		State:   state,
		Context: ctx,
	})
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, res)
}

func (a userTelegramOIDCTransportAdapter) BindTelegramOIDC(ctx context.Context, userID uint, code, state string) (*externalidentitydomain.Identity, error) {
	identity, err := a.auth.BindTelegramOIDC(userauthapp.BindTelegramOIDCInput{
		UserID:  userID,
		Code:    code,
		State:   state,
		Context: ctx,
	})
	return identity, mapUserAuthTransportError(err)
}

// userLoginTransportAdapter 将设置/认证服务适配为注册登录 transport 端口。
type userLoginTransportAdapter struct {
	auth     *userauthapp.Service
	settings *settingsapp.Service
}

func (a userLoginTransportAdapter) GetRegistrationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetRegistrationEnabled(defaultValue)
}

func (a userLoginTransportAdapter) GetEmailVerificationEnabled(defaultValue bool) (bool, error) {
	return a.settings.GetEmailVerificationEnabled(defaultValue)
}

func (a userLoginTransportAdapter) Register(ctx context.Context, email, password, code, inviteCode string, agreementAccepted, emailVerificationEnabled bool) (*userdomain.User, string, time.Time, error) {
	user, token, expiresAt, err := a.auth.Register(email, password, code, inviteCode, agreementAccepted, emailVerificationEnabled)
	if err != nil {
		return user, token, expiresAt, mapUserAuthTransportError(err)
	}
	token, err = a.auth.BindUserTokenTenant(token, resellercontract.RequestResellerID(ctx))
	return user, token, expiresAt, err
}

func (a userLoginTransportAdapter) LoginStep1(ctx context.Context, email, password string, rememberMe bool) (*userauthtransport.AuthLoginResult, error) {
	res, err := a.auth.LoginStep1(email, password, rememberMe)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, res)
}

// userLoginRecorderAdapter 将登录日志服务适配为 transport 记录端口。
type userLoginRecorderAdapter struct {
	logs *auditlogapp.UserLoginService
}

func (a userLoginRecorderAdapter) Record(email string, userID uint, status, failReason, source, clientIP, userAgent, requestID string) {
	if a.logs == nil {
		return
	}
	_ = a.logs.Record(auditlogapp.UserLoginRecord{
		UserID:      userID,
		Email:       email,
		Status:      status,
		FailReason:  failReason,
		ClientIP:    clientIP,
		UserAgent:   userAgent,
		LoginSource: source,
		RequestID:   strings.TrimSpace(requestID),
	})
}

// user2FATOTPTransportAdapter 将用户 TOTP 服务适配为 2FA transport 端口。
type user2FATOTPTransportAdapter struct {
	totp *usertotpapp.Service
}

func (a user2FATOTPTransportAdapter) GetStatus(userID uint) (*userauthtransport.UserTOTPStatus, error) {
	st, err := a.totp.GetStatus(userID)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	if st == nil {
		return nil, nil
	}
	return &userauthtransport.UserTOTPStatus{
		Enabled:                st.Enabled,
		EnabledAt:              st.EnabledAt,
		RecoveryCodesRemaining: st.RecoveryCodesRemaining,
		RecoveryCodesTotal:     st.RecoveryCodesTotal,
	}, nil
}

func (a user2FATOTPTransportAdapter) Setup(userID uint) (*userauthtransport.UserTOTPSetupResult, error) {
	res, err := a.totp.Setup(userID)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	if res == nil {
		return nil, nil
	}
	return &userauthtransport.UserTOTPSetupResult{
		Secret:     res.Secret,
		OtpauthURL: res.OtpauthURL,
		ExpiresAt:  res.ExpiresAt,
	}, nil
}

func (a user2FATOTPTransportAdapter) Enable(userID uint, code string) (*userauthtransport.UserTOTPEnableResult, error) {
	res, err := a.totp.Enable(userID, code)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	if res == nil {
		return nil, nil
	}
	return &userauthtransport.UserTOTPEnableResult{
		EnabledAt:     res.EnabledAt,
		RecoveryCodes: res.RecoveryCodes,
		Token:         res.Token,
		ExpiresAt:     res.ExpiresAt,
	}, nil
}

func (a user2FATOTPTransportAdapter) Disable(userID uint, code string, isRecoveryCode bool) error {
	return mapUserAuthTransportError(a.totp.Disable(userID, code, isRecoveryCode))
}

func (a user2FATOTPTransportAdapter) RegenerateRecoveryCodes(userID uint, code string) ([]string, error) {
	codes, err := a.totp.RegenerateRecoveryCodes(userID, code)
	return codes, mapUserAuthTransportError(err)
}

func (a user2FATOTPTransportAdapter) VerifyChallengeCode(userID uint, code string) error {
	return mapUserAuthTransportError(a.totp.VerifyChallengeCode(userID, code))
}

func (a user2FATOTPTransportAdapter) VerifyChallengeRecoveryCode(userID uint, code string) error {
	return mapUserAuthTransportError(a.totp.VerifyChallengeRecoveryCode(userID, code))
}

// user2FAAuthTransportAdapter 将用户认证/仓储适配为 2FA 登录完成端口。
type user2FAAuthTransportAdapter struct {
	auth  *userauthapp.Service
	users usercontract.Store
}

func (a user2FAAuthTransportAdapter) ParseUserChallengeToken(tokenString string) (*userauthtransport.UserChallengeClaims, error) {
	claims, err := a.auth.ParseUserChallengeToken(tokenString)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	if claims == nil {
		return nil, nil
	}
	return &userauthtransport.UserChallengeClaims{
		UserID:      claims.UserID,
		JTI:         claims.JTI,
		RememberMe:  claims.RememberMe,
		LoginSource: claims.LoginSource,
	}, nil
}

func (a user2FAAuthTransportAdapter) CompleteLoginAfter2FA(ctx context.Context, userID uint, rememberMe bool) (*userauthtransport.AuthLoginResult, error) {
	res, err := a.auth.CompleteLoginAfter2FA(userID, rememberMe)
	if err != nil {
		return nil, mapUserAuthTransportError(err)
	}
	return toUserAuthTransportLoginResult(ctx, a.auth, res)
}

func (a user2FAAuthTransportAdapter) GetUserEmail(userID uint) (string, error) {
	user, err := a.users.GetByID(userID)
	if err != nil {
		return "", mapUserAuthTransportError(err)
	}
	if user == nil {
		return "", nil
	}
	return user.Email, nil
}

// user2FAChallengeStoreAdapter 将 Redis 挑战状态适配为 transport 端口。
type user2FAChallengeStoreAdapter struct{}

func (user2FAChallengeStoreAdapter) IsRevoked(ctx context.Context, jti string) bool {
	rdb := cache.Client()
	if rdb == nil {
		return false
	}
	v, _ := rdb.Exists(ctx, challenge.RevocationKey(jti)).Result()
	return v == 1
}

func (user2FAChallengeStoreAdapter) BumpFails(ctx context.Context, jti string) int64 {
	rdb := cache.Client()
	if rdb == nil {
		return 0
	}
	cnt, err := rdb.Incr(ctx, challenge.FailureKey(jti)).Result()
	if err == nil && cnt == 1 {
		_ = rdb.Expire(ctx, challenge.FailureKey(jti), challenge.TTL).Err()
	}
	return cnt
}

func (user2FAChallengeStoreAdapter) Revoke(ctx context.Context, jti string) {
	rdb := cache.Client()
	if rdb == nil {
		return
	}
	_ = rdb.Set(ctx, challenge.RevocationKey(jti), "1", challenge.TTL).Err()
}

func mapUserAuthTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, passwordpolicy.ErrWeak) {
		type keyed interface {
			Key() string
			Args() []interface{}
		}
		var k keyed
		if errors.As(err, &k) {
			return userauthtransport.NewWeakPasswordError(k.Key(), k.Args()...)
		}
		if perr, ok := err.(keyed); ok {
			return userauthtransport.NewWeakPasswordError(perr.Key(), perr.Args()...)
		}
		return fmt.Errorf("%w: %v", userauthtransport.ErrWeakPassword, err)
	}
	for _, mapping := range []struct {
		source error
		target error
	}{
		{userauthapp.ErrProfileEmpty, userauthtransport.ErrProfileEmpty},
		{userauthapp.ErrNicknameInvalid, userauthtransport.ErrNicknameInvalid},
		{userauthapp.ErrNotFound, userauthtransport.ErrUserNotFound},
		{totpapplication.ErrSubjectNotFound, userauthtransport.ErrUserNotFound},
		{userauthapp.ErrInvalidEmail, userauthtransport.ErrInvalidEmail},
		{userauthapp.ErrEmailChangeInvalid, userauthtransport.ErrEmailChangeInvalid},
		{userauthapp.ErrEmailChangeExists, userauthtransport.ErrEmailChangeExists},
		{userauthapp.ErrVerifyCodeInvalid, userauthtransport.ErrVerifyCodeInvalid},
		{userauthapp.ErrVerifyCodeExpired, userauthtransport.ErrVerifyCodeExpired},
		{userauthapp.ErrVerifyCodeTooFrequent, userauthtransport.ErrVerifyCodeTooFrequent},
		{userauthapp.ErrVerifyCodeAttemptsExceeded, userauthtransport.ErrVerifyCodeAttemptsExceeded},
		{notificationcontract.ErrEmailServiceDisabled, userauthtransport.ErrEmailServiceDisabled},
		{userauthapp.ErrEmailServiceNotConfigured, userauthtransport.ErrEmailServiceNotConfigured},
		{notificationcontract.ErrEmailNotConfigured, userauthtransport.ErrEmailServiceNotConfigured},
		{notificationcontract.ErrEmailRecipientRejected, userauthtransport.ErrEmailRecipientRejected},
		{userauthapp.ErrInvalidPassword, userauthtransport.ErrInvalidPassword},
		{userauthapp.ErrInvalidVerifyPurpose, userauthtransport.ErrInvalidVerifyPurpose},
		{userauthapp.ErrEmailExists, userauthtransport.ErrEmailExists},
		{settingsapp.ErrEmailDomainNotAllowed, userauthtransport.ErrEmailDomainNotAllowed},
		{telegramauthapp.ErrTelegramAuthDisabled, userauthtransport.ErrTelegramAuthDisabled},
		{telegramauthapp.ErrTelegramAuthConfigInvalid, userauthtransport.ErrTelegramAuthConfigInvalid},
		{telegramauthapp.ErrTelegramOIDCStateInvalid, userauthtransport.ErrTelegramOIDCStateInvalid},
		{telegramauthapp.ErrTelegramOIDCTokenExchange, userauthtransport.ErrTelegramOIDCTokenExchange},
		{telegramauthapp.ErrTelegramOIDCIDTokenInvalid, userauthtransport.ErrTelegramOIDCIDTokenInvalid},
		{telegramauthapp.ErrTelegramAuthPayloadInvalid, userauthtransport.ErrTelegramAuthPayloadInvalid},
		{telegramauthapp.ErrTelegramAuthSignatureInvalid, userauthtransport.ErrTelegramAuthSignatureInvalid},
		{telegramauthapp.ErrTelegramAuthExpired, userauthtransport.ErrTelegramAuthExpired},
		{telegramauthapp.ErrTelegramAuthReplay, userauthtransport.ErrTelegramAuthReplay},
		{googleauthapp.ErrGoogleAuthDisabled, userauthtransport.ErrGoogleAuthDisabled},
		{googleauthapp.ErrGoogleAuthConfigInvalid, userauthtransport.ErrGoogleAuthConfigInvalid},
		{googleauthapp.ErrGoogleCredentialInvalid, userauthtransport.ErrGoogleCredentialInvalid},
		{googleauthapp.ErrGoogleCredentialExpired, userauthtransport.ErrGoogleCredentialExpired},
		{googleauthapp.ErrGoogleEmailUnverified, userauthtransport.ErrGoogleEmailUnverified},
		{googleauthapp.ErrGoogleJWKSUnavailable, userauthtransport.ErrGoogleJWKSUnavailable},
		{userauthapp.ErrUserOAuthIdentityExists, userauthtransport.ErrUserOAuthIdentityExists},
		{userauthapp.ErrUserOAuthAlreadyBound, userauthtransport.ErrUserOAuthAlreadyBound},
		{userauthapp.ErrUserOAuthNotBound, userauthtransport.ErrUserOAuthNotBound},
		{userauthapp.ErrTelegramUnbindRequiresEmail, userauthtransport.ErrTelegramUnbindRequiresEmail},
		{userauthapp.ErrGoogleAutoLinkForbidden, userauthtransport.ErrGoogleAutoLinkForbidden},
		{userauthapp.ErrGoogleUnbindLocked, userauthtransport.ErrGoogleUnbindLocked},
		{userauthapp.ErrGoogleRedirectUnavailable, userauthtransport.ErrGoogleRedirectUnavailable},
		{userauthapp.ErrGoogleRedirectSessionExpired, userauthtransport.ErrGoogleRedirectSessionExpired},
		{userauthapp.ErrGoogleRedirectTenantMismatch, userauthtransport.ErrGoogleRedirectTenantMismatch},
		{userauthapp.ErrGoogleRedirectUserMismatch, userauthtransport.ErrGoogleRedirectUserMismatch},
		{userauthapp.ErrGoogleRedirectFlowInvalid, userauthtransport.ErrGoogleRedirectFlowInvalid},
		{userauthapp.ErrUserDisabled, userauthtransport.ErrUserDisabled},
		{userauthapp.ErrRegistrationDisabled, userauthtransport.ErrRegistrationDisabled},
		{userauthapp.ErrInviteCodeRequired, userauthtransport.ErrInviteCodeRequired},
		{userauthapp.ErrInviteCodeInvalid, userauthtransport.ErrInviteCodeInvalid},
		{userauthapp.ErrInviteTicketInvalid, userauthtransport.ErrInviteTicketInvalid},
		{userauthapp.ErrAgreementRequired, userauthtransport.ErrAgreementRequired},
		{userauthapp.ErrInvalidCredentials, userauthtransport.ErrInvalidCredentials},
		{userauthapp.ErrEmailNotVerified, userauthtransport.ErrEmailNotVerified},
		{usertotpapp.ErrNotFound, userauthtransport.ErrUserNotFound},
		{totpapplication.ErrAlreadyEnabled, userauthtransport.ErrTOTPAlreadyEnabled},
		{totpapplication.ErrNotEnabled, userauthtransport.ErrTOTPNotEnabled},
		{totpapplication.ErrPendingExpired, userauthtransport.ErrTOTPPendingExpired},
		{totpapplication.ErrCodeInvalid, userauthtransport.ErrTOTPCodeInvalid},
		{totpapplication.ErrRecoveryCodeInvalid, userauthtransport.ErrTOTPRecoveryInvalid},
		{totpapplication.ErrTooManyAttempts, userauthtransport.ErrTOTPTooManyAttempts},
	} {
		if errors.Is(err, mapping.source) {
			return fmt.Errorf("%w: %v", mapping.target, err)
		}
	}
	return err
}
