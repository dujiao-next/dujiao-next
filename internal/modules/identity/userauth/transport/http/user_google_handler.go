package userauthhttp

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/i18n"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

const (
	maxGoogleCredentialRequestBodyBytes = 64 << 10

	googleRedirectIntentCookieName  = "__Host-dujiao_google_state"
	googleRedirectHandoffCookieName = "__Host-dujiao_google_handoff"

	googleRedirectFlowLogin = "login"
	googleRedirectFlowBind  = "bind"

	googleRedirectIntentTTLSeconds  = 10 * 60
	googleRedirectHandoffTTLSeconds = 2 * 60
)

var (
	ErrGoogleAuthDisabled           = errors.New("google auth disabled")
	ErrGoogleAuthConfigInvalid      = errors.New("google auth config invalid")
	ErrGoogleCredentialInvalid      = errors.New("google credential invalid")
	ErrGoogleCredentialExpired      = errors.New("google credential expired")
	ErrGoogleEmailUnverified        = errors.New("google email unverified")
	ErrGoogleJWKSUnavailable        = errors.New("google jwks unavailable")
	ErrGoogleAutoLinkForbidden      = errors.New("google email auto link forbidden")
	ErrGoogleUnbindLocked           = errors.New("google unbind would lock account")
	ErrGoogleRedirectUnavailable    = errors.New("google redirect state store unavailable")
	ErrGoogleRedirectSessionExpired = errors.New("google redirect session expired")
	ErrGoogleRedirectTenantMismatch = errors.New("google redirect tenant mismatch")
	ErrGoogleRedirectUserMismatch   = errors.New("google redirect user mismatch")
	ErrGoogleRedirectFlowInvalid    = errors.New("google redirect flow invalid")
)

// GoogleBindingResult is the transport view returned by the application adapter.
type GoogleBindingResult struct {
	Identity    *externalidentitydomain.Identity
	Email       string
	DisplayName string
	CanUnbind   bool
}

// GoogleRedirectTenant is the transport-safe tenant identity bound into each
// one-time redirect state record.
type GoogleRedirectTenant struct {
	Host          string
	IsMain        bool
	HasResellerID bool
	ResellerID    uint
}

type GoogleRedirectCompletionResult struct {
	Flow          string
	HandoffHandle string
}

// UserGoogleService is the minimal Google authentication transport port.
type UserGoogleService interface {
	LoginWithGoogle(ctx context.Context, credential string) (*AuthLoginResult, error)
	GetGoogleBinding(userID uint) (*GoogleBindingResult, error)
	BindGoogle(ctx context.Context, userID uint, credential string) (*GoogleBindingResult, error)
	UnbindGoogle(userID uint) error
	CreateGoogleRedirectIntent(ctx context.Context, flow string, userID uint, tenant GoogleRedirectTenant) (string, error)
	CompleteGoogleRedirect(ctx context.Context, state, credential string, tenant GoogleRedirectTenant) (*GoogleRedirectCompletionResult, error)
	ExchangeGoogleRedirectLogin(ctx context.Context, handle string, tenant GoogleRedirectTenant) (*AuthLoginResult, error)
	ExchangeGoogleRedirectBind(ctx context.Context, handle string, userID uint, tenant GoogleRedirectTenant) (*GoogleBindingResult, error)
}

// UserGoogleHandler handles Google Identity Services login and binding.
type UserGoogleHandler struct {
	service  UserGoogleService
	recorder LoginRecorder
}

func NewUserGoogleHandler(service UserGoogleService, recorder LoginRecorder) *UserGoogleHandler {
	if service == nil {
		panic("user google handler: service is nil")
	}
	return &UserGoogleHandler{service: service, recorder: recorder}
}

type UserGoogleCredentialRequest struct {
	Credential string `json:"credential" binding:"required"`
}

func bindGoogleCredentialRequest(c *gin.Context, request *UserGoogleCredentialRequest) (bool, error) {
	if c == nil || c.Request == nil {
		return false, errors.New("google credential request is unavailable")
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxGoogleCredentialRequestBodyBytes)
	err := c.ShouldBindJSON(request)
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr), err
}

func respondGoogleCredentialRequestError(c *gin.Context, tooLarge bool, err error) {
	if tooLarge {
		response.ErrorWithHTTPStatus(
			c,
			http.StatusRequestEntityTooLarge,
			response.CodeBadRequest,
			i18n.T(i18n.ResolveLocale(c), "error.request_too_large"),
		)
		return
	}
	ginutil.RespondBindError(c, err)
}

