package adminauthhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/modules/identity/jwttoken"

	"github.com/gin-gonic/gin"
)

type fakeSessionAuth struct {
	admin        *admindomain.Admin
	getErr       error
	refreshErr   error
	token        string
	expiresAt    time.Time
	receivedSST  int64
	refreshCalls int
}

func (f *fakeSessionAuth) Login(string, string) (*AuthLoginResult, error) {
	return nil, ErrInvalidCredentials
}

func (f *fakeSessionAuth) ChangePassword(uint, string, string) error { return nil }

func (f *fakeSessionAuth) GetAdminByID(uint) (*admindomain.Admin, error) {
	return f.admin, f.getErr
}

func (f *fakeSessionAuth) RefreshJWT(admin *admindomain.Admin, sessionStartedAt int64) (string, time.Time, error) {
	f.refreshCalls++
	f.receivedSST = sessionStartedAt
	if f.refreshErr != nil {
		return "", time.Time{}, f.refreshErr
	}
	return f.token, f.expiresAt, nil
}

type refreshResponse struct {
	StatusCode int    `json:"status_code"`
	Msg        string `json:"msg"`
	Data       struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	} `json:"data"`
}

func runRefresh(t *testing.T, auth LoginAuthService, ctxValues map[string]interface{}) refreshResponse {
	t.Helper()
	gin.SetMode(gin.TestMode)

	r := gin.New()
	h := NewAdminLoginHandler(auth, nil, nil)
	r.POST("/auth/refresh", func(c *gin.Context) {
		for key, value := range ctxValues {
			c.Set(key, value)
		}
		h.RefreshSession(c)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	r.ServeHTTP(w, req)

	var resp refreshResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v (body=%s)", err, w.Body.String())
	}
	return resp
}

func TestRefreshSessionReturnsNewTokenAndKeepsSessionStart(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)
	auth := &fakeSessionAuth{
		admin:     &admindomain.Admin{ID: 7, Username: "ops", TokenVersion: 3},
		token:     "refreshed-token",
		expiresAt: expiresAt,
	}

	resp := runRefresh(t, auth, map[string]interface{}{
		"admin_id":                 uint(7),
		"admin_token_version":      uint64(3),
		"admin_session_started_at": int64(1700000000),
	})

	if resp.StatusCode != 0 {
		t.Fatalf("status_code = %d, want 0 (msg=%s)", resp.StatusCode, resp.Msg)
	}
	if resp.Data.Token != "refreshed-token" {
		t.Fatalf("token = %q, want refreshed-token", resp.Data.Token)
	}
	if resp.Data.ExpiresAt == "" {
		t.Fatal("expires_at is empty")
	}
	// 会话起点必须原样透传给应用层，否则续期会把会话上限无限延长。
	if auth.receivedSST != 1700000000 {
		t.Fatalf("session start passed to service = %d, want 1700000000", auth.receivedSST)
	}
}

func TestRefreshSessionRejectsStaleTokenVersion(t *testing.T) {
	auth := &fakeSessionAuth{
		admin: &admindomain.Admin{ID: 7, TokenVersion: 4},
		token: "should-not-be-issued",
	}

	resp := runRefresh(t, auth, map[string]interface{}{
		"admin_id":            uint(7),
		"admin_token_version": uint64(3),
	})

	if resp.StatusCode == 0 {
		t.Fatal("expected rejection when token version no longer matches")
	}
	if auth.refreshCalls != 0 {
		t.Fatalf("RefreshJWT calls = %d, want 0", auth.refreshCalls)
	}
}

func TestRefreshSessionRejectsMissingAdmin(t *testing.T) {
	auth := &fakeSessionAuth{admin: nil}

	resp := runRefresh(t, auth, map[string]interface{}{
		"admin_id": uint(7),
	})

	if resp.StatusCode == 0 {
		t.Fatal("expected rejection when admin no longer exists")
	}
	if auth.refreshCalls != 0 {
		t.Fatalf("RefreshJWT calls = %d, want 0", auth.refreshCalls)
	}
}

func TestRefreshSessionReportsSessionLifetimeExceeded(t *testing.T) {
	auth := &fakeSessionAuth{
		admin:      &admindomain.Admin{ID: 7, TokenVersion: 3},
		refreshErr: jwttoken.ErrSessionExpired,
	}

	resp := runRefresh(t, auth, map[string]interface{}{
		"admin_id":                 uint(7),
		"admin_token_version":      uint64(3),
		"admin_session_started_at": int64(1),
	})

	if resp.StatusCode != 401 {
		t.Fatalf("status_code = %d, want 401 (msg=%s)", resp.StatusCode, resp.Msg)
	}
	if resp.Data.Token != "" {
		t.Fatalf("token = %q, want empty on rejected refresh", resp.Data.Token)
	}
}
