package router

import (
	"go.uber.org/zap"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/authz"
	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/repository"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const requestIDKey = "request_id"
const requestIDHeader = "X-Request-ID"
const adminIsSuperContextKey = "admin_is_super"
const authHeaderKey = "Authorization"
const authSchemeBearer = "Bearer"

// CORSMiddleware 跨域中间件
func CORSMiddleware(cfg config.CORSConfig) gin.HandlerFunc {
	allowedOrigins := cfg.AllowedOrigins
	if len(allowedOrigins) == 0 {
		allowedOrigins = config.DefaultCORSAllowedOrigins()
	}
	allowedMethods := cfg.AllowedMethods
	if len(allowedMethods) == 0 {
		allowedMethods = config.DefaultCORSAllowedMethods()
	}
	allowedHeaders := cfg.AllowedHeaders
	if len(allowedHeaders) == 0 {
		allowedHeaders = config.DefaultCORSAllowedHeaders()
	}
	exposedHeaders := cfg.ExposedHeaders
	if len(exposedHeaders) == 0 {
		exposedHeaders = config.DefaultCORSExposedHeaders()
	}
	methodsHeader := strings.Join(allowedMethods, ", ")
	headersHeader := strings.Join(allowedHeaders, ", ")
	exposedHeadersHeader := strings.Join(exposedHeaders, ", ")

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		allowedOrigin := resolveAllowedOrigin(origin, allowedOrigins, cfg.AllowCredentials)
		if allowedOrigin != "" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			if allowedOrigin != "*" {
				c.Writer.Header().Add("Vary", "Origin")
			}
		}
		if cfg.AllowCredentials {
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		}
		c.Writer.Header().Set("Access-Control-Allow-Headers", headersHeader)
		c.Writer.Header().Set("Access-Control-Allow-Methods", methodsHeader)
		if exposedHeadersHeader != "" {
			c.Writer.Header().Set("Access-Control-Expose-Headers", exposedHeadersHeader)
		}
		if cfg.MaxAge > 0 {
			c.Writer.Header().Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func resolveAllowedOrigin(origin string, allowedOrigins []string, allowCredentials bool) string {
	if len(allowedOrigins) == 0 {
		return ""
	}
	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			if allowCredentials && origin != "" {
				return origin
			}
			return "*"
		}
	}
	if origin == "" {
		return ""
	}
	for _, allowed := range allowedOrigins {
		if strings.EqualFold(allowed, origin) {
			return origin
		}
	}
	return ""
}

// RequestIDMiddleware 请求 ID 中间件
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader(requestIDHeader))
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Set(requestIDKey, requestID)
		c.Writer.Header().Set(requestIDHeader, requestID)
		c.Next()
	}
}

// LoggerMiddleware 结构化请求日志中间件
func LoggerMiddleware(logger *zap.Logger) gin.HandlerFunc {
	if logger == nil {
		logger = zap.L()
	}
	sugar := logger.Sugar()
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		log := sugar.With(
			"request_id", getRequestID(c),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
		if len(c.Errors) > 0 {
			log.Errorw("request", "errors", c.Errors.String())
			return
		}
		log.Infow("request")
	}
}

func getRequestID(c *gin.Context) string {
	value, ok := c.Get(requestIDKey)
	if !ok {
		return ""
	}
	if requestID, ok := value.(string); ok {
		return requestID
	}
	return ""
}

