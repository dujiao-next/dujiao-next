package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/config"
	auditapp "github.com/dujiao-next/internal/modules/auditlog/application"
	auditdomain "github.com/dujiao-next/internal/modules/auditlog/domain"
	auditstore "github.com/dujiao-next/internal/modules/auditlog/infrastructure/gormstore"
	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	tokenapp "github.com/dujiao-next/internal/modules/identity/adminapitoken/application"
	tokendomain "github.com/dujiao-next/internal/modules/identity/adminapitoken/domain"
	tokenstore "github.com/dujiao-next/internal/modules/identity/adminapitoken/infrastructure/gormstore"
	tokenhttp "github.com/dujiao-next/internal/modules/identity/adminapitoken/transport/http"
	adminauthapp "github.com/dujiao-next/internal/modules/identity/adminauth/application"
	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type tokenFixture struct {
	db         *gorm.DB
	service    *tokenapp.Service
	authz      *authz.Service
	admin      *admindomain.Admin
	audit      *auditapp.AuthzService
	admins     *adminstore.Store
	compliance *complianceapp.Service
	events     []tokenhttp.AuditEvent
}

func setupTokenMiddleware(t *testing.T) *tokenFixture {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&admindomain.Admin{}, &tokendomain.Token{}, &auditdomain.AuthzAuditLog{}, &settingsstore.SettingRecord{}))
	admins := adminstore.New(db)
	admin := &admindomain.Admin{Username: "machine", PasswordHash: "hash"}
	require.NoError(t, admins.Create(admin))
	rbac, err := authz.NewService(db)
	require.NoError(t, err)
	require.NoError(t, rbac.GrantRolePolicy("automation", "/admin/*", "*"))
	require.NoError(t, rbac.SetAdminRoles(admin.ID, []string{"automation"}))
	return &tokenFixture{
		db: db, service: tokenapp.NewService(tokenstore.New(db), admins),
		authz: rbac, admin: admin, admins: admins,
		audit:      auditapp.NewAuthzService(auditstore.NewAuthzStore(db)),
		compliance: complianceapp.NewService(settingsstore.New(db)),
	}
}

func (f *tokenFixture) mint(t *testing.T) (*tokendomain.Token, string) {
	t.Helper()
	token, secret, err := f.service.Create(f.admin.ID, "task", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	return token, secret
}

func (f *tokenFixture) router(maxRequests int) *gin.Engine {
	r := gin.New()
	r.Use(
		AdminAPITokenRateLimit(nil, config.AdminAPIRateLimitConfig{WindowSeconds: 60, MaxRequests: 1000}, false),
		JWTAuthMiddleware("test", f.admins, AdminAPITokenAuthentication(f.service, f.audit)),
		AdminAPITokenRateLimit(nil, config.AdminAPIRateLimitConfig{WindowSeconds: 60, MaxRequests: maxRequests}, true),
		AdminRBACMiddleware(f.authz),
		AdminAPITokenAccess(f.compliance),
	)
	pass := func(c *gin.Context) { response.Success(c, gin.H{"reached": true}) }
	for _, path := range []string{
		"/products", "/products/export-secrets", "/orders/:id", "/authz/roles",
		"/settings", "/api-credentials", "/channel-clients", "/2fa/status",
	} {
		r.GET("/api/v1/admin"+path, pass)
	}
	for _, path := range []string{"/orders/:id/refund-to-wallet", "/orders/:id/manual-refund", "/orders/:id/manual-confirm-payment", "/users/:id/wallet/adjust", "/system/update/start", "/2fa/disable"} {
		r.POST("/api/v1/admin"+path, pass)
	}
	r.PUT("/api/v1/admin/password", pass)
	r.GET("/api/v1/admin/payments", func(c *gin.Context) { response.Error(c, response.CodeBadRequest, "business_failure") })
	tokenhttp.RegisterAdminRoutes(r.Group("/api/v1/admin"), tokenhttp.NewHandler(f.service, func(event tokenhttp.AuditEvent) {
		f.events = append(f.events, event)
	}))
	return r
}

func tokenRequest(r http.Handler, method, path, secret string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAPITokenUsesFreshPermissionsAndAuditsResults(t *testing.T) {
	f := setupTokenMiddleware(t)
	token, secret := f.mint(t)
	r := f.router(100)
	rec := tokenRequest(r, "GET", "/api/v1/admin/products", secret)
	require.Contains(t, rec.Body.String(), `"reached":true`)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))

	// A second enforcer simulates another application instance changing RBAC.
	other, err := authz.NewService(f.db)
	require.NoError(t, err)
	require.NoError(t, other.SetAdminRoles(f.admin.ID, nil))
	rec = tokenRequest(r, "GET", "/api/v1/admin/products", secret)
	require.Contains(t, rec.Body.String(), `"status_code":403`)
	require.NotContains(t, rec.Body.String(), `"reached":true`)
	require.NoError(t, other.SetAdminRoles(f.admin.ID, []string{"automation"}))
	require.NoError(t, other.RevokeRolePolicy("automation", "/admin/*", "*"))
	rec = tokenRequest(r, "GET", "/api/v1/admin/products", secret)
	require.Contains(t, rec.Body.String(), `"status_code":403`)
	require.NoError(t, other.GrantRolePolicy("automation", "/admin/*", "*"))
	rec = tokenRequest(r, "GET", "/api/v1/admin/payments", secret)
	require.Contains(t, rec.Body.String(), `"status_code":400`)
	var logs []auditdomain.AuthzAuditLog
	require.NoError(t, f.db.Order("id").Find(&logs).Error)
	require.Len(t, logs, 4)
	require.EqualValues(t, token.ID, logs[0].DetailJSON["token_id"])
	require.EqualValues(t, 403, logs[1].DetailJSON["status_code"])
	require.EqualValues(t, 400, logs[3].DetailJSON["status_code"])
	encoded, err := json.Marshal(logs)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), secret)
}

