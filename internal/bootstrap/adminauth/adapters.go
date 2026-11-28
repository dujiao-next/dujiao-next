package adminauthwiring

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	totpapplication "github.com/dujiao-next/internal/modules/identity/totp/application"
	"github.com/dujiao-next/internal/shared/passwordpolicy"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	usertotpapp "github.com/dujiao-next/internal/modules/identity/userauth/totp/application"

	"github.com/dujiao-next/internal/cache"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminauthapp "github.com/dujiao-next/internal/modules/identity/adminauth/application"
	adminchallenge "github.com/dujiao-next/internal/modules/identity/adminauth/challenge"
	admintotpapp "github.com/dujiao-next/internal/modules/identity/adminauth/totp/application"
	adminauthtransport "github.com/dujiao-next/internal/modules/identity/adminauth/transport/http"
)

type admin2FATOTPTransportAdapter struct {
	totp *admintotpapp.Service
}

func (a admin2FATOTPTransportAdapter) GetStatus(adminID uint) (*adminauthtransport.TOTPStatus, error) {
	st, err := a.totp.GetStatus(adminID)
	if err != nil {
		return nil, mapAdminAuthTransportError(err)
	}
	if st == nil {
		return nil, nil
	}
	return &adminauthtransport.TOTPStatus{
		Enabled:                st.Enabled,
		EnabledAt:              st.EnabledAt,
		RecoveryCodesRemaining: st.RecoveryCodesRemaining,
		RecoveryCodesTotal:     st.RecoveryCodesTotal,
	}, nil
}

func (a admin2FATOTPTransportAdapter) Setup(adminID uint) (*adminauthtransport.TOTPSetupResult, error) {
	res, err := a.totp.Setup(adminID)
	if err != nil {
		return nil, mapAdminAuthTransportError(err)
	}
	if res == nil {
		return nil, nil
	}
	return &adminauthtransport.TOTPSetupResult{
		Secret:     res.Secret,
		OtpauthURL: res.OtpauthURL,
		ExpiresAt:  res.ExpiresAt,
	}, nil
}

func (a admin2FATOTPTransportAdapter) Enable(adminID uint, code string) (*adminauthtransport.TOTPEnableResult, error) {
	res, err := a.totp.Enable(adminID, code)
	if err != nil {
		return nil, mapAdminAuthTransportError(err)
	}
	if res == nil {
		return nil, nil
	}
	return &adminauthtransport.TOTPEnableResult{
		EnabledAt:     res.EnabledAt,
		RecoveryCodes: res.RecoveryCodes,
	}, nil
}

func (a admin2FATOTPTransportAdapter) Disable(adminID uint, code string, isRecoveryCode bool) error {
	return mapAdminAuthTransportError(a.totp.Disable(adminID, code, isRecoveryCode))
}

func (a admin2FATOTPTransportAdapter) RegenerateRecoveryCodes(adminID uint, code string) ([]string, error) {
	codes, err := a.totp.RegenerateRecoveryCodes(adminID, code)
	return codes, mapAdminAuthTransportError(err)
}

func (a admin2FATOTPTransportAdapter) VerifyChallengeCode(adminID uint, code string) error {
	return mapAdminAuthTransportError(a.totp.VerifyChallengeCode(adminID, code))
}

func (a admin2FATOTPTransportAdapter) VerifyChallengeRecoveryCode(adminID uint, code string) error {
	return mapAdminAuthTransportError(a.totp.VerifyChallengeRecoveryCode(adminID, code))
}

func (a admin2FATOTPTransportAdapter) AdminReset(operatorID, targetID uint) error {
	return mapAdminAuthTransportError(a.totp.AdminReset(operatorID, targetID))
}

type adminLoginAuthTransportAdapter struct {
	auth *adminauthapp.Service
}

func (a adminLoginAuthTransportAdapter) Login(username, password string) (*adminauthtransport.AuthLoginResult, error) {
	res, err := a.auth.Login(username, password)
	if err != nil {
		return nil, mapAdminAuthTransportError(err)
	}
	if res == nil {
		return nil, nil
	}
	return &adminauthtransport.AuthLoginResult{
		RequiresTOTP:       res.RequiresTOTP,
		Admin:              res.Admin,
		Token:              res.Token,
		ExpiresAt:          res.ExpiresAt,
		ChallengeToken:     res.ChallengeToken,
		ChallengeExpiresAt: res.ChallengeExpiresAt,
	}, nil
}

func (a adminLoginAuthTransportAdapter) ChangePassword(adminID uint, oldPassword, newPassword string) error {
	return mapAdminAuthTransportError(a.auth.ChangePassword(adminID, oldPassword, newPassword))
}

func (a adminLoginAuthTransportAdapter) GetAdminByID(adminID uint) (*admindomain.Admin, error) {
	admin, err := a.auth.GetAdminByID(adminID)
	return admin, mapAdminAuthTransportError(err)
}

func (a adminLoginAuthTransportAdapter) RefreshJWT(admin *admindomain.Admin, sessionStartedAt int64) (string, time.Time, error) {
	token, expiresAt, err := a.auth.RefreshJWT(admin, sessionStartedAt)
	return token, expiresAt, mapAdminAuthTransportError(err)
}

type admin2FAAuthTransportAdapter struct {
	auth *adminauthapp.Service
}

