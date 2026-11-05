package application

import (
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	"github.com/dujiao-next/internal/telegramidentity"
)

// TelegramChannelIdentityInput Telegram 渠道身份输入
type TelegramChannelIdentityInput struct {
	ChannelUserID string
	Username      string
	FirstName     string
	LastName      string
	AvatarURL     string
	// InviteCode 邀请注册模式下 Bot 新用户建号所需的邀请码。
	InviteCode string
}

// BindTelegramChannelByEmailCodeInput Telegram 渠道邮箱验证码绑定输入
type BindTelegramChannelByEmailCodeInput struct {
	Identity TelegramChannelIdentityInput
	Email    string
	Code     string
}

// ResolveTelegramChannelIdentity 解析 Telegram 渠道身份
func (s *Service) ResolveTelegramChannelIdentity(input TelegramChannelIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, error) {
	verified, err := normalizeTelegramChannelIdentityInput(input)
	if err != nil {
		return nil, nil, err
	}
	return s.resolveTelegramChannelIdentity(verified)
}

// ProvisionTelegramChannelIdentity 预置 Telegram 渠道身份
func (s *Service) ProvisionTelegramChannelIdentity(input TelegramChannelIdentityInput) (*userdomain.User, *externalidentitydomain.Identity, bool, error) {
	verified, err := normalizeTelegramChannelIdentityInput(input)
	if err != nil {
		return nil, nil, false, err
	}
	return s.provisionTelegramChannelIdentity(verified, input.InviteCode)
}

// BindTelegramChannelByEmailCode 使用邮箱验证码绑定 Telegram 渠道身份到既有账号
func (s *Service) BindTelegramChannelByEmailCode(input BindTelegramChannelByEmailCodeInput) (*userdomain.User, *externalidentitydomain.Identity, uint, error) {
	verified, err := normalizeTelegramChannelIdentityInput(input.Identity)
	if err != nil {
		return nil, nil, 0, err
	}
	if s.userOAuthIdentityRepo == nil || s.userRepo == nil || s.codeRepo == nil {
		return nil, nil, 0, telegramauthapp.ErrTelegramAuthConfigInvalid
	}

	email, err := normalizeEmail(input.Email)
	if err != nil {
		return nil, nil, 0, err
	}
	if _, err := s.verifyCode(email, constants.VerifyPurposeTelegramBind, input.Code); err != nil {
		return nil, nil, 0, err
	}

	targetUser, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, nil, 0, err
	}
	if targetUser == nil {
		return nil, nil, 0, ErrNotFound
	}
	if strings.ToLower(strings.TrimSpace(targetUser.Status)) != constants.UserStatusActive {
		return nil, nil, 0, ErrUserDisabled
	}

	return s.bindTelegramIdentityToUser(targetUser, verified)
}

