package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"golang.org/x/crypto/bcrypt"
)

const (
	maxGoogleProviderUserIDBytes = 255
	maxGoogleIdentityEmailBytes  = 128
	maxGooglePictureURLBytes     = 2048
)

// LoginWithGoogleInput is the credential submitted by Google Identity Services.
type LoginWithGoogleInput struct {
	Credential string
	Context    context.Context
}

// BindGoogleInput binds a verified Google credential to an authenticated user.
type BindGoogleInput struct {
	UserID     uint
	Credential string
	Context    context.Context
}

// GoogleBinding is the account-safe Google binding view.
type GoogleBinding struct {
	Identity    *externalidentitydomain.Identity
	Email       string
	DisplayName string
	CanUnbind   bool
}

// LoginWithGoogle verifies a Google ID token and completes the normal JWT/2FA
// login flow.
func (s *Service) LoginWithGoogle(input LoginWithGoogleInput) (*UserLoginResult, error) {
	if s == nil || s.googleAuthService == nil || s.userOAuthIdentityRepo == nil || s.authUnitOfWork == nil {
		return nil, googleauthapp.ErrGoogleAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, err := s.googleAuthService.VerifyCredential(ctx, input.Credential)
	if err != nil {
		return nil, err
	}
	return s.loginVerifiedGoogle(ctx, verified, "")
}

// LoginVerifiedGoogle completes login after the dedicated Google verifier has
// authenticated and normalized the credential.
func (s *Service) LoginVerifiedGoogle(verified *googleauthapp.VerifiedIdentity) (*UserLoginResult, error) {
	return s.loginVerifiedGoogle(context.Background(), verified, "")
}

func (s *Service) loginVerifiedGoogle(ctx context.Context, verified *googleauthapp.VerifiedIdentity, inviteCode string) (*UserLoginResult, error) {
	if s == nil || s.userOAuthIdentityRepo == nil || s.authUnitOfWork == nil {
		return nil, googleauthapp.ErrGoogleAuthConfigInvalid
	}
	verified, err := normalizeVerifiedGoogleIdentity(verified)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}

	for attempt := 0; attempt < 2; attempt++ {
		// Resolve the identity without a row lock first so the transaction can
		// consistently acquire locks in user -> identity order. If the mapping
		// changes before the locked recheck, the transaction rolls back and this
		// lookup is retried once.
		identityBefore, lookupErr := s.userOAuthIdentityRepo.GetByProviderUserID(
			constants.UserOAuthProviderGoogle,
			verified.Sub,
		)
		if lookupErr != nil {
			return nil, lookupErr
		}

		var userBefore *userdomain.User
		var registration registrationPolicy
		registrationLoaded := false
		if identityBefore == nil {
			userBefore, lookupErr = s.userRepo.GetByEmail(verified.Email)
			if lookupErr != nil {
				return nil, lookupErr
			}
			if userBefore == nil {
				registration, lookupErr = s.loadRegistrationPolicy(inviteCode)
				if lookupErr != nil {
					return nil, lookupErr
				}
				registrationLoaded = true
			}
		}

		user, createdUser, txErr := s.loginGoogleTransaction(
			ctx,
			verified,
			identityBefore,
			userBefore,
			registration,
			registrationLoaded,
		)
		if txErr != nil {
			if errors.Is(txErr, ErrInviteCodeRequired) {
				return s.inviteRequiredResult(inviteTicketClaims{Provider: inviteTicketProviderGoogle, Google: verified})
			}
			if errors.Is(txErr, errGoogleLoginMappingChanged) && attempt == 0 {
				continue
			}
			// A provider/user uniqueness race can commit after the pre-read.
			// Retry only when this exact verified provider identity now exists;
			// arbitrary persistence failures (including injected identity
			// failures) are returned and the transaction has rolled back.
			occupied, occupiedErr := s.userOAuthIdentityRepo.GetByProviderUserID(
				constants.UserOAuthProviderGoogle,
				verified.Sub,
			)
			if attempt == 0 && occupiedErr == nil && occupied != nil {
				continue
			}
			// PostgreSQL reports a users.email 23505 after the winning
			// transaction commits. For two different Google subjects claiming
			// the same authoritative email, translate that race to the stable
			// already-bound contract instead of leaking a raw database error.
			emailUser, emailErr := s.userRepo.GetByEmail(verified.Email)
			if emailErr == nil && emailUser != nil {
				current, identityErr := s.userOAuthIdentityRepo.GetByUserProvider(
					emailUser.ID,
					constants.UserOAuthProviderGoogle,
				)
				if identityErr == nil && current != nil {
					if current.ProviderUserID != verified.Sub {
						return nil, ErrUserOAuthAlreadyBound
					}
					if attempt == 0 {
						continue
					}
				}
				// A concurrent local registration can win the email insert
				// without a Google identity. Retry once to apply the normal
				// authoritative-link policy against the committed user.
				if current == nil && attempt == 0 {
					continue
				}
			}
			return nil, txErr
		}

		if createdUser && s.memberLevelSvc != nil {
			_ = s.memberLevelSvc.AssignDefaultLevel(user.ID)
			if refreshed, refreshErr := s.userRepo.GetByID(user.ID); refreshErr == nil && refreshed != nil {
				user = refreshed
			}
		}
		return s.completeExternalLogin(user, constants.LoginLogSourceGoogle)
	}
	return nil, errGoogleLoginMappingChanged
}

