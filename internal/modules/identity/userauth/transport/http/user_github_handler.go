package userauthhttp

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/constants"
	githubauthapp "github.com/dujiao-next/internal/modules/identity/githubauth/application"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	userpresenter "github.com/dujiao-next/internal/modules/identity/userauth/transport/presenter"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

type UserGitHubHandler struct {
	oauth    *githubauthapp.Service
	users    *userauthapp.Service
	recorder LoginRecorder
}

func NewUserGitHubHandler(oauth *githubauthapp.Service, users *userauthapp.Service, recorder LoginRecorder) *UserGitHubHandler {
	if oauth == nil || users == nil {
		panic("user github handler dependencies nil")
	}
	return &UserGitHubHandler{oauth: oauth, users: users, recorder: recorder}
}

func (h *UserGitHubHandler) Start(c *gin.Context) {
	origin, tenant, ok := trustedGitHubRequestContext(c)
	if !ok {
		response.ErrorWithHTTPStatus(c, http.StatusBadRequest, response.CodeBadRequest, "invalid site origin")
		return
	}
	state, err := h.users.CreateGitHubOAuthIntent(c.Request.Context(), userauthapp.GitHubOAuthIntent{Origin: origin, Tenant: tenant})
	if err != nil {
		response.ErrorWithHTTPStatus(c, http.StatusServiceUnavailable, response.CodeInternal, "GitHub login unavailable")
		return
	}
	callback, _ := userauthapp.BuildGitHubCallbackURL(origin)
	location, err := h.oauth.AuthorizationURL(state, callback)
	if err != nil {
		response.ErrorWithHTTPStatus(c, http.StatusBadRequest, response.CodeBadRequest, "GitHub login disabled")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusFound, location)
}
func (h *UserGitHubHandler) Callback(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	state := strings.TrimSpace(c.Query("state"))
	code := strings.TrimSpace(c.Query("code"))
	if state == "" || code == "" {
		h.popup(c, "", nil, "invalid_request")
		return
	}
	intent, err := h.users.ConsumeGitHubOAuthIntent(c.Request.Context(), state)
	if err != nil {
		h.popup(c, "", nil, "session_expired")
		return
	}
	origin, tenant, ok := trustedGitHubRequestContext(c)
	if !ok || origin != intent.Origin || tenant != intent.Tenant {
		h.popup(c, intent.Origin, nil, "context_mismatch")
		return
	}
	callback, _ := userauthapp.BuildGitHubCallbackURL(intent.Origin)
	identity, err := h.oauth.ExchangeCode(c.Request.Context(), code, callback)
	if err != nil {
		h.record(c, "", 0, constants.LoginLogStatusFailed, "github_invalid")
		h.popup(c, intent.Origin, nil, "github_failed")
		return
	}
	result, err := h.users.LoginVerifiedGitHub(identity)
	if err != nil {
		h.record(c, identity.Email, 0, constants.LoginLogStatusFailed, "github_login_failed")
		h.popup(c, intent.Origin, nil, "login_failed")
		return
	}
	h.record(c, result.User.Email, result.User.ID, constants.LoginLogStatusSuccess, "")
	h.popup(c, intent.Origin, result, "")
}
func (h *UserGitHubHandler) record(c *gin.Context, email string, userID uint, status, reason string) {
	if h.recorder == nil {
		return
	}
	h.recorder.Record(email, userID, status, reason, constants.LoginLogSourceGitHub, c.ClientIP(), c.GetHeader("User-Agent"), "")
}
func trustedGitHubRequestContext(c *gin.Context) (string, userauthapp.GitHubRedirectTenant, bool) {
	t, ok := resellercontract.TenantFromContext(c.Request.Context())
	if !ok || t.Unavailable {
		return "", userauthapp.GitHubRedirectTenant{}, false
	}
	host := resellercontract.NormalizeHost(t.Host)
	if host == "" {
		return "", userauthapp.GitHubRedirectTenant{}, false
	}
	scheme := "https"
	if c.Request.TLS == nil && (strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1")) {
		scheme = "http"
	}
	tenant := userauthapp.GitHubRedirectTenant{Host: host, IsMain: t.IsMain}
	if t.ResellerID != nil {
		tenant.HasResellerID = true
		tenant.ResellerID = *t.ResellerID
	}
	return scheme + "://" + host, tenant, true
}

var gitHubPopupTemplate = template.Must(template.New("github").Parse(`<!doctype html><meta charset="utf-8"><script>const data={{.Payload}};if(window.opener)window.opener.postMessage({type:"dujiao:github-oauth",data},{{.Origin}});window.close()</script>`))

func (h *UserGitHubHandler) popup(c *gin.Context, origin string, result *userauthapp.UserLoginResult, code string) {
	payload := map[string]interface{}{"error": code}
	if result != nil {
		payload = map[string]interface{}{"requires_totp": result.RequiresTOTP, "token": result.Token, "expires_at": result.ExpiresAt, "challenge_token": result.ChallengeToken, "challenge_expires_at": result.ChallengeExpiresAt, "user": userpresenter.NewUserAuthBriefResp(result.User)}
	}
	raw, _ := json.Marshal(payload)
	originJSON, _ := json.Marshal(origin)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = gitHubPopupTemplate.Execute(c.Writer, map[string]template.JS{"Payload": template.JS(raw), "Origin": template.JS(originJSON)})
}
func RegisterUserGitHubAuthRoutes(auth gin.IRoutes, h *UserGitHubHandler, limit gin.HandlerFunc) {
	auth.GET("/github", limit, h.Start)
	auth.GET("/github/callback", limit, h.Callback)
}
