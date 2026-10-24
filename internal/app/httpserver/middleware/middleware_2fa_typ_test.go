package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/identity/jwttoken"
	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/modules/identity/userauth/challenge"
	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

const middlewareTestSecret = "user-jwt-test-secret-typ"

func setupUserMiddlewareTestRepo(t *testing.T) (usercontract.Store, *userdomain.User) {
	t.Helper()
	dsn := fmt.Sprintf("file:user_middleware_typ_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := userstore.New(db)
	now := time.Now()
	user := &userdomain.User{
		Email:           "mw@example.com",
		PasswordHash:    "x",
		Status:          "active",
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("create: %v", err)
	}
	return repo, user
}

func signToken(t *testing.T, claims jwt.Claims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(middlewareTestSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

// runUserMiddleware 跑一遍 UserJWTAuthMiddleware，返回业务 status_code：
// 成功放行返回 200（handler 实际响应），失败时统一响应包内 status_code = 401。
func runUserMiddleware(t *testing.T, repo usercontract.Store, token string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(UserJWTAuthMiddleware(middlewareTestSecret, repo))
	r.GET("/me/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/me/ping", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r.ServeHTTP(w, req)

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if okVal, hasOK := resp["ok"]; hasOK {
		if v, ok := okVal.(bool); ok && v {
			return 200
		}
	}
	if sc, ok := resp["status_code"].(float64); ok {
		return int(sc)
	}
	t.Fatalf("unexpected response body: %s", w.Body.String())
	return -1
}

func TestUserJWTMiddlewareAcceptsAccessToken(t *testing.T) {
	repo, user := setupUserMiddlewareTestRepo(t)
	claims := userauthapp.UserJWTClaims{
		UserID:       user.ID,
		Email:        user.Email,
		TokenVersion: user.TokenVersion,
		Typ:          jwttoken.TypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	got := runUserMiddleware(t, repo, signToken(t, claims))
	if got != 200 {
		t.Fatalf("expected status_code=200 for access token, got %d", got)
	}
}

func TestUserJWTMiddlewareAcceptsLegacyTokenWithoutTyp(t *testing.T) {
	// 兼容旧 token：没有 typ 字段时仍按访问 token 放行
	repo, user := setupUserMiddlewareTestRepo(t)
	claims := userauthapp.UserJWTClaims{
		UserID:       user.ID,
		Email:        user.Email,
		TokenVersion: user.TokenVersion,
		// Typ 留空
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	got := runUserMiddleware(t, repo, signToken(t, claims))
	if got != 200 {
		t.Fatalf("expected status_code=200 for legacy token, got %d", got)
	}
}

func TestUserJWTMiddlewareRejectsChallengeToken(t *testing.T) {
	// 关键安全测试：挑战 token 即便签名通过，也必须被中间件拒绝
	repo, user := setupUserMiddlewareTestRepo(t)
	claims := userauthapp.UserChallengeClaims{
		UserID:  user.ID,
		JTI:     "challenge-jti",
		Purpose: challenge.PurposeTwoFactor,
		Typ:     jwttoken.TypeTwoFactorChallenge,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(5 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        "challenge-jti",
		},
	}
	got := runUserMiddleware(t, repo, signToken(t, claims))
	if got != 401 {
		t.Fatalf("expected 401 rejecting challenge token, got %d", got)
	}
}

func TestUserJWTMiddlewareRejectsCustomTypValue(t *testing.T) {
	// 任何非空非 access 的 typ 一律拒绝，防御未来引入的其它 token 类型被误用
	repo, user := setupUserMiddlewareTestRepo(t)
	claims := userauthapp.UserJWTClaims{
		UserID:       user.ID,
		Email:        user.Email,
		TokenVersion: user.TokenVersion,
		Typ:          "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	got := runUserMiddleware(t, repo, signToken(t, claims))
	if got != 401 {
		t.Fatalf("expected 401 for unknown typ, got %d", got)
	}
}

// 令牌只在签发时所在的站点有效：分销站签发的令牌不能拿到主站或其他分销站用，反之亦然。
func TestUserJWTMiddlewareBindsTokenToTenant(t *testing.T) {
	repo, user := setupUserMiddlewareTestRepo(t)
	gin.SetMode(gin.TestMode)
	run := func(tokenRID uint, tenantRID *uint) int {
		claims := userauthapp.UserJWTClaims{
			UserID: user.ID, Email: user.Email, TokenVersion: user.TokenVersion,
			Typ: jwttoken.TypeAccess, ResellerID: tokenRID,
			RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
		}
		r := gin.New()
		r.Use(func(c *gin.Context) {
			if tenantRID != nil {
				ctx := resellercontract.WithTenantContext(c.Request.Context(), resellercontract.TenantContext{ResellerID: tenantRID})
				c.Request = c.Request.WithContext(ctx)
			}
		}, UserJWTAuthMiddleware(middlewareTestSecret, repo))
		r.GET("/me/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/me/ping", nil)
		req.Header.Set("Authorization", "Bearer "+signToken(t, claims))
		r.ServeHTTP(w, req)
		var resp map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		if ok, _ := resp["ok"].(bool); ok {
			return 200
		}
		return 401
	}
	reseller := uint(7)
	other := uint(8)
	if got := run(7, nil); got != 401 {
		t.Fatalf("reseller token on main site must be rejected, got %d", got)
	}
	if got := run(7, &other); got != 401 {
		t.Fatalf("reseller token on another reseller site must be rejected, got %d", got)
	}
	if got := run(0, &reseller); got != 401 {
		t.Fatalf("main token on reseller site must be rejected, got %d", got)
	}
	if got := run(7, &reseller); got != 200 {
		t.Fatalf("reseller token on its own site must pass, got %d", got)
	}
}