func (a admin2FAAuthTransportAdapter) ParseChallengeToken(tokenString string) (*adminauthtransport.ChallengeClaims, error) {
	claims, err := a.auth.ParseChallengeToken(tokenString)
	if err != nil {
		return nil, mapAdminAuthTransportError(err)
	}
	if claims == nil {
		return nil, nil
	}
	return &adminauthtransport.ChallengeClaims{
		AdminID: claims.AdminID,
		JTI:     claims.JTI,
	}, nil
}

func (a admin2FAAuthTransportAdapter) CompleteLoginAfter2FA(adminID uint) (*adminauthtransport.AuthLoginResult, error) {
	res, err := a.auth.CompleteLoginAfter2FA(adminID)
	if err != nil {
		return nil, mapAdminAuthTransportError(err)
	}
	if res == nil {
		return nil, nil
	}
	return &adminauthtransport.AuthLoginResult{
		RequiresTOTP:       res.RequiresTOTP,
		Admin:              res.Admin,
		Token:              res.Token,
		ExpiresAt:          res.ExpiresAt,
		ChallengeToken:     res.ChallengeToken,
		ChallengeExpiresAt: res.ChallengeExpiresAt,
	}, nil
}

func (a admin2FAAuthTransportAdapter) GetAdminUsername(adminID uint) (string, error) {
	admin, err := a.auth.GetAdminByID(adminID)
	if err != nil {
		return "", mapAdminAuthTransportError(err)
	}
	if admin == nil {
		return "", nil
	}
	return admin.Username, nil
}

type admin2FAChallengeStoreAdapter struct{}

func (admin2FAChallengeStoreAdapter) IsRevoked(ctx context.Context, jti string) bool {
	rdb := cache.Client()
	if rdb == nil {
		return false
	}
	v, _ := rdb.Exists(ctx, adminchallenge.RevocationKey(jti)).Result()
	return v == 1
}

func (admin2FAChallengeStoreAdapter) BumpFails(ctx context.Context, jti string) int64 {
	rdb := cache.Client()
	if rdb == nil {
		return 0
	}
	cnt, err := rdb.Incr(ctx, adminchallenge.FailureKey(jti)).Result()
	if err == nil && cnt == 1 {
		_ = rdb.Expire(ctx, adminchallenge.FailureKey(jti), adminchallenge.TTL).Err()
	}
	return cnt
}

func (admin2FAChallengeStoreAdapter) Revoke(ctx context.Context, jti string) {
	rdb := cache.Client()
	if rdb == nil {
		return
	}
	_ = rdb.Set(ctx, adminchallenge.RevocationKey(jti), "1", adminchallenge.TTL).Err()
}

type adminLoginRecorderAdapter struct {
	logs *auditlogapp.AdminLoginService
}

func (a adminLoginRecorderAdapter) Record(adminID uint, username, eventType, status, failReason, clientIP, userAgent, requestID string, operatorID *uint) {
	if a.logs == nil {
		return
	}
	_ = a.logs.Record(auditlogapp.AdminLoginRecord{
		AdminID:    adminID,
		Username:   username,
		EventType:  eventType,
		Status:     status,
		FailReason: failReason,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		RequestID:  strings.TrimSpace(requestID),
		OperatorID: operatorID,
	})
}

type adminUser2FATransportAdapter struct {
	totp *usertotpapp.Service
}

func (a adminUser2FATransportAdapter) AdminResetUser2FA(operatorID, userID uint) (*userdomain.User, error) {
	user, err := a.totp.AdminResetUser2FA(operatorID, userID)
	return user, mapAdminAuthTransportError(err)
}

func mapAdminAuthTransportError(err error) error {
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
			return adminauthtransport.NewWeakPasswordError(k.Key(), k.Args()...)
		}
		if perr, ok := err.(keyed); ok {
			return adminauthtransport.NewWeakPasswordError(perr.Key(), perr.Args()...)
		}
		return fmt.Errorf("%w: %v", adminauthtransport.ErrWeakPassword, err)
	}
	for _, mapping := range []struct {
		source error
		target error
	}{
		{adminauthapp.ErrNotFound, adminauthtransport.ErrNotFound},
		{admintotpapp.ErrNotFound, adminauthtransport.ErrNotFound},
		{usertotpapp.ErrNotFound, adminauthtransport.ErrNotFound},
		{totpapplication.ErrSubjectNotFound, adminauthtransport.ErrNotFound},
		{adminauthapp.ErrInvalidCredentials, adminauthtransport.ErrInvalidCredentials},
		{adminauthapp.ErrInvalidPassword, adminauthtransport.ErrInvalidPassword},
		{totpapplication.ErrAlreadyEnabled, adminauthtransport.ErrTOTPAlreadyEnabled},
		{totpapplication.ErrNotEnabled, adminauthtransport.ErrTOTPNotEnabled},
		{totpapplication.ErrPendingExpired, adminauthtransport.ErrTOTPPendingExpired},
		{totpapplication.ErrCodeInvalid, adminauthtransport.ErrTOTPCodeInvalid},
		{totpapplication.ErrRecoveryCodeInvalid, adminauthtransport.ErrTOTPRecoveryInvalid},
		{totpapplication.ErrTooManyAttempts, adminauthtransport.ErrTOTPTooManyAttempts},
		{admintotpapp.ErrCannotResetSelf, adminauthtransport.ErrTOTPCannotResetSelf},
	} {
		if errors.Is(err, mapping.source) {
			return fmt.Errorf("%w: %v", mapping.target, err)
		}
	}
	return err
}