func (s *Service) resolveTelegramChannelIdentity(verified *telegramauthapp.IdentityVerified) (*userdomain.User, *externalidentitydomain.Identity, error) {
	if verified == nil {
		return nil, nil, telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	if s.userOAuthIdentityRepo == nil || s.userRepo == nil {
		return nil, nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}

	identity, err := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
	if err != nil {
		return nil, nil, err
	}
	if identity == nil {
		return nil, nil, nil
	}

	user, err := s.getActiveUserByID(identity.UserID)
	if err != nil {
		return nil, nil, err
	}
	if applyTelegramIdentity(verified, identity) {
		identity.UpdatedAt = time.Now()
		if err := s.userOAuthIdentityRepo.Update(identity); err != nil {
			return nil, nil, err
		}
	}
	return user, identity, nil
}

func (s *Service) provisionTelegramChannelIdentity(verified *telegramauthapp.IdentityVerified, inviteCode string) (*userdomain.User, *externalidentitydomain.Identity, bool, error) {
	if verified == nil {
		return nil, nil, false, telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	if s.userOAuthIdentityRepo == nil || s.userRepo == nil {
		return nil, nil, false, telegramauthapp.ErrTelegramAuthConfigInvalid
	}

	user, identity, err := s.resolveTelegramChannelIdentity(verified)
	if err != nil {
		return nil, nil, false, err
	}
	if identity != nil {
		return user, identity, false, nil
	}

	placeholderUser, err := s.userRepo.GetByEmail(telegramidentity.BuildPlaceholderEmail(verified.ProviderUserID))
	if err != nil {
		return nil, nil, false, err
	}
	created := placeholderUser == nil

	user, err = s.findOrCreateTelegramUser(verified, inviteCode)
	if err != nil {
		return nil, nil, false, err
	}

	identity, err = s.userOAuthIdentityRepo.GetByUserProvider(user.ID, verified.Provider)
	if err != nil {
		return nil, nil, false, err
	}
	if identity != nil {
		if identity.ProviderUserID != verified.ProviderUserID {
			return nil, nil, false, ErrUserOAuthAlreadyBound
		}
		if applyTelegramIdentity(verified, identity) {
			identity.UpdatedAt = time.Now()
			if err := s.userOAuthIdentityRepo.Update(identity); err != nil {
				return nil, nil, false, err
			}
		}
		return user, identity, created, nil
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
			return nil, nil, false, err
		}
		if existing == nil {
			return nil, nil, false, err
		}
		identity = existing
		user, err = s.getActiveUserByID(existing.UserID)
		if err != nil {
			return nil, nil, false, err
		}
		return user, identity, false, nil
	}

	return user, identity, created, nil
}

func (s *Service) bindTelegramIdentityToUser(targetUser *userdomain.User, verified *telegramauthapp.IdentityVerified) (*userdomain.User, *externalidentitydomain.Identity, uint, error) {
	if targetUser == nil || verified == nil {
		return nil, nil, 0, ErrNotFound
	}
	if s.userOAuthIdentityRepo == nil {
		return nil, nil, 0, telegramauthapp.ErrTelegramAuthConfigInvalid
	}

	current, err := s.userOAuthIdentityRepo.GetByUserProvider(targetUser.ID, verified.Provider)
	if err != nil {
		return nil, nil, 0, err
	}
	if current != nil && current.ProviderUserID != verified.ProviderUserID {
		return nil, nil, 0, ErrUserOAuthAlreadyBound
	}

	occupied, err := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
	if err != nil {
		return nil, nil, 0, err
	}
	if occupied != nil && occupied.UserID == targetUser.ID {
		if applyTelegramIdentity(verified, occupied) {
			occupied.UpdatedAt = time.Now()
			if err := s.userOAuthIdentityRepo.Update(occupied); err != nil {
				return nil, nil, 0, err
			}
		}
		return targetUser, occupied, 0, nil
	}

	if occupied != nil {
		previousUser, err := s.userRepo.GetByID(occupied.UserID)
		if err != nil {
			return nil, nil, 0, err
		}
		if previousUser == nil || !telegramidentity.IsPlaceholderEmail(previousUser.Email) {
			return nil, nil, 0, ErrUserOAuthIdentityExists
		}

		previousUserID := occupied.UserID
		occupied.UserID = targetUser.ID
		applyTelegramIdentity(verified, occupied)
		occupied.UpdatedAt = time.Now()
		if err := s.userOAuthIdentityRepo.Update(occupied); err != nil {
			return nil, nil, 0, err
		}
		return targetUser, occupied, previousUserID, nil
	}

	identity := &externalidentitydomain.Identity{
		UserID:         targetUser.ID,
		Provider:       verified.Provider,
		ProviderUserID: verified.ProviderUserID,
		Username:       verified.Username,
		AvatarURL:      verified.AvatarURL,
		AuthAt:         &verified.AuthAt,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.userOAuthIdentityRepo.Create(identity); err != nil {
		return nil, nil, 0, err
	}
	return targetUser, identity, 0, nil
}

func normalizeTelegramChannelIdentityInput(input TelegramChannelIdentityInput) (*telegramauthapp.IdentityVerified, error) {
	providerUserID := strings.TrimSpace(input.ChannelUserID)
	if providerUserID == "" {
		return nil, telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	return &telegramauthapp.IdentityVerified{
		Provider:       constants.UserOAuthProviderTelegram,
		ProviderUserID: providerUserID,
		Username:       strings.TrimSpace(input.Username),
		AvatarURL:      strings.TrimSpace(input.AvatarURL),
		FirstName:      strings.TrimSpace(input.FirstName),
		LastName:       strings.TrimSpace(input.LastName),
		AuthAt:         time.Now(),
	}, nil
}