func TestAPITokenCannotAccessSensitiveOrUnlistedRoutes(t *testing.T) {
	f := setupTokenMiddleware(t)
	_, secret := f.mint(t)
	r := f.router(100)
	for _, endpoint := range []string{
		"GET /api/v1/admin/authz/roles", "GET /api/v1/admin/settings",
		"GET /api/v1/admin/api-credentials", "GET /api/v1/admin/channel-clients",
		"GET /api/v1/admin/2fa/status", "POST /api/v1/admin/2fa/disable",
		"PUT /api/v1/admin/password", "POST /api/v1/admin/system/update/start",
		"GET /api/v1/admin/products/export-secrets",
		"GET /api/v1/admin/authz/admins/1/api-tokens",
		"POST /api/v1/admin/authz/admins/1/api-tokens",
		"DELETE /api/v1/admin/authz/admins/1/api-tokens/1",
		"POST /api/v1/admin/authz/admins/1/api-tokens/1/rotate",
	} {
		parts := strings.SplitN(endpoint, " ", 2)
		rec := tokenRequest(r, parts[0], parts[1], secret)
		require.Equal(t, http.StatusForbidden, rec.Code, endpoint)
		require.NotContains(t, rec.Body.String(), `"reached":true`, endpoint)
	}
}

func TestAPITokenFinancialWritesRequireComplianceAndCurrentRBAC(t *testing.T) {
	f := setupTokenMiddleware(t)
	_, secret := f.mint(t)
	r := f.router(100)
	paths := []string{"/orders/1/refund-to-wallet", "/orders/1/manual-refund", "/orders/1/manual-confirm-payment", "/users/1/wallet/adjust"}
	for _, path := range paths {
		rec := tokenRequest(r, "POST", "/api/v1/admin"+path, secret)
		require.Contains(t, rec.Body.String(), "compliance_required_by_super_admin")
		require.NotContains(t, rec.Body.String(), `"reached":true`)
	}
	require.NoError(t, f.compliance.Acknowledge(complianceapp.AcknowledgeCommand{
		Segment1: "我已阅读并理解上述合规声明提醒",
		Segment2: "知悉相关法律风险",
		Segment3: "并确认自行承担部署运营和收费行为产生的法律责任",
		AdminID:  1, Username: "admin",
	}))
	for _, path := range paths {
		rec := tokenRequest(r, "POST", "/api/v1/admin"+path, secret)
		require.Contains(t, rec.Body.String(), `"reached":true`)
	}
	require.NoError(t, f.authz.SetAdminRoles(f.admin.ID, nil))
	for _, path := range paths {
		rec := tokenRequest(r, "POST", "/api/v1/admin"+path, secret)
		require.Contains(t, rec.Body.String(), `"status_code":403`)
		require.NotContains(t, rec.Body.String(), `"reached":true`)
	}
}

