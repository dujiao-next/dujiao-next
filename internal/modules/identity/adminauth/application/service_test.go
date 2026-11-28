package application

import (
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/modules/identity/jwttoken"
)

const testSecret = "test-secret-key-for-session-refresh"

func newTestService(maxSessionHours int) *Service {
	return NewService(&config.Config{
		JWT: config.JWTConfig{
			SecretKey:       testSecret,
			ExpireHours:     24,
			MaxSessionHours: maxSessionHours,
		},
	}, nil)
}

func testAdmin() *admindomain.Admin {
	return &admindomain.Admin{ID: 7, Username: "ops", TokenVersion: 3}
}

func TestGenerateJWTStampsSessionStart(t *testing.T) {
	svc := newTestService(168)
	before := time.Now().Unix()

	token, expiresAt, err := svc.GenerateJWT(testAdmin())
	if err != nil {
		t.Fatalf("GenerateJWT: %v", err)
	}

	claims, err := svc.ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if claims.Typ != jwttoken.TypeAccess {
		t.Fatalf("typ = %q, want %q", claims.Typ, jwttoken.TypeAccess)
	}
	if claims.TokenVersion != 3 {
		t.Fatalf("token_version = %d, want 3", claims.TokenVersion)
	}
	if claims.SessionStartedAt < before || claims.SessionStartedAt > time.Now().Unix() {
		t.Fatalf("session_started_at = %d, want within [%d, now]", claims.SessionStartedAt, before)
	}
	if !expiresAt.After(time.Now()) {
		t.Fatalf("expires_at = %v, want in the future", expiresAt)
	}
}

func TestRefreshJWTKeepsSessionStartAndExtendsExpiry(t *testing.T) {
	svc := newTestService(168)
	admin := testAdmin()
	startedAt := time.Now().Add(-10 * time.Hour).Unix()

	original, _, err := svc.generateJWT(admin, startedAt)
	if err != nil {
		t.Fatalf("generateJWT: %v", err)
	}
	originalClaims, err := svc.ParseJWT(original)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}

	token, _, err := svc.RefreshJWT(admin, startedAt)
	if err != nil {
		t.Fatalf("RefreshJWT: %v", err)
	}

	claims, err := svc.ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if claims.SessionStartedAt != startedAt {
		t.Fatalf("session_started_at = %d, want %d (must not slide)", claims.SessionStartedAt, startedAt)
	}
	if claims.TokenVersion != admin.TokenVersion {
		t.Fatalf("token_version = %d, want %d", claims.TokenVersion, admin.TokenVersion)
	}
	if claims.ExpiresAt.Before(originalClaims.ExpiresAt.Time) {
		t.Fatalf("refreshed expiry %v is earlier than original %v", claims.ExpiresAt.Time, originalClaims.ExpiresAt.Time)
	}
	// 续期必须按当前时间重算，而不是沿用旧 exp。JWT NumericDate 只有秒级精度，
	// 因此用「从现在起约 expire_hours」判断，允许几秒误差。
	expected := time.Now().Add(time.Duration(24) * time.Hour)
	if diff := claims.ExpiresAt.Time.Sub(expected); diff > time.Minute || diff < -time.Minute {
		t.Fatalf("refreshed expiry %v is not ~24h from now (%v)", claims.ExpiresAt.Time, expected)
	}
}

func TestRefreshJWTRejectsExpiredSession(t *testing.T) {
	svc := newTestService(168)
	startedAt := time.Now().Add(-169 * time.Hour).Unix()

	_, _, err := svc.RefreshJWT(testAdmin(), startedAt)
	if !errors.Is(err, jwttoken.ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired", err)
	}
}

func TestRefreshJWTAllowsLegacyTokenWithinCap(t *testing.T) {
	svc := newTestService(168)
	// 续期机制上线前签发的 token 没有 sst，按当前时间重新起算。
	token, _, err := svc.RefreshJWT(testAdmin(), 0)
	if err != nil {
		t.Fatalf("RefreshJWT: %v", err)
	}

	claims, err := svc.ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if claims.SessionStartedAt <= 0 {
		t.Fatalf("session_started_at = %d, want > 0", claims.SessionStartedAt)
	}
}

func TestRefreshJWTWithoutSessionCap(t *testing.T) {
	svc := newTestService(0)
	startedAt := time.Now().Add(-10000 * time.Hour).Unix()

	if _, _, err := svc.RefreshJWT(testAdmin(), startedAt); err != nil {
		t.Fatalf("RefreshJWT with no cap: %v", err)
	}
}

func TestRefreshJWTRejectsFutureSessionStart(t *testing.T) {
	svc := newTestService(168)
	future := time.Now().Add(48 * time.Hour).Unix()

	// 未来时间戳（时钟漂移或被篡改的 token）不应延长会话，按当前时间重算。
	token, _, err := svc.RefreshJWT(testAdmin(), future)
	if err != nil {
		t.Fatalf("RefreshJWT: %v", err)
	}

	claims, err := svc.ParseJWT(token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}
	if claims.SessionStartedAt > time.Now().Unix() {
		t.Fatalf("session_started_at = %d, want <= now", claims.SessionStartedAt)
	}
}
