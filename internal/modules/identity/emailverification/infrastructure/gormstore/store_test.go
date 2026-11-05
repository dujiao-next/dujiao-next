package emailverificationstore

import (
	"fmt"
	"testing"
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := fmt.Sprintf("file:email_verification_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&emailverificationdomain.Code{}); err != nil {
		t.Fatalf("migrate verification code failed: %v", err)
	}
	return New(db)
}

func TestStoreReturnsLatestActiveCodeAndPersistsAttempts(t *testing.T) {
	store := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	older := &emailverificationdomain.Code{
		Email: "buyer@example.com", Purpose: "login", Code: "111111",
		ExpiresAt: now.Add(time.Minute), SentAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Minute),
	}
	latest := &emailverificationdomain.Code{
		Email: "buyer@example.com", Purpose: "login", Code: "222222",
		ExpiresAt: now.Add(time.Minute), SentAt: now, CreatedAt: now,
	}
	deletedAt := now.Add(time.Second)
	deleted := &emailverificationdomain.Code{
		Email: "buyer@example.com", Purpose: "login", Code: "333333",
		ExpiresAt: now.Add(time.Minute), SentAt: now.Add(time.Minute), CreatedAt: now.Add(time.Minute), DeletedAt: &deletedAt,
	}
	for _, code := range []*emailverificationdomain.Code{older, latest, deleted} {
		if err := store.Create(code); err != nil {
			t.Fatalf("create verification code failed: %v", err)
		}
	}

	got, err := store.GetLatest("buyer@example.com", "login")
	if err != nil {
		t.Fatalf("get latest verification code failed: %v", err)
	}
	if got == nil || got.ID != latest.ID {
		t.Fatalf("unexpected latest verification code: %#v", got)
	}
	if ok, err := store.ReserveAttempt(got.ID, 1); err != nil || !ok {
		t.Fatalf("reserve attempt failed: ok=%v err=%v", ok, err)
	}
	if ok, err := store.ReserveAttempt(got.ID, 1); err != nil || ok {
		t.Fatalf("reserve attempt beyond max must fail: ok=%v err=%v", ok, err)
	}
	verifiedAt := now.Add(2 * time.Second)
	if ok, err := store.MarkVerified(got.ID, verifiedAt); err != nil || !ok {
		t.Fatalf("mark verified failed: ok=%v err=%v", ok, err)
	}
	if ok, err := store.MarkVerified(got.ID, verifiedAt); err != nil || ok {
		t.Fatalf("verified code must not be marked twice: ok=%v err=%v", ok, err)
	}
	refreshed, err := store.GetLatest("buyer@example.com", "login")
	if err != nil {
		t.Fatalf("reload verification code failed: %v", err)
	}
	if refreshed.AttemptCount != 1 || refreshed.VerifiedAt == nil || !refreshed.VerifiedAt.Equal(verifiedAt) {
		t.Fatalf("verification state was not persisted: %#v", refreshed)
	}
}