func TestAPITokenRevocationAndLimitArePerToken(t *testing.T) {
	f := setupTokenMiddleware(t)
	token, first := f.mint(t)
	_, second := f.mint(t)
	r := f.router(1)
	require.Contains(t, tokenRequest(r, "GET", "/api/v1/admin/products", first).Body.String(), `"reached":true`)
	require.Equal(t, http.StatusTooManyRequests, tokenRequest(r, "GET", "/api/v1/admin/products", first).Code)
	require.Contains(t, tokenRequest(r, "GET", "/api/v1/admin/products", second).Body.String(), `"reached":true`)
	require.NoError(t, f.service.Revoke(f.admin.ID, token.ID))
	require.Equal(t, http.StatusUnauthorized, tokenRequest(r, "GET", "/api/v1/admin/products", first).Code)
	require.Equal(t, http.StatusUnauthorized, tokenRequest(r, "GET", "/api/v1/admin/products", tokenapp.Prefix+"bad").Code)
	require.NoError(t, f.admins.UpdatePassword(f.admin.ID, "changed"))
	require.Equal(t, http.StatusUnauthorized, tokenRequest(r, "GET", "/api/v1/admin/products", second).Code)
}

func TestAPITokenManagementRequiresInteractivePermissions(t *testing.T) {
	f := setupTokenMiddleware(t)
	require.NoError(t, f.authz.BootstrapBuiltinRoles())
	manager := &admindomain.Admin{Username: "manager", PasswordHash: "hash"}
	require.NoError(t, f.admins.Create(manager))
	require.NoError(t, f.authz.SetAdminRoles(manager.ID, []string{"system_admin"}))
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, adminauthapp.JWTClaims{
		AdminID: manager.ID, Username: manager.Username, Typ: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}).SignedString([]byte("test"))
	require.NoError(t, err)
	r := f.router(100)
	path := fmt.Sprintf("/api/v1/admin/authz/admins/%d/api-tokens", f.admin.ID)
	payload := fmt.Sprintf(`{"name":"sync","expires_at":%q,"ip_whitelist":["192.0.2.42/24"]}`, time.Now().Add(time.Hour).Format(time.RFC3339))
	create := func(target string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", target, strings.NewReader(payload))
		req.Header.Set("Authorization", "Bearer "+signed)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	rec := create(path)
	require.Contains(t, rec.Body.String(), `"status_code":0`)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var created struct {
		Data struct {
			Token  tokendomain.Token `json:"token"`
			Secret string            `json:"secret"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.NotEmpty(t, created.Data.Secret)
	require.NotContains(t, rec.Body.String(), "allow_financial_writes")
	require.Equal(t, []string{"192.0.2.0/24"}, created.Data.Token.IPWhitelist)
	list := tokenRequest(r, "GET", path, signed)
	require.Contains(t, list.Body.String(), `"name":"sync"`)
	require.NotContains(t, list.Body.String(), created.Data.Secret)
	require.NotContains(t, list.Body.String(), "digest")
	require.Contains(t, list.Body.String(), `"ip_whitelist":["192.0.2.0/24"]`)
	rotatePath := fmt.Sprintf("%s/%d/rotate", path, created.Data.Token.ID)
	rotated := tokenRequest(r, "POST", rotatePath, signed)
	require.Contains(t, rotated.Body.String(), `"status_code":0`)
	require.Equal(t, "no-store", rotated.Header().Get("Cache-Control"))
	var rotation struct {
		Data struct {
			Token  tokendomain.Token `json:"token"`
			Secret string            `json:"secret"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rotated.Body.Bytes(), &rotation))
	require.NotEmpty(t, rotation.Data.Secret)
	require.NotEqual(t, created.Data.Secret, rotation.Data.Secret)
	require.Equal(t, created.Data.Token.ID, rotation.Data.Token.ID)
	require.Equal(t, created.Data.Token.IPWhitelist, rotation.Data.Token.IPWhitelist)
	require.Equal(t, "admin_api_token_rotate", f.events[len(f.events)-1].Action)
	require.Equal(t, manager.ID, f.events[len(f.events)-1].OperatorID)
	require.Equal(t, created.Data.Token.ID, f.events[len(f.events)-1].TokenID)
	require.Equal(t, http.StatusUnauthorized, tokenRequest(r, "GET", "/api/v1/admin/products", created.Data.Secret).Code)
	_, _, err = f.service.Authenticate(rotation.Data.Secret)
	require.NoError(t, err)
	require.NotContains(t, tokenRequest(r, "GET", path, signed).Body.String(), rotation.Data.Secret)
	super := &admindomain.Admin{Username: "root", PasswordHash: "hash", IsSuper: true}
	require.NoError(t, f.admins.Create(super))
	rec = create(fmt.Sprintf("/api/v1/admin/authz/admins/%d/api-tokens", super.ID))
	require.Contains(t, rec.Body.String(), `"status_code":400`)
	require.NotContains(t, rec.Body.String(), `"secret"`)
	// Removing management privileges blocks issuance through an existing JWT.
	require.NoError(t, f.authz.SetAdminRoles(manager.ID, []string{"readonly_auditor"}))
	rec = tokenRequest(r, "POST", rotatePath, signed)
	require.Contains(t, rec.Body.String(), `"status_code":403`)
	require.NotContains(t, rec.Body.String(), `"secret"`)
	rec = create(path)
	require.Contains(t, rec.Body.String(), `"status_code":403`)
	require.NotContains(t, rec.Body.String(), `"secret"`)
	require.NoError(t, f.authz.SetAdminRoles(manager.ID, []string{"system_admin"}))
	rec = tokenRequest(r, "DELETE", fmt.Sprintf("%s/%d", path, created.Data.Token.ID), signed)
	require.Contains(t, rec.Body.String(), `"status_code":0`)
	require.Equal(t, http.StatusUnauthorized, tokenRequest(r, "GET", "/api/v1/admin/products", created.Data.Secret).Code)
	require.Equal(t, http.StatusUnauthorized, tokenRequest(r, "GET", "/api/v1/admin/products", rotation.Data.Secret).Code)
}

