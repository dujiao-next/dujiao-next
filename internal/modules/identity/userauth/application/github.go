package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	githubauthapp "github.com/dujiao-next/internal/modules/identity/githubauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"golang.org/x/crypto/bcrypt"
)

func (s *Service) LoginVerifiedGitHub(v *githubauthapp.VerifiedIdentity) (*UserLoginResult, error) {
	if s == nil || s.userOAuthIdentityRepo == nil || s.authUnitOfWork == nil || v == nil {
		return nil, githubauthapp.ErrGitHubCodeInvalid
	}
	id := strings.TrimSpace(v.ID)
	email, err := normalizeEmail(v.Email)
	if err != nil || id == "" {
		return nil, githubauthapp.ErrGitHubEmailUnverified
	}
	identity, err := s.userOAuthIdentityRepo.GetByProviderUserID(constants.UserOAuthProviderGitHub, id)
	if err != nil {
		return nil, err
	}
	var existing *userdomain.User
	if identity == nil {
		existing, err = s.userRepo.GetByEmail(email)
		if err != nil {
			return nil, err
		}
	}
	var user *userdomain.User
	created := false
	var registration googleRegistrationSnapshot
	if identity == nil && existing == nil {
		registration, err = s.loadGoogleRegistrationSnapshot()
		if err != nil {
			return nil, err
		}
		if !registration.Enabled {
			return nil, ErrRegistrationDisabled
		}
		if err = settingsapp.CheckRegistrationEmailDomainAllowed(email, registration.EmailDomain); err != nil {
			return nil, err
		}
	}
	err = s.authUnitOfWork.WithinTransaction(context.Background(), func(tx AuthTransaction) error {
		if identity != nil {
			user, err = activeTransactionUser(tx, identity.UserID)
			return err
		}
		user, err = tx.GetUserByEmail(email)
		if err != nil {
			return err
		}
		if existing != nil && user == nil {
			return errGoogleLoginMappingChanged
		}
		if user == nil {
			user, err = newGitHubUser(v, email)
			if err != nil {
				return err
			}
			if err = tx.CreateUser(user); err != nil {
				return err
			}
			created = true
		}
		current, err := tx.GetIdentityByUserProvider(user.ID, constants.UserOAuthProviderGitHub)
		if err != nil {
			return err
		}
		if current != nil && current.ProviderUserID != id {
			return ErrUserOAuthAlreadyBound
		}
		if current == nil {
			now := time.Now()
			current = &externalidentitydomain.Identity{UserID: user.ID, Provider: constants.UserOAuthProviderGitHub, ProviderUserID: id, Username: email, AvatarURL: v.AvatarURL, AuthAt: &now, CreatedAt: now, UpdatedAt: now}
			return tx.CreateIdentity(current)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if created && s.memberLevelSvc != nil {
		_ = s.memberLevelSvc.AssignDefaultLevel(user.ID)
	}
	return s.completeExternalLogin(user, constants.LoginLogSourceGitHub)
}
func newGitHubUser(v *githubauthapp.VerifiedIdentity, email string) (*userdomain.User, error) {
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(base64.RawURLEncoding.EncodeToString(seed)), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	name := strings.TrimSpace(v.Name)
	if name == "" {
		name = strings.TrimSpace(v.Login)
	}
	if name == "" {
		name = resolveNicknameFromEmail(email)
	}
	return &userdomain.User{Email: email, PasswordHash: string(hash), PasswordSetupRequired: true, DisplayName: truncateRunes(name, 128), Status: constants.UserStatusActive, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now}, nil
}

const GitHubOAuthIntentTTL = 10 * time.Minute

var ErrGitHubOAuthStateExpired = errors.New("github oauth state expired")

type GitHubOAuthIntent struct {
	Origin    string
	Tenant    GitHubRedirectTenant
	CreatedAt time.Time
}
type GitHubRedirectTenant struct {
	Host          string
	IsMain        bool
	HasResellerID bool
	ResellerID    uint
}

func (s *Service) CreateGitHubOAuthIntent(ctx context.Context, intent GitHubOAuthIntent) (string, error) {
	if s == nil || s.gitHubStateStore == nil {
		return "", ErrGitHubOAuthStateExpired
	}
	origin, err := normalizeGitHubOrigin(intent.Origin)
	if err != nil {
		return "", err
	}
	intent.Origin = origin
	var seed [32]byte
	if _, err = rand.Read(seed[:]); err != nil {
		return "", err
	}
	state := base64.RawURLEncoding.EncodeToString(seed[:])
	if err = s.gitHubStateStore.PutGitHubIntent(ctx, state, intent, GitHubOAuthIntentTTL); err != nil {
		return "", err
	}
	return state, nil
}
func (s *Service) ConsumeGitHubOAuthIntent(ctx context.Context, state string) (*GitHubOAuthIntent, error) {
	if s == nil || s.gitHubStateStore == nil || !validGitHubState(state) {
		return nil, ErrGitHubOAuthStateExpired
	}
	intent, err := s.gitHubStateStore.TakeGitHubIntent(ctx, state)
	if err != nil {
		return nil, err
	}
	if intent == nil {
		return nil, ErrGitHubOAuthStateExpired
	}
	return intent, nil
}
func validGitHubState(v string) bool {
	if len(v) != 43 {
		return false
	}
	b, err := base64.RawURLEncoding.DecodeString(v)
	return err == nil && len(b) == 32 && base64.RawURLEncoding.EncodeToString(b) == v
}
func BuildGitHubCallbackURL(origin string) (string, error) {
	normalized, err := normalizeGitHubOrigin(origin)
	if err != nil {
		return "", err
	}
	return normalized + "/api/v1/auth/github/callback", nil
}
func normalizeGitHubOrigin(origin string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(origin))
	if err != nil || u.User != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid trusted site origin")
	}
	return u.Scheme + "://" + u.Host, nil
}