// JWTAuthMiddleware JWT 鉴权中间件
func JWTAuthMiddleware(secretKey string, adminRepo repository.AdminRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secretKey == "" {
			msg := i18n.T(i18n.ResolveLocale(c), "error.jwt_secret_missing")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if adminRepo == nil {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		authHeader := c.GetHeader(authHeaderKey)
		if authHeader == "" {
			msg := i18n.T(i18n.ResolveLocale(c), "error.auth_header_missing")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == authSchemeBearer) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.auth_header_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		tokenString := parts[1]
		parser := newHS256JWTParser()
		claims := &service.JWTClaims{}
		token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})
		if err != nil || !token.Valid || claims.AdminID == 0 {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if !service.IsAccessTokenTyp(claims.Typ) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		if cached, hit, cacheErr := cache.GetAdminAuthState(c.Request.Context(), claims.AdminID); cacheErr == nil && hit && cached != nil {
			if claims.TokenVersion != cached.TokenVersion || !isIssuedAfterInvalidBeforeUnix(claims.IssuedAt, cached.TokenInvalidBefore) {
				msg := i18n.T(i18n.ResolveLocale(c), "error.token_revoked")
				response.Unauthorized(c, msg)
				c.Abort()
				return
			}
			c.Set("admin_id", claims.AdminID)
			c.Set("username", claims.Username)
			c.Set(adminIsSuperContextKey, cached.IsSuper)
			c.Next()
			return
		}

		admin, err := adminRepo.GetByID(claims.AdminID)
		if err != nil || admin == nil {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if claims.TokenVersion != admin.TokenVersion || !isIssuedAfterInvalidBefore(claims.IssuedAt, admin.TokenInvalidBefore) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_revoked")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		_ = cache.SetAdminAuthState(c.Request.Context(), cache.BuildAdminAuthState(admin))

		c.Set("admin_id", claims.AdminID)
		c.Set("username", claims.Username)
		c.Set(adminIsSuperContextKey, admin.IsSuper)
		c.Next()
	}
}

// AdminRBACMiddleware 管理端 RBAC 鉴权中间件
func AdminRBACMiddleware(authzService *authz.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authzService == nil {
			logger.Errorw("admin_rbac_service_unavailable")
			msg := i18n.T(i18n.ResolveLocale(c), "error.unauthorized")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		if isSuper, ok := c.Get(adminIsSuperContextKey); ok {
			if superValue, typeOK := isSuper.(bool); typeOK && superValue {
				c.Next()
				return
			}
		}

		adminIDRaw, exists := c.Get("admin_id")
		if !exists {
			msg := i18n.T(i18n.ResolveLocale(c), "error.unauthorized")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		var adminID uint
		switch value := adminIDRaw.(type) {
		case uint:
			adminID = value
		case int:
			if value > 0 {
				adminID = uint(value)
			}
		case float64:
			if value > 0 {
				adminID = uint(value)
			}
		}
		if adminID == 0 {
			msg := i18n.T(i18n.ResolveLocale(c), "error.unauthorized")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		resource := c.FullPath()
		if strings.TrimSpace(resource) == "" {
			resource = c.Request.URL.Path
		}

		allowed, err := authzService.EnforceAdmin(adminID, resource, c.Request.Method)
		if err != nil {
			logger.Errorw("admin_rbac_enforce_failed",
				"admin_id", adminID,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"error", err,
			)
			msg := i18n.T(i18n.ResolveLocale(c), "error.unauthorized")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if !allowed {
			logger.Warnw("admin_rbac_permission_denied",
				"admin_id", adminID,
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"resource", authz.NormalizeObject(resource),
			)
			msg := i18n.T(i18n.ResolveLocale(c), "error.forbidden")
			response.Forbidden(c, msg)
			c.Abort()
			return
		}

		c.Next()
	}
}

// UserJWTAuthMiddleware 用户 JWT 鉴权中间件
func UserJWTAuthMiddleware(secretKey string, userRepo repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if secretKey == "" {
			msg := i18n.T(i18n.ResolveLocale(c), "error.jwt_secret_missing")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if userRepo == nil {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		authHeader := c.GetHeader(authHeaderKey)
		if authHeader == "" {
			msg := i18n.T(i18n.ResolveLocale(c), "error.auth_header_missing")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if !(len(parts) == 2 && parts[0] == authSchemeBearer) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.auth_header_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		tokenString := parts[1]
		parser := newHS256JWTParser()
		claims := &service.UserJWTClaims{}
		token, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(secretKey), nil
		})
		if err != nil || !token.Valid || claims.UserID == 0 {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if !service.IsAccessTokenTyp(claims.Typ) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}

		if cached, hit, cacheErr := cache.GetUserAuthState(c.Request.Context(), claims.UserID); cacheErr == nil && hit && cached != nil {
			if !isActiveUserStatus(cached.Status) {
				msg := i18n.T(i18n.ResolveLocale(c), "error.user_disabled")
				response.Unauthorized(c, msg)
				c.Abort()
				return
			}
			if claims.TokenVersion != cached.TokenVersion || !isIssuedAfterInvalidBeforeUnix(claims.IssuedAt, cached.TokenInvalidBefore) {
				msg := i18n.T(i18n.ResolveLocale(c), "error.token_revoked")
				response.Unauthorized(c, msg)
				c.Abort()
				return
			}
			c.Set("user_id", claims.UserID)
			c.Set("user_email", claims.Email)
			c.Next()
			return
		}

		user, err := userRepo.GetByID(claims.UserID)
		if err != nil || user == nil {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_invalid")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if !isActiveUserStatus(user.Status) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.user_disabled")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		if claims.TokenVersion != user.TokenVersion || !isIssuedAfterInvalidBefore(claims.IssuedAt, user.TokenInvalidBefore) {
			msg := i18n.T(i18n.ResolveLocale(c), "error.token_revoked")
			response.Unauthorized(c, msg)
			c.Abort()
			return
		}
		_ = cache.SetUserAuthState(c.Request.Context(), cache.BuildUserAuthState(user))

		c.Set("user_id", claims.UserID)
		c.Set("user_email", claims.Email)
		c.Next()
	}
}

func isIssuedAfterInvalidBefore(issuedAt *jwt.NumericDate, invalidBefore *time.Time) bool {
	if invalidBefore == nil {
		return true
	}
	if issuedAt == nil {
		return false
	}
	return issuedAt.Time.Unix() >= invalidBefore.Unix()
}

func isIssuedAfterInvalidBeforeUnix(issuedAt *jwt.NumericDate, invalidBeforeUnix int64) bool {
	if invalidBeforeUnix <= 0 {
		return true
	}
	if issuedAt == nil {
		return false
	}
	return issuedAt.Time.Unix() >= invalidBeforeUnix
}

func newHS256JWTParser() *jwt.Parser {
	return jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
}

func isActiveUserStatus(status string) bool {
	return strings.ToLower(strings.TrimSpace(status)) == constants.UserStatusActive
}
