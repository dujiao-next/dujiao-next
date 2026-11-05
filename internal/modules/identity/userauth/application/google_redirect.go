package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
	"time"

	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
)

const (
	GoogleRedirectFlowLogin = "login"
	GoogleRedirectFlowBind  = "bind"

	GoogleRedirectIntentTTL  = 10 * time.Minute
	GoogleRedirectHandoffTTL = 2 * time.Minute
)

// GoogleRedirectTenant is the minimum tenant identity persisted with an OAuth
// redirect. Host-only cookies are not sufficient because deployments may
// legitimately serve multiple tenants from one parent domain.
type GoogleRedirectTenant struct {
	Host          string
	IsMain        bool
	HasResellerID bool
	ResellerID    uint
}

// GoogleRedirectIntent binds a browser redirect to its original flow, user
// (for binding), and storefront tenant.
type GoogleRedirectIntent struct {
	Flow      string
	UserID    uint
	Tenant    GoogleRedirectTenant
	CreatedAt time.Time
}

// GoogleRedirectCompletion is returned after Google credentials have been
// verified and replaced by a short-lived, one-time handoff.
type GoogleRedirectCompletion struct {
	Flow          string
	HandoffHandle string
}

// CreateGoogleRedirectIntent creates state for either login or authenticated
// account binding. Redis is mandatory only for this redirect UX; popup login
// does not use the redirect store.
func (s *Service) CreateGoogleRedirectIntent(
	ctx context.Context,
	flow string,
	userID uint,
	tenant GoogleRedirectTenant,
) (string, error) {
	if err := s.validateGoogleRedirectDependencies(); err != nil {
		return "", err
	}
	flow = strings.ToLower(strings.TrimSpace(flow))
	if !validGoogleRedirectFlow(flow) || (flow == GoogleRedirectFlowBind && userID == 0) {
		return "", ErrGoogleRedirectFlowInvalid
	}
	if flow == GoogleRedirectFlowLogin {
		userID = 0
	}
	normalizedTenant, ok := normalizeGoogleRedirectTenant(tenant)
	if !ok {
		return "", ErrGoogleRedirectTenantMismatch
	}
	state, err := newGoogleRedirectHandle()
	if err != nil {
		return "", err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	intent := GoogleRedirectIntent{
		Flow:      flow,
		UserID:    userID,
		Tenant:    normalizedTenant,
		CreatedAt: time.Now().UTC(),
	}
	if err = s.googleRedirectStore.PutIntent(ctx, state, intent, GoogleRedirectIntentTTL); err != nil {
		return "", errors.Join(ErrGoogleRedirectUnavailable, err)
	}
	return state, nil
}

// CompleteGoogleRedirect atomically consumes state before verifying the
// credential. Any tenant or credential failure therefore cannot replay the
// same intent. Only normalized verified claims are written to the handoff.
func (s *Service) CompleteGoogleRedirect(
	ctx context.Context,
	state string,
	credential string,
	tenant GoogleRedirectTenant,
) (*GoogleRedirectCompletion, error) {
	if s == nil || s.googleRedirectStore == nil {
		return nil, ErrGoogleRedirectUnavailable
	}
	if !ValidGoogleRedirectHandle(state) {
		return nil, ErrGoogleRedirectSessionExpired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	intent, err := s.googleRedirectStore.TakeIntent(ctx, state)
	if err != nil {
		return nil, errors.Join(ErrGoogleRedirectUnavailable, err)
	}
	if intent == nil {
		return nil, ErrGoogleRedirectSessionExpired
	}
	if !validGoogleRedirectFlow(intent.Flow) {
		return nil, ErrGoogleRedirectFlowInvalid
	}
	completion := &GoogleRedirectCompletion{Flow: intent.Flow}
	currentTenant, ok := normalizeGoogleRedirectTenant(tenant)
	if !ok {
		return completion, ErrGoogleRedirectTenantMismatch
	}
	if !sameGoogleRedirectTenant(intent.Tenant, currentTenant) {
		return completion, ErrGoogleRedirectTenantMismatch
	}
	if s.googleAuthService == nil {
		return completion, googleauthapp.ErrGoogleAuthConfigInvalid
	}

	verified, err := s.googleAuthService.VerifyCredential(ctx, credential)
	if err != nil {
		return completion, err
	}
	verified, err = normalizeVerifiedGoogleIdentity(verified)
	if err != nil {
		return completion, err
	}
	handle, err := newGoogleRedirectHandle()
	if err != nil {
		return completion, err
	}
	handoff := GoogleRedirectHandoff{
		Flow:      intent.Flow,
		UserID:    intent.UserID,
		Tenant:    intent.Tenant,
		Identity:  *verified,
		CreatedAt: time.Now().UTC(),
	}
	if err = s.googleRedirectStore.PutHandoff(ctx, handle, handoff, GoogleRedirectHandoffTTL); err != nil {
		return completion, errors.Join(ErrGoogleRedirectUnavailable, err)
	}
	completion.HandoffHandle = handle
	return completion, nil
}

// ExchangeGoogleRedirectLogin atomically consumes a login handoff and
// continues through the same JWT/2FA path as popup authentication.
func (s *Service) ExchangeGoogleRedirectLogin(
	ctx context.Context,
	handle string,
	tenant GoogleRedirectTenant,
) (*UserLoginResult, error) {
	handoff, err := s.takeGoogleRedirectHandoff(
		ctx,
		handle,
		GoogleRedirectFlowLogin,
		0,
		tenant,
	)
	if err != nil {
		return nil, err
	}
	if err = s.validateGoogleRedirectHandoffRuntime(handoff); err != nil {
		return nil, err
	}
	return s.loginVerifiedGoogle(ctx, &handoff.Identity, "")
}

// ExchangeGoogleRedirectBind atomically consumes a bind handoff. The
// authenticated user must be the same user that created the intent.
func (s *Service) ExchangeGoogleRedirectBind(
	ctx context.Context,
	handle string,
	userID uint,
	tenant GoogleRedirectTenant,
) (*GoogleBinding, error) {
	if userID == 0 {
		return nil, ErrGoogleRedirectUserMismatch
	}
	handoff, err := s.takeGoogleRedirectHandoff(
		ctx,
		handle,
		GoogleRedirectFlowBind,
		userID,
		tenant,
	)
	if err != nil {
		return nil, err
	}
	if err = s.validateGoogleRedirectHandoffRuntime(handoff); err != nil {
		return nil, err
	}
	return s.bindVerifiedGoogle(ctx, userID, &handoff.Identity)
}

func (s *Service) takeGoogleRedirectHandoff(
	ctx context.Context,
	handle string,
	expectedFlow string,
	expectedUserID uint,
	tenant GoogleRedirectTenant,
) (*GoogleRedirectHandoff, error) {
	if s == nil || s.googleRedirectStore == nil {
		return nil, ErrGoogleRedirectUnavailable
	}
	if !ValidGoogleRedirectHandle(handle) {
		return nil, ErrGoogleRedirectSessionExpired
	}
	if ctx == nil {
		ctx = context.Background()
	}
	handoff, err := s.googleRedirectStore.TakeHandoff(ctx, handle)
	if err != nil {
		return nil, errors.Join(ErrGoogleRedirectUnavailable, err)
	}
	if handoff == nil {
		return nil, ErrGoogleRedirectSessionExpired
	}
	currentTenant, ok := normalizeGoogleRedirectTenant(tenant)
	if !ok {
		return nil, ErrGoogleRedirectTenantMismatch
	}
	if handoff.Flow != expectedFlow {
		return nil, ErrGoogleRedirectFlowInvalid
	}
	if !sameGoogleRedirectTenant(handoff.Tenant, currentTenant) {
		return nil, ErrGoogleRedirectTenantMismatch
	}
	if expectedFlow == GoogleRedirectFlowBind && handoff.UserID != expectedUserID {
		return nil, ErrGoogleRedirectUserMismatch
	}
	if expectedFlow == GoogleRedirectFlowLogin && handoff.UserID != 0 {
		return nil, ErrGoogleRedirectFlowInvalid
	}
	return handoff, nil
}

func (s *Service) validateGoogleRedirectHandoffRuntime(handoff *GoogleRedirectHandoff) error {
	if s == nil || s.googleAuthService == nil {
		return googleauthapp.ErrGoogleAuthConfigInvalid
	}
	clientID, err := s.googleAuthService.RuntimeClientID()
	if err != nil {
		return err
	}
	// A client ID change changes the accepted ID-token audience. A handoff
	// verified under the old audience is conservatively invalidated even
	// within its two-minute TTL.
	if handoff == nil || handoff.Identity.ClientID == "" || handoff.Identity.ClientID != clientID {
		return ErrGoogleRedirectSessionExpired
	}
	return nil
}

func (s *Service) validateGoogleRedirectDependencies() error {
	if s == nil || s.googleAuthService == nil {
		return googleauthapp.ErrGoogleAuthConfigInvalid
	}
	if err := s.googleAuthService.ValidateRuntimeConfig(); err != nil {
		return err
	}
	if s.googleRedirectStore == nil {
		return ErrGoogleRedirectUnavailable
	}
	return nil
}

func validGoogleRedirectFlow(flow string) bool {
	return flow == GoogleRedirectFlowLogin || flow == GoogleRedirectFlowBind
}

func normalizeGoogleRedirectTenant(tenant GoogleRedirectTenant) (GoogleRedirectTenant, bool) {
	tenant.Host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(tenant.Host), "."))
	if tenant.Host == "" {
		return GoogleRedirectTenant{}, false
	}
	if tenant.IsMain {
		if tenant.HasResellerID || tenant.ResellerID != 0 {
			return GoogleRedirectTenant{}, false
		}
		return tenant, true
	}
	if !tenant.HasResellerID || tenant.ResellerID == 0 {
		return GoogleRedirectTenant{}, false
	}
	return tenant, true
}

func sameGoogleRedirectTenant(left, right GoogleRedirectTenant) bool {
	left, leftOK := normalizeGoogleRedirectTenant(left)
	right, rightOK := normalizeGoogleRedirectTenant(right)
	return leftOK &&
		rightOK &&
		left.Host == right.Host &&
		left.IsMain == right.IsMain &&
		left.HasResellerID == right.HasResellerID &&
		left.ResellerID == right.ResellerID
}

func newGoogleRedirectHandle() (string, error) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(seed[:]), nil
}

// ValidGoogleRedirectHandle accepts only the canonical format generated by
// this service: 32 random bytes encoded as unpadded base64url.
func ValidGoogleRedirectHandle(value string) bool {
	if len(value) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil &&
		len(decoded) == 32 &&
		base64.RawURLEncoding.EncodeToString(decoded) == value
}
