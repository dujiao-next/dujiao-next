package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminauthapp "github.com/dujiao-next/internal/modules/identity/adminauth/application"
	"github.com/dujiao-next/internal/modules/identity/jwttoken"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func TestResolveAllowedOrigin(t *testing.T) {
	got := resolveAllowedOrigin("https://example.com", []string{"*"}, false)
	if got != "*" {
		t.Fatalf("wildcard without credentials should return *, got %s", got)
	}

	got = resolveAllowedOrigin("https://example.com", []string{"*"}, true)
	if got != "https://example.com" {
		t.Fatalf("wildcard with credentials should echo origin, got %s", got)
	}

	got = resolveAllowedOrigin("https://a.example.com", []string{"https://a.example.com", "https://b.example.com"}, false)
	if got != "https://a.example.com" {
		t.Fatalf("allow-list should return matched origin, got %s", got)
	}

	got = resolveAllowedOrigin("https://x.example.com", []string{"https://a.example.com"}, false)
	if got != "" {
		t.Fatalf("unmatched origin should be empty, got %s", got)
	}
}

func TestRequestIDMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(RequestIDMiddleware())
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"request_id": getRequestID(c)})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(requestIDHeader, "req-123")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if w.Header().Get(requestIDHeader) != "req-123" {
		t.Fatalf("response request id want req-123 got %s", w.Header().Get(requestIDHeader))
	}
	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp["request_id"] != "req-123" {
		t.Fatalf("context request id want req-123 got %s", resp["request_id"])
	}

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/ping", nil)
	r.ServeHTTP(w2, req2)
	generated := w2.Header().Get(requestIDHeader)
	if generated == "" {
		t.Fatalf("generated request id should not be empty")
	}
	if resp := strings.TrimSpace(generated); resp == "" {
		t.Fatalf("generated request id should not be blank")
	}
}

func TestJWTAuthMiddlewareMissingSecret(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.Use(JWTAuthMiddleware("", nil))
	r.GET("/admin/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/ping", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	var resp struct {
		StatusCode int `json:"status_code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if resp.StatusCode != 401 {
		t.Fatalf("status_code want 401 got %d", resp.StatusCode)
	}
}

type fakeAdminStore struct {
	admin *admindomain.Admin
}

func (f *fakeAdminStore) GetByUsername(string) (*admindomain.Admin, error) { return f.admin, nil }
func (f *fakeAdminStore) GetByID(uint) (*admindomain.Admin, error)         { return f.admin, nil }
func (f *fakeAdminStore) List() ([]admindomain.Admin, error)               { return nil, nil }
func (f *fakeAdminStore) Count() (int64, error)                            { return 0, nil }
func (f *fakeAdminStore) Create(*admindomain.Admin) error                  { return nil }
func (f *fakeAdminStore) Update(*admindomain.Admin) error                  { return nil }
func (f *fakeAdminStore) Delete(uint) error                                { return nil }

func (f *fakeAdminStore) UpdateTOTPPending(uint, string, time.Time) error         { return nil }
func (f *fakeAdminStore) UpdateTOTPEnabled(uint, string, time.Time, string) error { return nil }
func (f *fakeAdminStore) UpdateRecoveryCodes(uint, string) error                  { return nil }
func (f *fakeAdminStore) ClearTOTP(uint) error                                    { return nil }
func (f *fakeAdminStore) UpdatePassword(uint, string) error                       { return nil }

func signAdminToken(t *testing.T, secret string, admin *admindomain.Admin, sessionStartedAt int64) string {
	t.Helper()
	now := time.Now()
	claims := adminauthapp.JWTClaims{
		AdminID:          admin.ID,
		Username:         admin.Username,
		TokenVersion:     admin.TokenVersion,
		Typ:              jwttoken.TypeAccess,
		SessionStartedAt: sessionStartedAt,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// 续期接口依赖中间件写入的会话起点与 token 版本；键名一旦对不上，
// RefreshJWT 会拿到 sst=0，等于把会话上限静默取消。
func TestJWTAuthMiddlewareExposesSessionContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "middleware-session-context-secret"
	admin := &admindomain.Admin{ID: 7, Username: "ops", TokenVersion: 3}
	token := signAdminToken(t, secret, admin, 1700000000)

	r := gin.New()
	r.Use(JWTAuthMiddleware(secret, &fakeAdminStore{admin: admin}))
	r.GET("/context", func(c *gin.Context) {
		sessionStartedAt, _ := c.Get("admin_session_started_at")
		tokenVersion, _ := c.Get("admin_token_version")
		c.JSON(http.StatusOK, gin.H{
			"admin_id":  c.GetUint("admin_id"),
			"username":  c.GetString("username"),
			"sst":       sessionStartedAt,
			"token_ver": tokenVersion,
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/context", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", w.Code, w.Body.String())
	}

	var resp struct {
		AdminID      uint   `json:"admin_id"`
		Username     string `json:"username"`
		SST          int64  `json:"sst"`
		TokenVersion uint64 `json:"token_ver"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode body: %v (body=%s)", err, w.Body.String())
	}

	if resp.AdminID != admin.ID || resp.Username != admin.Username {
		t.Fatalf("identity not propagated: %+v", resp)
	}
	if resp.SST != 1700000000 {
		t.Fatalf("session start in context = %d, want 1700000000", resp.SST)
	}
	if resp.TokenVersion != admin.TokenVersion {
		t.Fatalf("token version in context = %d, want %d", resp.TokenVersion, admin.TokenVersion)
	}
}
