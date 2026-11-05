package application

import (
	"context"
	"errors"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
)

// LoginWithTelegramInput Telegram 登录输入
type LoginWithTelegramInput struct {
	Payload telegramauthapp.LoginPayload
	Context context.Context
}

// LoginWithTelegramMiniAppInput Telegram Mini App 登录输入
type LoginWithTelegramMiniAppInput struct {
	InitData string
	Context  context.Context
}

// LoginWithTelegram Telegram 登录（已启用 2FA 的账号会返回挑战 token，不直接发 JWT）
func (s *Service) LoginWithTelegram(input LoginWithTelegramInput) (*UserLoginResult, error) {
	if s.telegramAuthService == nil || s.userOAuthIdentityRepo == nil {
		return nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, err := s.telegramAuthService.VerifyLogin(ctx, input.Payload)
	if err != nil {
		return nil, err
	}
	return s.LoginVerifiedTelegram(verified)
}

// LoginWithTelegramMiniApp Telegram Mini App 登录（已启用 2FA 的账号会返回挑战 token，不直接发 JWT）
func (s *Service) LoginWithTelegramMiniApp(input LoginWithTelegramMiniAppInput) (*UserLoginResult, error) {
	if s.telegramAuthService == nil || s.userOAuthIdentityRepo == nil {
		return nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, err := s.telegramAuthService.VerifyMiniAppInitData(ctx, input.InitData)
	if err != nil {
		return nil, err
	}
	return s.LoginVerifiedTelegram(verified)
}

// LoginVerifiedTelegram completes a login after a trusted Telegram verifier
// has authenticated and normalized the upstream identity.
func (s *Service) LoginVerifiedTelegram(verified *telegramauthapp.IdentityVerified) (*UserLoginResult, error) {
	return s.loginVerifiedTelegram(verified, "")
}

func (s *Service) loginVerifiedTelegram(verified *telegramauthapp.IdentityVerified, inviteCode string) (*UserLoginResult, error) {
	identity, err := s.getTelegramIdentityByVerifiedID(verified)
	if err != nil {
		return nil, err
	}

	var user *userdomain.User
	if identity != nil {
		user, err = s.getActiveUserByID(identity.UserID)
		if err != nil {
			return nil, err
		}
		identityChanged, err := s.canonicalizeTelegramProviderUserID(verified, identity)
		if err != nil {
			return nil, err
		}
		identityChanged = applyTelegramIdentity(verified, identity) || identityChanged
		if identityChanged {
			identity.UpdatedAt = time.Now()
			if err := s.userOAuthIdentityRepo.Update(identity); err != nil {
				return nil, err
			}
		}
	} else {
		user, err = s.findOrCreateTelegramUser(verified, inviteCode)
		if errors.Is(err, ErrInviteCodeRequired) {
			return s.inviteRequiredResult(inviteTicketClaims{Provider: inviteTicketProviderTG, Telegram: verified})
		}
		if err != nil {
			return nil, err
		}
		identity = &externalidentitydomain.Identity{
			UserID:         user.ID,
			Provider:       verified.Provider,
			ProviderUserID: verified.ProviderUserID,
			Username:       verified.Username,
			AvatarURL:      verified.AvatarURL,
			AuthAt:         &verified.AuthAt,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.userOAuthIdentityRepo.Create(identity); err != nil {
			existing, getErr := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
			if getErr != nil {
				return nil, err
			}
			if existing == nil {
				return nil, err
			}
			identity = existing
			user, err = s.getActiveUserByID(existing.UserID)
			if err != nil {
				return nil, err
			}
		}
	}

	return s.completeExternalLogin(user, constants.LoginLogSourceTelegram)
}
