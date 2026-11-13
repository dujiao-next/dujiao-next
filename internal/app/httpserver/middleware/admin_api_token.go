package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	auditlogapp "github.com/dujiao-next/internal/modules/auditlog/application"
	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	tokenapp "github.com/dujiao-next/internal/modules/identity/adminapitoken/application"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func isAdminAPITokenRequest(c *gin.Context) bool {
	return strings.HasPrefix(c.GetHeader(authHeaderKey), authSchemeBearer+" "+tokenapp.Prefix)
}

// AdminAPITokenAuthentication is explicitly enabled by the admin route group.
// API tokens cannot reach the super-admin fast path or fall back to JWT.
func AdminAPITokenAuthentication(tokens *tokenapp.Service, audit *auditlogapp.AuthzService) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		if tokens == nil {
			response.ErrorWithHTTPStatus(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.token_invalid"))
			return
		}
		token, admin, err := tokens.Authenticate(strings.TrimPrefix(c.GetHeader(authHeaderKey), authSchemeBearer+" "))
		if err != nil {
			response.ErrorWithHTTPStatus(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.token_invalid"))
			return
		}
		c.Set("admin_id", admin.ID)
		c.Set("username", admin.Username)
		c.Set(adminIsSuperContextKey, false)
		c.Set("admin_api_token_id", token.ID)

		clientIP := c.ClientIP()
		completed := false
		defer func() {
			status := c.Writer.Status()
			if !completed {
				status = http.StatusInternalServerError
			}
			record := auditlogapp.AuthzRecord{
				OperatorAdminID: admin.ID, OperatorUsername: admin.Username,
				TargetAdminID: &admin.ID, Action: "admin_api_token_request",
				Object: c.FullPath(), Method: c.Request.Method, RequestID: c.GetString(requestIDKey),
				Detail: jsonmap.JSON{
					"token_id": token.ID, "http_status": status,
					"status_code":   c.GetInt("response_status_code"),
					"client_ip":     clientIP,
					"denial_reason": c.GetString("admin_api_token_denial_reason"),
				},
			}
			if err := audit.Record(record); err != nil {
				logger.Warnw("admin_api_token_audit_failed", "token_id", token.ID, "error", err)
			}
		}()
		if !tokenapp.AllowsIP(token.IPWhitelist, clientIP) {
			c.Set("admin_api_token_denial_reason", "ip_not_allowed")
			response.ErrorWithHTTPStatus(c, http.StatusForbidden, response.CodeForbidden, i18n.T(i18n.ResolveLocale(c), "error.admin_api_token_ip_forbidden"))
			completed = true
			return
		}
		if err := tokens.Touch(token.ID); err != nil {
			logger.Warnw("admin_api_token_touch_failed", "token_id", token.ID, "error", err)
		}
		c.Next()
		completed = true
	}
}

// Pre-auth uses IP only (never an untrusted credential); post-auth uses a
// verified token ID shared across source IPs. Both reuse the bounded fallback.
func AdminAPITokenRateLimit(client *redis.Client, cfg config.AdminAPIRateLimitConfig, authenticated bool) gin.HandlerFunc {
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 60
	}
	if cfg.MaxRequests <= 0 {
		cfg.MaxRequests = 120
	}
	rule := RateLimitRule{Prefix: "admin_api_token_ip", WindowSeconds: cfg.WindowSeconds, MaxRequests: cfg.MaxRequests}
	key := KeyByIP
	if authenticated {
		rule.Prefix = "admin_api_token"
		key = func(c *gin.Context) string { return fmt.Sprint(c.GetUint("admin_api_token_id")) }
	}
	limit := RateLimitMiddleware(client, rule, key)
	return func(c *gin.Context) {
		if (authenticated && c.GetUint("admin_api_token_id") == 0) || (!authenticated && !isAdminAPITokenRequest(c)) {
			c.Next()
			return
		}
		limit(c)
	}
}

func AdminAPITokenAccess(compliance *complianceapp.Service) gin.HandlerFunc {
	paymentGuard := PaymentComplianceRequired(compliance)
	return func(c *gin.Context) {
		if c.GetUint("admin_api_token_id") == 0 {
			c.Next()
			return
		}
		allowed, financial := tokenapp.RouteAccess(c.Request.Method, c.FullPath())
		if !allowed {
			response.ErrorWithHTTPStatus(c, http.StatusForbidden, response.CodeForbidden, i18n.T(i18n.ResolveLocale(c), "error.admin_api_token_access"))
			return
		}
		if financial {
			paymentGuard(c)
			return
		}
		c.Next()
	}
}