func (s *Service) loginGoogleTransaction(
	ctx context.Context,
	verified *googleauthapp.VerifiedIdentity,
	identityBefore *externalidentitydomain.Identity,
	userBefore *userdomain.User,
	registration registrationPolicy,
	registrationLoaded bool,
) (resolvedUser *userdomain.User, createdUser bool, resultErr error) {
	resultErr = s.authUnitOfWork.WithinTransaction(ctx, func(tx AuthTransaction) error {
		if identityBefore != nil {
			user, err := activeTransactionUser(tx, identityBefore.UserID)
			if err != nil {
				return err
			}
			identity, err := tx.GetIdentityByProviderUserID(constants.UserOAuthProviderGoogle, verified.Sub)
			if err != nil {
				return err
			}
			if identity == nil || identity.UserID != user.ID {
				return errGoogleLoginMappingChanged
			}
			resolvedUser = user
			if applyGoogleIdentity(verified, identity) {
				identity.UpdatedAt = time.Now()
				return tx.UpdateIdentity(identity)
			}
			return nil
		}

		// Lock the existing email account first. Looking up the provider
		// identity only after this point keeps the same lock order as bind and
		// unbind. If an identity appeared after the pre-read, retry from a new
		// user-first transaction based on its mapped user ID.
		user, err := tx.GetUserByEmail(verified.Email)
		if err != nil {
			return err
		}
		identity, err := tx.GetIdentityByProviderUserID(constants.UserOAuthProviderGoogle, verified.Sub)
		if err != nil {
			return err
		}
		if identity != nil {
			return errGoogleLoginMappingChanged
		}
		if userBefore != nil && user == nil {
			return errGoogleLoginMappingChanged
		}

		if user != nil {
			if !verified.EmailAuthoritative {
				return ErrGoogleAutoLinkForbidden
			}
			if strings.ToLower(strings.TrimSpace(user.Status)) != constants.UserStatusActive {
				return ErrUserDisabled
			}
			if user.EmailVerifiedAt == nil {
				now := time.Now()
				user.EmailVerifiedAt = &now
				user.UpdatedAt = now
				if err = tx.UpdateUser(user); err != nil {
					return err
				}
			}
		} else {
			// Google documents gmail.com and Workspace (hd) identities as
			// authoritative for current email ownership. A custom-domain email
			// without hd must first establish a local verified account and bind
			// explicitly; it is never sufficient for automatic account creation.
			if !verified.EmailAuthoritative {
				return ErrGoogleAutoLinkForbidden
			}
			if !registrationLoaded {
				return googleauthapp.ErrGoogleAuthConfigInvalid
			}
			if !registration.Enabled {
				return ErrRegistrationDisabled
			}
			if err = settingsapp.CheckRegistrationEmailDomainAllowed(verified.Email, registration.EmailDomain); err != nil {
				return err
			}
			user, err = newGoogleUser(verified)
			if err != nil {
				return err
			}
			if err = createRegisteredUser(tx, user, registration); err != nil {
				return err
			}
			createdUser = true
		}

		current, err := tx.GetIdentityByUserProvider(user.ID, constants.UserOAuthProviderGoogle)
		if err != nil {
			return err
		}
		if current != nil && current.ProviderUserID != verified.Sub {
			return ErrUserOAuthAlreadyBound
		}
		if current == nil {
			now := time.Now()
			current = &externalidentitydomain.Identity{
				UserID:         user.ID,
				Provider:       constants.UserOAuthProviderGoogle,
				ProviderUserID: verified.Sub,
				Username:       verified.Email,
				AvatarURL:      verified.Picture,
				AuthAt:         googleAuthTime(verified, now),
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			if err = tx.CreateIdentity(current); err != nil {
				return err
			}
		} else if applyGoogleIdentity(verified, current) {
			current.UpdatedAt = time.Now()
			if err = tx.UpdateIdentity(current); err != nil {
				return err
			}
		}
		resolvedUser = user
		return nil
	})
	return resolvedUser, createdUser, resultErr
}

func activeTransactionUser(tx AuthTransaction, userID uint) (*userdomain.User, error) {
	user, err := tx.GetUserByIDForUpdate(userID)
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

func newGoogleUser(verified *googleauthapp.VerifiedIdentity) (*userdomain.User, error) {
	passwordHash, err := generateGooglePlaceholderPassword()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	displayName := userdomain.SanitizeDisplayName(verified.Name)
	if displayName == "" {
		displayName = resolveNicknameFromEmail(verified.Email)
	}
	return &userdomain.User{
		Email:                 verified.Email,
		PasswordHash:          passwordHash,
		PasswordSetupRequired: true,
		DisplayName:           displayName,
		Status:                constants.UserStatusActive,
		EmailVerifiedAt:       &now,
		CreatedAt:             now,
		UpdatedAt:             now,
	}, nil
}

// BindGoogle verifies and binds a Google credential to the authenticated user.
func (s *Service) BindGoogle(input BindGoogleInput) (*GoogleBinding, error) {
	if input.UserID == 0 {
		return nil, ErrNotFound
	}
	if s == nil || s.googleAuthService == nil || s.userOAuthIdentityRepo == nil || s.authUnitOfWork == nil {
		return nil, googleauthapp.ErrGoogleAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, err := s.googleAuthService.VerifyCredential(ctx, input.Credential)
	if err != nil {
		return nil, err
	}
	return s.bindVerifiedGoogle(ctx, input.UserID, verified)
}

// BindVerifiedGoogle applies provider/user conflict rules after credential verification.
func (s *Service) BindVerifiedGoogle(userID uint, verified *googleauthapp.VerifiedIdentity) (*GoogleBinding, error) {
	return s.bindVerifiedGoogle(context.Background(), userID, verified)
}

func (s *Service) bindVerifiedGoogle(
	ctx context.Context,
	userID uint,
	verified *googleauthapp.VerifiedIdentity,
) (*GoogleBinding, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	if s == nil || s.userOAuthIdentityRepo == nil || s.authUnitOfWork == nil {
		return nil, googleauthapp.ErrGoogleAuthConfigInvalid
	}
	normalized, err := normalizeVerifiedGoogleIdentity(verified)
	if err != nil {
		return nil, err
	}
	verified = normalized
	if ctx == nil {
		ctx = context.Background()
	}

	var user *userdomain.User
	var current *externalidentitydomain.Identity
	err = s.authUnitOfWork.WithinTransaction(ctx, func(tx AuthTransaction) error {
		var txErr error
		user, txErr = activeTransactionUser(tx, userID)
		if txErr != nil {
			return txErr
		}

		occupied, txErr := tx.GetIdentityByProviderUserID(constants.UserOAuthProviderGoogle, verified.Sub)
		if txErr != nil {
			return txErr
		}
		if occupied != nil && occupied.UserID != userID {
			return ErrUserOAuthIdentityExists
		}

		current, txErr = tx.GetIdentityByUserProvider(userID, constants.UserOAuthProviderGoogle)
		if txErr != nil {
			return txErr
		}
		if current != nil && current.ProviderUserID != verified.Sub {
			return ErrUserOAuthAlreadyBound
		}
		if current == nil {
			now := time.Now()
			current = &externalidentitydomain.Identity{
				UserID:         userID,
				Provider:       constants.UserOAuthProviderGoogle,
				ProviderUserID: verified.Sub,
				Username:       verified.Email,
				AvatarURL:      verified.Picture,
				AuthAt:         googleAuthTime(verified, now),
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			return tx.CreateIdentity(current)
		}
		if applyGoogleIdentity(verified, current) {
			current.UpdatedAt = time.Now()
			return tx.UpdateIdentity(current)
		}
		return nil
	})
	if err != nil {
		// Translate database uniqueness races into stable application errors.
		occupied, occupiedErr := s.userOAuthIdentityRepo.GetByProviderUserID(constants.UserOAuthProviderGoogle, verified.Sub)
		if occupiedErr == nil && occupied != nil && occupied.UserID != userID {
			return nil, ErrUserOAuthIdentityExists
		}
		latest, latestErr := s.userOAuthIdentityRepo.GetByUserProvider(userID, constants.UserOAuthProviderGoogle)
		if latestErr == nil && latest != nil && latest.ProviderUserID != verified.Sub {
			return nil, ErrUserOAuthAlreadyBound
		}
		return nil, err
	}
	return s.buildGoogleBinding(user, current)
}

// GetGoogleBinding returns the current binding and whether deleting it would
// leave the account with another usable login method.
func (s *Service) GetGoogleBinding(userID uint) (*GoogleBinding, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	if s == nil || s.userOAuthIdentityRepo == nil {
		return nil, googleauthapp.ErrGoogleAuthConfigInvalid
	}
	user, err := s.getActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(userID, constants.UserOAuthProviderGoogle)
	if err != nil {
		return nil, err
	}
	return s.buildGoogleBinding(user, identity)
}

// UnbindGoogle removes a Google binding only when a local password or another
// third-party identity remains usable.
func (s *Service) UnbindGoogle(userID uint) error {
	if err := s.unbindExternalIdentity(userID, constants.UserOAuthProviderGoogle); err != nil {
		if err == errExternalIdentityUnbindLocked {
			return ErrGoogleUnbindLocked
		}
		return err
	}
	return nil
}

func (s *Service) unbindExternalIdentity(userID uint, provider string) error {
	if userID == 0 {
		return ErrNotFound
	}
	if s == nil || s.authUnitOfWork == nil {
		return googleauthapp.ErrGoogleAuthConfigInvalid
	}
	return s.authUnitOfWork.WithinTransaction(context.Background(), func(tx AuthTransaction) error {
		user, err := activeTransactionUser(tx, userID)
		if err != nil {
			return err
		}
		identity, err := tx.GetIdentityByUserProvider(userID, provider)
		if err != nil {
			return err
		}
		if identity == nil {
			return ErrUserOAuthNotBound
		}
		identities, err := tx.ListIdentitiesByUserID(userID)
		if err != nil {
			return err
		}
		if !s.hasUsableLoginAfterRemoval(user, identity.ID, identities) {
			return errExternalIdentityUnbindLocked
		}
		return tx.DeleteIdentityByID(identity.ID)
	})
}

func (s *Service) hasUsableLoginAfterRemoval(
	user *userdomain.User,
	removedIdentityID uint,
	identities []externalidentitydomain.Identity,
) bool {
	if hasUsableLocalPassword(user) {
		return true
	}
	for index := range identities {
		if identities[index].ID != removedIdentityID && s.isUsableExternalIdentity(&identities[index]) {
			return true
		}
	}
	return false
}

func hasUsableLocalPassword(user *userdomain.User) bool {
	if user == nil || user.PasswordSetupRequired {
		return false
	}
	_, err := bcrypt.Cost([]byte(strings.TrimSpace(user.PasswordHash)))
	return err == nil
}

func (s *Service) isUsableExternalIdentity(identity *externalidentitydomain.Identity) bool {
	if s == nil || identity == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(identity.Provider)) {
	case constants.UserOAuthProviderGoogle:
		if s.googleAuthService == nil {
			return false
		}
		public := s.googleAuthService.PublicConfig()
		enabled, _ := public["enabled"].(bool)
		clientID, _ := public["client_id"].(string)
		return enabled && strings.TrimSpace(clientID) != ""
	case constants.UserOAuthProviderTelegram:
		if s.telegramAuthService == nil {
			return false
		}
		public := s.telegramAuthService.PublicConfig()
		enabled, _ := public["enabled"].(bool)
		mode, _ := public["mode"].(string)
		username, _ := public["bot_username"].(string)
		mode = strings.ToLower(strings.TrimSpace(mode))
		return enabled &&
			strings.TrimSpace(username) != "" &&
			(mode == "widget" || mode == "oidc")
	default:
		// Unknown providers are not assumed to be a usable recovery method.
		return false
	}
}

func (s *Service) completeExternalLogin(user *userdomain.User, source string) (*UserLoginResult, error) {
	if user == nil {
		return nil, ErrNotFound
	}
	if user.TOTPEnabledAt != nil {
		challengeToken, jti, expiresAt, err := s.IssueUserChallengeTokenForSource(user.ID, false, source)
		if err != nil {
			return nil, err
		}
		return &UserLoginResult{
			RequiresTOTP:       true,
			User:               user,
			ChallengeToken:     challengeToken,
			ChallengeJTI:       jti,
			ChallengeExpiresAt: expiresAt,
		}, nil
	}

	token, expiresAt, err := s.GenerateUserJWT(user, 0)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	user.LastLoginAt = &now
	user.UpdatedAt = now
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	_ = cache.SetUserAuthState(context.Background(), cache.BuildUserAuthState(user))
	return &UserLoginResult{
		User:      user,
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}

func (s *Service) buildGoogleBinding(user *userdomain.User, identity *externalidentitydomain.Identity) (*GoogleBinding, error) {
	result := &GoogleBinding{Identity: identity}
	if user != nil {
		result.Email = user.Email
		result.DisplayName = user.DisplayName
	}
	if identity == nil {
		return result, nil
	}
	if strings.TrimSpace(identity.Username) != "" {
		result.Email = strings.TrimSpace(identity.Username)
	}
	canUnbind, err := s.canUnbindExternalIdentity(user, identity)
	if err != nil {
		return nil, err
	}
	result.CanUnbind = canUnbind
	return result, nil
}

func (s *Service) canUnbindExternalIdentity(user *userdomain.User, identity *externalidentitydomain.Identity) (bool, error) {
	if user == nil || identity == nil {
		return false, nil
	}
	if hasUsableLocalPassword(user) {
		return true, nil
	}
	identities, err := s.userOAuthIdentityRepo.ListByUserID(user.ID)
	if err != nil {
		return false, err
	}
	return s.hasUsableLoginAfterRemoval(user, identity.ID, identities), nil
}

func normalizeVerifiedGoogleIdentity(verified *googleauthapp.VerifiedIdentity) (*googleauthapp.VerifiedIdentity, error) {
	if verified == nil {
		return nil, googleauthapp.ErrGoogleCredentialInvalid
	}
	subject := strings.TrimSpace(verified.Sub)
	if subject == "" || len(subject) > maxGoogleProviderUserIDBytes {
		return nil, googleauthapp.ErrGoogleCredentialInvalid
	}
	email, err := normalizeEmail(verified.Email)
	if err != nil || len(email) > maxGoogleIdentityEmailBytes {
		return nil, googleauthapp.ErrGoogleCredentialInvalid
	}
	copyValue := *verified
	copyValue.Sub = subject
	copyValue.Email = email
	copyValue.Name = strings.TrimSpace(verified.Name)
	copyValue.Picture = normalizeGoogleIdentityPicture(verified.Picture)
	copyValue.HD = strings.ToLower(strings.TrimSpace(verified.HD))
	return &copyValue, nil
}

func normalizeGoogleIdentityPicture(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" || len(normalized) > maxGooglePictureURLBytes {
		return ""
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

func applyGoogleIdentity(verified *googleauthapp.VerifiedIdentity, identity *externalidentitydomain.Identity) bool {
	if verified == nil || identity == nil {
		return false
	}
	changed := false
	if identity.Provider != constants.UserOAuthProviderGoogle {
		identity.Provider = constants.UserOAuthProviderGoogle
		changed = true
	}
	if identity.ProviderUserID != verified.Sub {
		identity.ProviderUserID = verified.Sub
		changed = true
	}
	if identity.Username != verified.Email {
		identity.Username = verified.Email
		changed = true
	}
	if identity.AvatarURL != verified.Picture {
		identity.AvatarURL = verified.Picture
		changed = true
	}
	authAt := verified.AuthAt
	if authAt.IsZero() {
		authAt = time.Now()
	}
	if identity.AuthAt == nil || !identity.AuthAt.Equal(authAt) {
		identity.AuthAt = &authAt
		changed = true
	}
	return changed
}

func googleAuthTime(verified *googleauthapp.VerifiedIdentity, fallback time.Time) *time.Time {
	authAt := fallback
	if verified != nil && !verified.AuthAt.IsZero() {
		authAt = verified.AuthAt
	}
	return &authAt
}

func generateGooglePlaceholderPassword() (string, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(seed)
	hashed, err := bcrypt.GenerateFromPassword([]byte(encoded), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}
