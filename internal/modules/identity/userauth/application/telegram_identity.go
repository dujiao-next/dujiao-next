package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	"github.com/dujiao-next/internal/telegramidentity"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) getActiveUserByID(userID uint) (*userdomain.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if strings.ToLower(strings.TrimSpace(user.Status)) != constants.UserStatusActive {
		return nil, ErrUserDisabled
	}
	return user, nil
}

func (s *Service) findOrCreateTelegramUser(verified *telegramauthapp.IdentityVerified, inviteCode string) (*userdomain.User, error) {
	if verified == nil {
		return nil, telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	email := telegramidentity.BuildPlaceholderEmail(verified.ProviderUserID)
	user, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if user != nil {
		if strings.ToLower(strings.TrimSpace(user.Status)) != constants.UserStatusActive {
			return nil, ErrUserDisabled
		}
		return user, nil
	}
	policy, err := s.loadRegistrationPolicy(inviteCode)
	if err != nil {
		return nil, err
	}
	if !policy.Enabled {
		return nil, ErrRegistrationDisabled
	}
	if err := s.precheckInviteCode(policy); err != nil {
		return nil, err
	}

	randomSuffix, err := randomNumericCode(16)
	if err != nil {
		return nil, err
	}
	passwordSeed := fmt.Sprintf("tg_%s_%s", verified.ProviderUserID, randomSuffix)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(passwordSeed), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	user = &userdomain.User{
		Email:                 email,
		PasswordHash:          string(hashedPassword),
		PasswordSetupRequired: true,
		DisplayName:           userdomain.SanitizeDisplayName(telegramidentity.ResolveDisplayName(verified.ProviderUserID, verified.Username, verified.FirstName, verified.LastName)),
		Status:                constants.UserStatusActive,
		LastLoginAt:           &now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	if err := s.createUserInTransaction(context.Background(), user, policy); err != nil {
		return nil, err
	}
	// 分配默认会员等级
	if s.memberLevelSvc != nil {
		_ = s.memberLevelSvc.AssignDefaultLevel(user.ID)
		// 同步内存对象的等级，避免调用方后续 Update(Save) 用零值覆盖数据库
		if refreshed, err := s.userRepo.GetByID(user.ID); err == nil && refreshed != nil {
			user.MemberLevelID = refreshed.MemberLevelID
		}
	}
	return user, nil
}

// getTelegramIdentityByVerifiedID 按 Telegram 数字 ID 查询绑定，未命中时兼容历史 OIDC subject 绑定。
func (s *Service) getTelegramIdentityByVerifiedID(verified *telegramauthapp.IdentityVerified) (*externalidentitydomain.Identity, error) {
	if verified == nil || s.userOAuthIdentityRepo == nil {
		return nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	identity, err := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
	if err != nil || identity != nil {
		return identity, err
	}
	for _, alias := range verified.ProviderUserIDAliases {
		alias = strings.TrimSpace(alias)
		if alias == "" || alias == verified.ProviderUserID {
			continue
		}
		identity, err = s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, alias)
		if err != nil || identity != nil {
			return identity, err
		}
	}
	return nil, nil
}

// canonicalizeTelegramProviderUserID 将历史 OIDC subject 绑定迁移为 Telegram 数字用户 ID。
func (s *Service) canonicalizeTelegramProviderUserID(verified *telegramauthapp.IdentityVerified, identity *externalidentitydomain.Identity) (bool, error) {
	if verified == nil || identity == nil || identity.ProviderUserID == verified.ProviderUserID {
		return false, nil
	}
	occupied, err := s.userOAuthIdentityRepo.GetByProviderUserID(verified.Provider, verified.ProviderUserID)
	if err != nil {
		return false, err
	}
	if occupied != nil && occupied.ID != identity.ID {
		return false, ErrUserOAuthIdentityExists
	}
	identity.ProviderUserID = verified.ProviderUserID
	return true, nil
}

// telegramProviderUserIDMatchesVerified 判断绑定 ID 是否匹配当前 Telegram 身份或其历史别名。
func telegramProviderUserIDMatchesVerified(providerUserID string, verified *telegramauthapp.IdentityVerified) bool {
	providerUserID = strings.TrimSpace(providerUserID)
	if verified == nil || providerUserID == "" {
		return false
	}
	if providerUserID == verified.ProviderUserID {
		return true
	}
	for _, alias := range verified.ProviderUserIDAliases {
		if providerUserID == strings.TrimSpace(alias) {
			return true
		}
	}
	return false
}

func applyTelegramIdentity(verified *telegramauthapp.IdentityVerified, identity *externalidentitydomain.Identity) bool {
	if verified == nil || identity == nil {
		return false
	}
	changed := false
	if identity.Provider == "" {
		identity.Provider = verified.Provider
		changed = true
	}
	if identity.ProviderUserID == "" {
		identity.ProviderUserID = verified.ProviderUserID
		changed = true
	}
	if identity.Username != verified.Username {
		identity.Username = verified.Username
		changed = true
	}
	if identity.AvatarURL != verified.AvatarURL {
		identity.AvatarURL = verified.AvatarURL
		changed = true
	}
	if identity.AuthAt == nil || !identity.AuthAt.Equal(verified.AuthAt) {
		authAt := verified.AuthAt
		identity.AuthAt = &authAt
		changed = true
	}
	return changed
}