type googleLoginErrorRule struct {
	target     error
	code       int
	key        string
	failReason string
	logErr     bool
}

var googleLoginErrorRules = []googleLoginErrorRule{
	{target: ErrGoogleAuthDisabled, code: response.CodeBadRequest, key: "error.google_auth_disabled", failReason: constants.LoginLogFailReasonGoogleConfig},
	{target: ErrGoogleAuthConfigInvalid, code: response.CodeInternal, key: "error.google_auth_config_invalid", failReason: constants.LoginLogFailReasonGoogleConfig, logErr: true},
	{target: ErrGoogleJWKSUnavailable, code: response.CodeInternal, key: "error.google_service_unavailable", failReason: constants.LoginLogFailReasonGoogleConfig, logErr: true},
	{target: ErrGoogleCredentialInvalid, code: response.CodeBadRequest, key: "error.google_credential_invalid", failReason: constants.LoginLogFailReasonGoogleInvalid},
	{target: ErrGoogleCredentialExpired, code: response.CodeBadRequest, key: "error.google_credential_expired", failReason: constants.LoginLogFailReasonGoogleInvalid},
	{target: ErrGoogleEmailUnverified, code: response.CodeBadRequest, key: "error.google_email_unverified", failReason: constants.LoginLogFailReasonGoogleInvalid},
	{target: ErrGoogleAutoLinkForbidden, code: response.CodeForbidden, key: "error.google_auto_link_forbidden", failReason: constants.LoginLogFailReasonGoogleInvalid},
	{target: ErrUserOAuthAlreadyBound, code: response.CodeBadRequest, key: "error.google_already_bound", failReason: constants.LoginLogFailReasonGoogleInvalid},
	{target: ErrEmailDomainNotAllowed, code: response.CodeBadRequest, key: "error.email_domain_not_allowed", failReason: constants.LoginLogFailReasonGoogleInvalid},
	{target: ErrUserDisabled, code: response.CodeUnauthorized, key: "error.user_disabled", failReason: constants.LoginLogFailReasonUserDisabled},
	{target: ErrRegistrationDisabled, code: response.CodeForbidden, key: "error.registration_disabled", failReason: constants.LoginLogFailReasonBadRequest},
}

func (h *UserGoogleHandler) recordLogin(c *gin.Context, email string, userID uint, status, failReason string) {
	if h != nil {
		recordLoginEvent(h.recorder, c, email, userID, status, failReason, constants.LoginLogSourceGoogle)
	}
}

func (h *UserGoogleHandler) respondGoogleLoginError(c *gin.Context, err error) {
	for _, rule := range googleLoginErrorRules {
		if !errors.Is(err, rule.target) {
			continue
		}
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, rule.failReason)
		var cause error
		if rule.logErr {
			cause = err
		}
		ginutil.RespondError(c, rule.code, rule.key, cause)
		return
	}
	h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonInternalError)
	ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
}

func respondGoogleBindError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrGoogleAuthDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_auth_disabled", nil)
	case errors.Is(err, ErrGoogleAuthConfigInvalid):
		ginutil.RespondError(c, response.CodeInternal, "error.google_auth_config_invalid", err)
	case errors.Is(err, ErrGoogleJWKSUnavailable):
		ginutil.RespondError(c, response.CodeInternal, "error.google_service_unavailable", err)
	case errors.Is(err, ErrGoogleCredentialInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_credential_invalid", nil)
	case errors.Is(err, ErrGoogleCredentialExpired):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_credential_expired", nil)
	case errors.Is(err, ErrGoogleEmailUnverified):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_email_unverified", nil)
	case errors.Is(err, ErrUserOAuthIdentityExists):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_bind_conflict", nil)
	case errors.Is(err, ErrUserOAuthAlreadyBound):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_already_bound", nil)
	case errors.Is(err, ErrUserDisabled):
		ginutil.RespondError(c, response.CodeUnauthorized, "error.user_disabled", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
	}
}

func (h *UserGoogleHandler) UserGoogleLogin(c *gin.Context) {
	var request UserGoogleCredentialRequest
	if tooLarge, err := bindGoogleCredentialRequest(c, &request); err != nil {
		h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, constants.LoginLogFailReasonBadRequest)
		respondGoogleCredentialRequestError(c, tooLarge, err)
		return
	}
	result, err := h.service.LoginWithGoogle(c.Request.Context(), request.Credential)
	if err != nil {
		h.respondGoogleLoginError(c, err)
		return
	}
	if result == nil || result.User == nil {
		h.respondGoogleLoginError(c, errors.New("google login returned an empty result"))
		return
	}
	h.respondGoogleLoginResult(c, result)
}

