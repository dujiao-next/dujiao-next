package adminapitokenapp

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	tokencontract "github.com/dujiao-next/internal/modules/identity/adminapitoken/contract"
	tokendomain "github.com/dujiao-next/internal/modules/identity/adminapitoken/domain"
)

const Prefix = "dj_admin_"

var (
	ErrInvalid    = errors.New("invalid admin API token")
	ErrInput      = errors.New("invalid admin API token input")
	ErrSubAccount = errors.New("API tokens require a non-super administrator")
	ErrNotFound   = errors.New("admin API token not found or already revoked")
	ErrRotation   = errors.New("admin API token is inactive or changed during rotation")
)

type Service struct {
	tokens tokencontract.Store
	admins admincontract.Store
}

// Owner is the account state needed by machine authentication.
type Owner struct {
	ID                 uint
	Username           string
	IsSuper            bool
	TokenVersion       uint64
	TokenInvalidBefore *time.Time
}

func (s *Service) owner(adminID uint) (*Owner, error) {
	admin, err := s.admins.GetByID(adminID)
	if err != nil || admin == nil || admin.DeletedAt != nil {
		return nil, err
	}
	return &Owner{
		ID: admin.ID, Username: admin.Username, IsSuper: admin.IsSuper,
		TokenVersion: admin.TokenVersion, TokenInvalidBefore: admin.TokenInvalidBefore,
	}, nil
}

func NewService(tokens tokencontract.Store, admins admincontract.Store) *Service {
	return &Service{tokens: tokens, admins: admins}
}

func (s *Service) Create(adminID uint, name string, expiresAt time.Time, ipWhitelist []string) (*tokendomain.Token, string, error) {
	now := time.Now()
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > 80 || !expiresAt.After(now) || expiresAt.After(now.Add(365*24*time.Hour)) {
		return nil, "", ErrInput
	}
	ipWhitelist, err := normalizeIPWhitelist(ipWhitelist)
	if err != nil {
		return nil, "", err
	}
	admin, err := s.owner(adminID)
	if err != nil {
		return nil, "", err
	}
	if admin == nil || admin.IsSuper {
		return nil, "", ErrSubAccount
	}
	secret, err := generateSecret()
	if err != nil {
		return nil, "", err
	}
	token := &tokendomain.Token{
		AdminID: admin.ID, Name: name, Digest: digest(secret),
		AdminTokenVersion: admin.TokenVersion,
		IPWhitelist:       ipWhitelist,
		ExpiresAt:         expiresAt.UTC(), CreatedAt: now.UTC(), Status: "active",
	}
	if err := s.tokens.Create(token); err != nil {
		return nil, "", err
	}
	return token, secret, nil
}

func (s *Service) List(adminID uint) ([]tokendomain.Token, error) {
	items, err := s.tokens.List(adminID)
	if err != nil {
		return nil, err
	}
	admin, err := s.owner(adminID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	for i := range items {
		token := &items[i]
		if token.IPWhitelist == nil {
			token.IPWhitelist = []string{}
		}
		switch {
		case token.RevokedAt != nil:
			token.Status = "revoked"
		case !now.Before(token.ExpiresAt):
			token.Status = "expired"
		case !validOwner(admin, token):
			token.Status = "invalidated"
		default:
			token.Status = "active"
		}
	}
	return items, nil
}

func (s *Service) Revoke(adminID, tokenID uint) error {
	ok, err := s.tokens.Revoke(adminID, tokenID, time.Now())
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// Rotate preserves identity, expiry and IP restrictions. It never refreshes the
// owner security version: rotating cannot revive an invalidated credential.
func (s *Service) Rotate(adminID, tokenID uint) (*tokendomain.Token, string, error) {
	token, err := s.tokens.GetByID(adminID, tokenID)
	if err != nil {
		return nil, "", err
	}
	if token == nil {
		return nil, "", ErrNotFound
	}
	admin, err := s.owner(adminID)
	if err != nil {
		return nil, "", err
	}
	if token.RevokedAt != nil || !time.Now().Before(token.ExpiresAt) || !validOwner(admin, token) {
		return nil, "", ErrRotation
	}
	secret, err := generateSecret()
	if err != nil {
		return nil, "", err
	}
	newDigest := digest(secret)
	ok, err := s.tokens.Rotate(adminID, tokenID, token.Digest, newDigest, time.Now())
	if err != nil {
		return nil, "", err
	}
	if !ok {
		return nil, "", ErrRotation
	}
	token.Digest, token.LastUsedAt, token.Status = newDigest, nil, "active"
	if token.IPWhitelist == nil {
		token.IPWhitelist = []string{}
	}
	return token, secret, nil
}

func generateSecret() (string, error) {
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return Prefix + hex.EncodeToString(random), nil
}

// Authenticate deliberately bypasses the JWT auth-state cache. Credential and
// owner state are re-read on each request, including CLI password/2FA resets.
func (s *Service) Authenticate(secret string) (*tokendomain.Token, *Owner, error) {
	if len(secret) != len(Prefix)+64 || !strings.HasPrefix(secret, Prefix) {
		return nil, nil, ErrInvalid
	}
	if _, err := hex.DecodeString(strings.TrimPrefix(secret, Prefix)); err != nil {
		return nil, nil, ErrInvalid
	}
	token, err := s.tokens.GetByDigest(digest(secret))
	if err != nil {
		return nil, nil, err
	}
	if token == nil || token.RevokedAt != nil || !time.Now().Before(token.ExpiresAt) {
		return nil, nil, ErrInvalid
	}
	admin, err := s.owner(token.AdminID)
	if err != nil {
		return nil, nil, err
	}
	if !validOwner(admin, token) {
		return nil, nil, ErrInvalid
	}
	return token, admin, nil
}

func validOwner(admin *Owner, token *tokendomain.Token) bool {
	return admin != nil && !admin.IsSuper &&
		admin.TokenVersion == token.AdminTokenVersion &&
		(admin.TokenInvalidBefore == nil || token.CreatedAt.After(*admin.TokenInvalidBefore))
}

func (s *Service) Touch(tokenID uint) error { return s.tokens.Touch(tokenID, time.Now()) }

func digest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