func TestAPITokenIPWhitelistRespectsTrustedProxiesAndAuditsDenial(t *testing.T) {
	f := setupTokenMiddleware(t)
	token, secret, err := f.service.Create(f.admin.ID, "restricted", time.Now().Add(time.Hour),
		[]string{"198.51.100.0/24", "2001:db8::/32"})
	require.NoError(t, err)
	r := f.router(100)
	require.NoError(t, r.SetTrustedProxies([]string{"127.0.0.1/32"}))
	for _, tc := range []struct {
		name, peer, forwarded string
		allowed               bool
	}{
		{"untrusted forged headers", "203.0.113.8:1234", "198.51.100.10", false},
		{"direct IPv4", "198.51.100.10:1234", "", true},
		{"outside range", "198.51.101.10:1234", "", false},
		{"direct IPv6", "[2001:db8::42]:1234", "", true},
		{"mapped IPv4", "[::ffff:198.51.100.10]:1234", "", true},
		{"trusted proxy", "127.0.0.1:1234", "198.51.100.10", true},
		{"untrusted hop in chain", "127.0.0.1:1234", "198.51.100.10, 203.0.113.8", false},
		{"missing source", "", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/admin/products", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("Authorization", "Bearer "+secret)
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			req.Header.Set("X-Real-IP", tc.forwarded)
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if tc.allowed {
				require.Contains(t, rec.Body.String(), `"reached":true`)
			} else {
				require.Equal(t, http.StatusForbidden, rec.Code)
				require.NotContains(t, rec.Body.String(), `"reached":true`)
			}
			if tc.name == "untrusted forged headers" {
				var stored tokendomain.Token
				require.NoError(t, f.db.First(&stored, token.ID).Error)
				require.Nil(t, stored.LastUsedAt)
				var log auditdomain.AuthzAuditLog
				require.NoError(t, f.db.Last(&log).Error)
				require.Equal(t, "203.0.113.8", log.DetailJSON["client_ip"])
				require.Equal(t, "ip_not_allowed", log.DetailJSON["denial_reason"])
				require.EqualValues(t, http.StatusForbidden, log.DetailJSON["http_status"])
			}
		})
	}
}