func (h *UserGoogleHandler) respondGoogleLoginResult(c *gin.Context, result *AuthLoginResult) {
	if respondInviteRequired(c, result) {
		return
	}
	if result == nil || result.User == nil {
		h.respondGoogleLoginError(c, errors.New("google login returned an empty result"))
		return
	}
	if result.RequiresTOTP {
		h.recordLogin(c, result.User.Email, result.User.ID, constants.LoginLogStatusSuccess, constants.LoginLogPasswordOK2FAPending)
		response.Success(c, gin.H{
			"requires_totp":        true,
			"challenge_token":      result.ChallengeToken,
			"challenge_expires_at": result.ChallengeExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		return
	}
	h.recordLogin(c, result.User.Email, result.User.ID, constants.LoginLogStatusSuccess, "")
	response.Success(c, gin.H{
		"requires_totp": false,
		"user":          userpresenter.NewUserAuthBriefResp(result.User),
		"token":         result.Token,
		"expires_at":    result.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// CreateGoogleRedirectLoginIntent creates a tenant-bound, single-use state for
// a public redirect login.
func (h *UserGoogleHandler) CreateGoogleRedirectLoginIntent(c *gin.Context) {
	h.createGoogleRedirectIntent(c, googleRedirectFlowLogin, 0)
}

// CreateGoogleRedirectBindIntent creates a tenant/user-bound state for an
// authenticated account binding redirect.
func (h *UserGoogleHandler) CreateGoogleRedirectBindIntent(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	h.createGoogleRedirectIntent(c, googleRedirectFlowBind, userID)
}

func (h *UserGoogleHandler) createGoogleRedirectIntent(c *gin.Context, flow string, userID uint) {
	setGoogleRedirectNoStoreHeaders(c)
	tenant, ok := googleRedirectTenantFromContext(c)
	if !ok {
		respondGoogleRedirectAPIError(c, ErrGoogleRedirectTenantMismatch)
		return
	}
	state, err := h.service.CreateGoogleRedirectIntent(c.Request.Context(), flow, userID, tenant)
	if err != nil {
		respondGoogleRedirectAPIError(c, err)
		return
	}
	if !validGoogleRedirectHandle(state) {
		ginutil.RespondError(c, response.CodeInternal, "error.google_auth_config_invalid", errors.New("invalid Google redirect state"))
		return
	}
	setGoogleRedirectCookie(
		c,
		googleRedirectIntentCookieName,
		state,
		googleRedirectIntentTTLSeconds,
		http.SameSiteNoneMode,
	)
	response.Success(c, gin.H{
		"state":      state,
		"expires_in": googleRedirectIntentTTLSeconds,
		"issued_at":  time.Now().UTC().Format(time.RFC3339),
	})
}

// CompleteGoogleRedirect receives Google's form_post response. It never puts
// credentials, state, or tokens into the redirect URL.
func (h *UserGoogleHandler) CompleteGoogleRedirect(c *gin.Context) {
	setGoogleRedirectNoStoreHeaders(c)
	flow := googleRedirectFlowLogin

	form, callbackErr := parseGoogleRedirectCallback(c)
	if callbackErr != "" {
		redirectGoogleCallback(c, flow, callbackErr)
		return
	}
	if !form.ValidGISCSRF {
		// An arbitrary cross-site POST must not clear a legitimate in-flight
		// intent. State cookies are cleared only after Google's double-submit
		// CSRF token has been validated.
		redirectGoogleCallback(c, flow, "csrf_mismatch")
		return
	}

	intentState, ok := singleRequestCookie(c.Request, googleRedirectIntentCookieName)
	clearGoogleRedirectIntentCookies(c)
	if !ok {
		redirectGoogleCallback(c, flow, "session_expired")
		return
	}
	if !form.ValidCredential || !form.ValidState {
		redirectGoogleCallback(c, flow, "invalid_request")
		return
	}
	if !constantTimeStringEqual(form.State, intentState) {
		redirectGoogleCallback(c, flow, "csrf_mismatch")
		return
	}
	if !validGoogleRedirectHandle(form.State) {
		redirectGoogleCallback(c, flow, "session_expired")
		return
	}
	tenant, _ := googleRedirectTenantFromContext(c)
	completion, err := h.service.CompleteGoogleRedirect(
		c.Request.Context(),
		form.State,
		form.Credential,
		tenant,
	)
	if completion != nil &&
		(completion.Flow == googleRedirectFlowLogin || completion.Flow == googleRedirectFlowBind) {
		// The application returns only the flow loaded from the atomically
		// consumed server-side intent, including on downstream failures.
		flow = completion.Flow
	}
	if err != nil {
		if flow == googleRedirectFlowLogin {
			if failReason, ok := googleRedirectVerificationFailReason(err); ok {
				h.recordLogin(c, "", 0, constants.LoginLogStatusFailed, failReason)
			}
		}
		redirectGoogleCallback(c, flow, googleRedirectCallbackError(err))
		return
	}
	if completion == nil ||
		(completion.Flow != googleRedirectFlowLogin && completion.Flow != googleRedirectFlowBind) ||
		!validGoogleRedirectHandle(completion.HandoffHandle) {
		redirectGoogleCallback(c, flow, "internal_error")
		return
	}
	setGoogleRedirectCookie(
		c,
		googleRedirectHandoffCookieName,
		completion.HandoffHandle,
		googleRedirectHandoffTTLSeconds,
		http.SameSiteLaxMode,
	)
	redirectGoogleCallback(c, completion.Flow, "")
}

// ExchangeGoogleRedirectLogin consumes the verified-claims handoff and returns
// the same login/2FA response contract as popup mode.
func (h *UserGoogleHandler) ExchangeGoogleRedirectLogin(c *gin.Context) {
	setGoogleRedirectNoStoreHeaders(c)
	handle, ok := takeGoogleRedirectHandoffCookie(c)
	if !ok {
		respondGoogleRedirectAPIError(c, ErrGoogleRedirectSessionExpired)
		return
	}
	tenant, _ := googleRedirectTenantFromContext(c)
	result, err := h.service.ExchangeGoogleRedirectLogin(c.Request.Context(), handle, tenant)
	if err != nil {
		if isGoogleRedirectStateError(err) {
			respondGoogleRedirectAPIError(c, err)
			return
		}
		h.respondGoogleLoginError(c, err)
		return
	}
	h.respondGoogleLoginResult(c, result)
}

// ExchangeGoogleRedirectBind consumes a handoff bound to both the current
// tenant and authenticated user.
func (h *UserGoogleHandler) ExchangeGoogleRedirectBind(c *gin.Context) {
	setGoogleRedirectNoStoreHeaders(c)
	handle, ok := takeGoogleRedirectHandoffCookie(c)
	if !ok {
		respondGoogleRedirectAPIError(c, ErrGoogleRedirectSessionExpired)
		return
	}
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	tenant, _ := googleRedirectTenantFromContext(c)
	binding, err := h.service.ExchangeGoogleRedirectBind(c.Request.Context(), handle, userID, tenant)
	if err != nil {
		if isGoogleRedirectStateError(err) {
			respondGoogleRedirectAPIError(c, err)
			return
		}
		respondGoogleBindError(c, err)
		return
	}
	if binding == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", errors.New("google binding result is nil"))
		return
	}
	response.Success(c, userpresenter.NewGoogleBindingResp(
		binding.Identity,
		binding.Email,
		binding.DisplayName,
		binding.CanUnbind,
	))
}

type googleRedirectCallbackForm struct {
	Credential      string
	State           string
	ValidCredential bool
	ValidState      bool
	ValidGISCSRF    bool
}

func parseGoogleRedirectCallback(c *gin.Context) (googleRedirectCallbackForm, string) {
	var result googleRedirectCallbackForm
	if c == nil || c.Request == nil {
		return result, "invalid_request"
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/x-www-form-urlencoded" {
		return result, "invalid_request"
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxGoogleCredentialRequestBodyBytes)
	if err = c.Request.ParseForm(); err != nil {
		return result, "invalid_request"
	}

	result.Credential, result.ValidCredential = singlePostFormValue(c.Request, "credential")
	result.State, result.ValidState = singlePostFormValue(c.Request, "state")
	formCSRF, validFormCSRF := singlePostFormValue(c.Request, "g_csrf_token")
	cookieCSRF, validCookieCSRF := singleRequestCookie(c.Request, "g_csrf_token")
	result.ValidGISCSRF = validFormCSRF &&
		validCookieCSRF &&
		formCSRF != "" &&
		constantTimeStringEqual(formCSRF, cookieCSRF)
	result.ValidCredential = result.ValidCredential && strings.TrimSpace(result.Credential) != ""
	result.ValidState = result.ValidState && strings.TrimSpace(result.State) != ""
	return result, ""
}

func singlePostFormValue(request *http.Request, key string) (string, bool) {
	if request == nil {
		return "", false
	}
	values, ok := request.PostForm[key]
	if !ok || len(values) != 1 {
		return "", false
	}
	return values[0], true
}

func singleRequestCookie(request *http.Request, name string) (string, bool) {
	if request == nil {
		return "", false
	}
	cookies := request.CookiesNamed(name)
	if len(cookies) != 1 || cookies[0] == nil || cookies[0].Value == "" {
		return "", false
	}
	return cookies[0].Value, true
}

func constantTimeStringEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func validGoogleRedirectHandle(value string) bool {
	if len(value) != 43 {
		return false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	return err == nil &&
		len(decoded) == 32 &&
		base64.RawURLEncoding.EncodeToString(decoded) == value
}

func googleRedirectTenantFromContext(c *gin.Context) (GoogleRedirectTenant, bool) {
	if c == nil || c.Request == nil {
		return GoogleRedirectTenant{}, false
	}
	tenant, ok := resellercontract.TenantFromContext(c.Request.Context())
	if !ok || tenant.Unavailable {
		return GoogleRedirectTenant{}, false
	}
	host := resellercontract.NormalizeHost(tenant.Host)
	if host == "" {
		return GoogleRedirectTenant{}, false
	}
	if tenant.IsMain {
		if tenant.ResellerID != nil {
			return GoogleRedirectTenant{}, false
		}
		return GoogleRedirectTenant{Host: host, IsMain: true}, true
	}
	if tenant.ResellerID == nil || *tenant.ResellerID == 0 {
		return GoogleRedirectTenant{}, false
	}
	return GoogleRedirectTenant{
		Host:          host,
		HasResellerID: true,
		ResellerID:    *tenant.ResellerID,
	}, true
}

func setGoogleRedirectNoStoreHeaders(c *gin.Context) {
	if c == nil {
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.Header("Referrer-Policy", "no-referrer")
}

func setGoogleRedirectCookie(
	c *gin.Context,
	name string,
	value string,
	maxAge int,
	sameSite http.SameSite,
) {
	if c == nil {
		return
	}
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: sameSite,
	}
	if maxAge > 0 {
		cookie.Expires = time.Now().Add(time.Duration(maxAge) * time.Second).UTC()
	} else {
		cookie.Expires = time.Unix(1, 0).UTC()
	}
	http.SetCookie(c.Writer, cookie)
}

func clearGoogleRedirectCookie(c *gin.Context, name string, sameSite http.SameSite) {
	setGoogleRedirectCookie(c, name, "", -1, sameSite)
}

func clearGoogleRedirectIntentCookies(c *gin.Context) {
	clearGoogleRedirectCookie(c, googleRedirectIntentCookieName, http.SameSiteNoneMode)
}

func takeGoogleRedirectHandoffCookie(c *gin.Context) (string, bool) {
	handle, ok := singleRequestCookie(c.Request, googleRedirectHandoffCookieName)
	// Clear before tenant/user validation or the application's atomic GETDEL.
	clearGoogleRedirectCookie(c, googleRedirectHandoffCookieName, http.SameSiteLaxMode)
	return handle, ok && validGoogleRedirectHandle(handle)
}

func redirectGoogleCallback(c *gin.Context, flow, errorCode string) {
	if flow != googleRedirectFlowBind {
		flow = googleRedirectFlowLogin
	}
	location := "/auth/google/callback?flow=" + flow
	if errorCode != "" {
		switch errorCode {
		case "invalid_request",
			"csrf_mismatch",
			"session_expired",
			"tenant_mismatch",
			"auth_disabled",
			"configuration_error",
			"credential_invalid",
			"credential_expired",
			"email_unverified",
			"service_unavailable",
			"internal_error":
		default:
			errorCode = "internal_error"
		}
		location += "&error=" + errorCode
	}
	c.Redirect(http.StatusSeeOther, location)
}

func googleRedirectCallbackError(err error) string {
	switch {
	case errors.Is(err, ErrGoogleRedirectSessionExpired):
		return "session_expired"
	case errors.Is(err, ErrGoogleRedirectTenantMismatch):
		return "tenant_mismatch"
	case errors.Is(err, ErrGoogleAuthDisabled):
		return "auth_disabled"
	case errors.Is(err, ErrGoogleAuthConfigInvalid):
		return "configuration_error"
	case errors.Is(err, ErrGoogleCredentialExpired):
		return "credential_expired"
	case errors.Is(err, ErrGoogleEmailUnverified):
		return "email_unverified"
	case errors.Is(err, ErrGoogleCredentialInvalid):
		return "credential_invalid"
	case errors.Is(err, ErrGoogleRedirectUnavailable), errors.Is(err, ErrGoogleJWKSUnavailable):
		return "service_unavailable"
	case errors.Is(err, ErrGoogleRedirectFlowInvalid):
		return "invalid_request"
	default:
		return "internal_error"
	}
}

func googleRedirectVerificationFailReason(err error) (string, bool) {
	switch {
	case errors.Is(err, ErrGoogleAuthDisabled),
		errors.Is(err, ErrGoogleAuthConfigInvalid),
		errors.Is(err, ErrGoogleJWKSUnavailable):
		return constants.LoginLogFailReasonGoogleConfig, true
	case errors.Is(err, ErrGoogleCredentialInvalid),
		errors.Is(err, ErrGoogleCredentialExpired),
		errors.Is(err, ErrGoogleEmailUnverified):
		return constants.LoginLogFailReasonGoogleInvalid, true
	default:
		return "", false
	}
}

func respondGoogleRedirectAPIError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrGoogleRedirectUnavailable):
		ginutil.RequestLog(c).Errorw("google_redirect_store_unavailable", "error", err)
		response.ErrorWithHTTPStatus(
			c,
			http.StatusServiceUnavailable,
			response.CodeInternal,
			i18n.T(i18n.ResolveLocale(c), "error.google_service_unavailable"),
		)
	case errors.Is(err, ErrGoogleRedirectSessionExpired),
		errors.Is(err, ErrGoogleRedirectFlowInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_redirect_session_expired", nil)
	case errors.Is(err, ErrGoogleRedirectTenantMismatch),
		errors.Is(err, ErrGoogleRedirectUserMismatch):
		ginutil.RespondError(c, response.CodeForbidden, "error.google_redirect_context_mismatch", nil)
	case errors.Is(err, ErrGoogleAuthDisabled):
		ginutil.RespondError(c, response.CodeBadRequest, "error.google_auth_disabled", nil)
	case errors.Is(err, ErrGoogleAuthConfigInvalid):
		ginutil.RespondError(c, response.CodeInternal, "error.google_auth_config_invalid", err)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.google_service_unavailable", err)
	}
}

func isGoogleRedirectStateError(err error) bool {
	return errors.Is(err, ErrGoogleRedirectUnavailable) ||
		errors.Is(err, ErrGoogleRedirectSessionExpired) ||
		errors.Is(err, ErrGoogleRedirectTenantMismatch) ||
		errors.Is(err, ErrGoogleRedirectUserMismatch) ||
		errors.Is(err, ErrGoogleRedirectFlowInvalid)
}

func (h *UserGoogleHandler) GetMyGoogleBinding(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	binding, err := h.service.GetGoogleBinding(userID)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", err)
		return
	}
	if binding == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_fetch_failed", errors.New("google binding result is nil"))
		return
	}
	response.Success(c, userpresenter.NewGoogleBindingResp(
		binding.Identity,
		binding.Email,
		binding.DisplayName,
		binding.CanUnbind,
	))
}

func (h *UserGoogleHandler) BindMyGoogle(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var request UserGoogleCredentialRequest
	if tooLarge, err := bindGoogleCredentialRequest(c, &request); err != nil {
		respondGoogleCredentialRequestError(c, tooLarge, err)
		return
	}
	binding, err := h.service.BindGoogle(c.Request.Context(), userID, request.Credential)
	if err != nil {
		respondGoogleBindError(c, err)
		return
	}
	if binding == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", errors.New("google binding result is nil"))
		return
	}
	response.Success(c, userpresenter.NewGoogleBindingResp(
		binding.Identity,
		binding.Email,
		binding.DisplayName,
		binding.CanUnbind,
	))
}

func (h *UserGoogleHandler) UnbindMyGoogle(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	if err := h.service.UnbindGoogle(userID); err != nil {
		switch {
		case errors.Is(err, ErrUserOAuthNotBound):
			ginutil.RespondError(c, response.CodeBadRequest, "error.google_not_bound", nil)
		case errors.Is(err, ErrGoogleUnbindLocked):
			ginutil.RespondError(c, response.CodeBadRequest, "error.google_unbind_locked", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.user_update_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"unbound": true})
}
