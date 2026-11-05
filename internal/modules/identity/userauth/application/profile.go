package application

import (
	"context"
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	"github.com/dujiao-next/internal/shared/passwordpolicy"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/telegramidentity"

	"golang.org/x/crypto/bcrypt"
)

// ResetPassword 重置密码
func (s *Service) ResetPassword(email, code, newPassword string) error {
	normalized, err := normalizeEmail(email)
	if err != nil {
		return err
	}
	if err := passwordpolicy.Validate(s.cfg.Security.PasswordPolicy.ValidationPolicy(), newPassword); err != nil {
		return err
	}
	user, err := s.userRepo.GetByEmail(normalized)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}

	if _, err := s.verifyCode(normalized, constants.VerifyPurposeReset, code); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hashedPassword)
	user.PasswordSetupRequired = false
	now := time.Now()
	user.UpdatedAt = now
	user.TokenVersion++
	user.TokenInvalidBefore = &now
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))
	return nil
}

// ChangePassword 登录态修改密码
func (s *Service) ChangePassword(userID uint, oldPassword, newPassword string) error {
	if userID == 0 {
		return ErrNotFound
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	mode, err := s.ResolvePasswordChangeMode(user)
	if err != nil {
		return err
	}
	if mode == PasswordChangeModeChangeWithOld {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
			return ErrInvalidPassword
		}
	}

	if err := passwordpolicy.Validate(s.cfg.Security.PasswordPolicy.ValidationPolicy(), newPassword); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	user.PasswordHash = string(hashedPassword)
	user.PasswordSetupRequired = false
	now := time.Now()
	user.UpdatedAt = now
	user.TokenVersion++
	user.TokenInvalidBefore = &now
	if err := s.userRepo.Update(user); err != nil {
		return err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))
	return nil
}

// UpdateProfile 更新用户资料
func (s *Service) UpdateProfile(userID uint, nickname, locale *string) (*userdomain.User, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}

	updated := false
	if nickname != nil {
		trimmed := strings.TrimSpace(*nickname)
		if trimmed != "" {
			if !userdomain.ValidDisplayName(trimmed) {
				return nil, ErrNicknameInvalid
			}
			user.DisplayName = trimmed
			updated = true
		}
	}

	if locale != nil {
		trimmed := strings.TrimSpace(*locale)
		if trimmed != "" {
			user.Locale = trimmed
			updated = true
		}
	}

	if !updated {
		return nil, ErrProfileEmpty
	}

	user.UpdatedAt = time.Now()
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

// SendChangeEmailCode 发送更换邮箱验证码
func (s *Service) SendChangeEmailCode(ctx context.Context, userID uint, kind, newEmail, locale string) error {
	if s.emailService == nil {
		return ErrEmailServiceNotConfigured
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}

	if strings.TrimSpace(user.Locale) != "" {
		locale = user.Locale
	}
	mode, err := s.ResolveEmailChangeMode(user)
	if err != nil {
		return err
	}

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "old":
		if mode == EmailChangeModeBindOnly {
			return ErrEmailChangeInvalid
		}
		return s.sendVerifyCode(ctx, user.Email, constants.VerifyPurposeChangeEmailOld, locale)
	case "new":
		normalized, err := normalizeUserSuppliedEmail(newEmail)
		if err != nil {
			return err
		}
		if strings.EqualFold(normalized, user.Email) {
			return ErrEmailChangeInvalid
		}
		exist, err := s.userRepo.GetByEmail(normalized)
		if err != nil {
			return err
		}
		if exist != nil {
			return ErrEmailChangeExists
		}
		return s.sendVerifyCode(ctx, normalized, constants.VerifyPurposeChangeEmailNew, locale)
	default:
		return ErrEmailChangeInvalid
	}
}

// ChangeEmail 更换邮箱（旧邮箱/新邮箱双验证）
func (s *Service) ChangeEmail(userID uint, newEmail, oldCode, newCode string) (*userdomain.User, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	mode, err := s.ResolveEmailChangeMode(user)
	if err != nil {
		return nil, err
	}

	normalized, err := normalizeUserSuppliedEmail(newEmail)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(normalized, user.Email) {
		return nil, ErrEmailChangeInvalid
	}
	exist, err := s.userRepo.GetByEmail(normalized)
	if err != nil {
		return nil, err
	}
	if exist != nil {
		return nil, ErrEmailChangeExists
	}

	if mode != EmailChangeModeBindOnly {
		if _, err := s.verifyCode(user.Email, constants.VerifyPurposeChangeEmailOld, oldCode); err != nil {
			return nil, err
		}
	}
	if _, err := s.verifyCode(normalized, constants.VerifyPurposeChangeEmailNew, newCode); err != nil {
		return nil, err
	}

	now := time.Now()
	user.Email = normalized
	user.EmailVerifiedAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID 获取用户信息
func (s *Service) GetUserByID(id uint) (*userdomain.User, error) {
	if id == 0 {
		return nil, ErrNotFound
	}
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.ensureTelegramVirtualEmailState(user); err != nil {
		return nil, err
	}
	return user, nil
}

// ResolveEmailChangeMode 返回当前用户邮箱修改模式
func (s *Service) ResolveEmailChangeMode(user *userdomain.User) (string, error) {
	if user == nil {
		return EmailChangeModeChangeWithOldAndNew, nil
	}
	if err := s.ensureTelegramVirtualEmailState(user); err != nil {
		return "", err
	}
	if telegramidentity.IsPlaceholderEmail(user.Email) {
		return EmailChangeModeBindOnly, nil
	}
	return EmailChangeModeChangeWithOldAndNew, nil
}

// ResolvePasswordChangeMode 返回当前用户密码修改模式
func (s *Service) ResolvePasswordChangeMode(user *userdomain.User) (string, error) {
	if user == nil {
		return PasswordChangeModeChangeWithOld, nil
	}
	if err := s.ensureTelegramVirtualEmailState(user); err != nil {
		return "", err
	}
	if user.PasswordSetupRequired {
		return PasswordChangeModeSetWithoutOld, nil
	}
	return PasswordChangeModeChangeWithOld, nil
}

func (s *Service) ensureTelegramVirtualEmailState(user *userdomain.User) error {
	if user == nil || !telegramidentity.IsPlaceholderEmail(user.Email) {
		return nil
	}
	updated := false
	if user.EmailVerifiedAt != nil {
		user.EmailVerifiedAt = nil
		updated = true
	}
	if !user.PasswordSetupRequired {
		user.PasswordSetupRequired = true
		updated = true
	}
	if !updated {
		return nil
	}
	user.UpdatedAt = time.Now()
	return s.userRepo.Update(user)
}
