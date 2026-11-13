package adminapitoken_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	tokenapp "github.com/dujiao-next/internal/modules/identity/adminapitoken/application"
	tokencontract "github.com/dujiao-next/internal/modules/identity/adminapitoken/contract"
	tokendomain "github.com/dujiao-next/internal/modules/identity/adminapitoken/domain"
	tokenstore "github.com/dujiao-next/internal/modules/identity/adminapitoken/infrastructure/gormstore"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setup(t *testing.T) (*gorm.DB, *tokenapp.Service, *adminstore.Store, *admindomain.Admin) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&admindomain.Admin{}, &tokendomain.Token{}))
	admins := adminstore.New(db)
	admin := &admindomain.Admin{Username: "automation", PasswordHash: "hash"}
	require.NoError(t, admins.Create(admin))
	return db, tokenapp.NewService(tokenstore.New(db), admins), admins, admin
}

func TestLifecycleStoresOnlyDigestAndRevokesOneOwnedToken(t *testing.T) {
	db, service, _, admin := setup(t)
	token, secret, err := service.Create(admin.ID, " inventory ", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(secret, tokenapp.Prefix))
	require.Len(t, secret, len(tokenapp.Prefix)+64)
	var persisted tokendomain.Token
	require.NoError(t, db.First(&persisted, token.ID).Error)
	require.NotContains(t, persisted.Digest, secret)
	require.Len(t, persisted.Digest, 64)
	require.Equal(t, "inventory", persisted.Name)
	_, _, err = service.Authenticate(secret)
	require.NoError(t, err)
	_, _, err = service.Authenticate(secret[:len(secret)-1] + "!")
	require.ErrorIs(t, err, tokenapp.ErrInvalid)
	second, secondSecret, err := service.Create(admin.ID, "second", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	require.NotEqual(t, secret, secondSecret)
	require.NotEqual(t, token.ID, second.ID)
	require.ErrorIs(t, service.Revoke(admin.ID+1, token.ID), tokenapp.ErrNotFound)
	require.NoError(t, service.Touch(token.ID))
	require.NoError(t, service.Revoke(admin.ID, token.ID))
	require.NoError(t, service.Touch(token.ID)) // stale usage update cannot resurrect it
	_, _, err = service.Authenticate(secret)
	require.ErrorIs(t, err, tokenapp.ErrInvalid)
	_, _, err = service.Authenticate(secondSecret)
	require.NoError(t, err)
	items, err := service.List(admin.ID)
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "revoked", items[1].Status)
	require.NotNil(t, items[1].LastUsedAt)
	data, err := json.Marshal(items)
	require.NoError(t, err)
	for _, sensitive := range []string{secret, secondSecret, persisted.Digest, "digest", "admin_token_version"} {
		require.NotContains(t, string(data), sensitive)
	}
}

func TestCredentialRechecksOwnerAndExpiry(t *testing.T) {
	for _, change := range []string{"expired", "revoked", "deleted", "super", "password", "2fa", "invalid_before"} {
		t.Run(change, func(t *testing.T) {
			db, service, admins, admin := setup(t)
			token, secret, err := service.Create(admin.ID, "sync", time.Now().Add(time.Hour), nil)
			require.NoError(t, err)
			_, _, err = service.Authenticate(secret)
			require.NoError(t, err)
			switch change {
			case "expired":
				require.NoError(t, db.Model(token).Update("expires_at", time.Now().Add(-time.Second)).Error)
			case "revoked":
				require.NoError(t, service.Revoke(admin.ID, token.ID))
			case "deleted":
				require.NoError(t, admins.Delete(admin.ID))
			case "super":
				admin.IsSuper = true
				require.NoError(t, admins.Update(admin))
			case "password":
				require.NoError(t, admins.UpdatePassword(admin.ID, "new-hash"))
			case "2fa":
				require.NoError(t, admins.ClearTOTP(admin.ID))
			case "invalid_before":
				now := time.Now()
				admin.TokenInvalidBefore = &now
				require.NoError(t, admins.Update(admin))
			}
			_, _, err = service.Authenticate(secret)
			require.ErrorIs(t, err, tokenapp.ErrInvalid)
			items, err := service.List(admin.ID)
			require.NoError(t, err)
			require.NotEqual(t, "active", items[0].Status)
			rotated, newSecret, err := service.Rotate(admin.ID, token.ID)
			require.ErrorIs(t, err, tokenapp.ErrRotation)
			require.Nil(t, rotated)
			require.Empty(t, newSecret)
		})
	}
}

func TestRotationReplacesSecretAndPreservesRestrictions(t *testing.T) {
	db, service, _, admin := setup(t)
	token, oldSecret, err := service.Create(admin.ID, "sync", time.Now().Add(time.Hour), []string{"192.0.2.0/24"})
	require.NoError(t, err)
	_, otherSecret, err := service.Create(admin.ID, "other", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	require.NoError(t, service.Touch(token.ID))
	_, failedSecret, err := service.Rotate(admin.ID+1, token.ID)
	require.ErrorIs(t, err, tokenapp.ErrNotFound)
	require.Empty(t, failedSecret)

	rotated, newSecret, err := service.Rotate(admin.ID, token.ID)
	require.NoError(t, err)
	require.NotEqual(t, oldSecret, newSecret)
	require.Len(t, newSecret, len(tokenapp.Prefix)+64)
	require.Equal(t, token.ID, rotated.ID)
	require.Equal(t, token.Name, rotated.Name)
	require.WithinDuration(t, token.ExpiresAt, rotated.ExpiresAt, time.Microsecond)
	require.WithinDuration(t, token.CreatedAt, rotated.CreatedAt, time.Microsecond)
	require.Equal(t, token.IPWhitelist, rotated.IPWhitelist)
	require.Nil(t, rotated.LastUsedAt)
	_, _, err = service.Authenticate(oldSecret)
	require.ErrorIs(t, err, tokenapp.ErrInvalid)
	_, _, err = service.Authenticate(newSecret)
	require.NoError(t, err)
	_, _, err = service.Authenticate(otherSecret)
	require.NoError(t, err)
	var persisted tokendomain.Token
	require.NoError(t, db.First(&persisted, token.ID).Error)
	require.NotEqual(t, token.Digest, persisted.Digest)
	require.Equal(t, rotated.Digest, persisted.Digest)
	data, err := json.Marshal(rotated)
	require.NoError(t, err)
	require.NotContains(t, string(data), newSecret)
	require.NotContains(t, string(data), persisted.Digest)
}

type rotationBarrierStore struct {
	tokencontract.Store
	ready  chan struct{}
	resume chan struct{}
}

func (s *rotationBarrierStore) GetByID(adminID, tokenID uint) (*tokendomain.Token, error) {
	token, err := s.Store.GetByID(adminID, tokenID)
	s.ready <- struct{}{}
	<-s.resume
	return token, err
}

func TestConcurrentRotationReturnsOnlyOneUsableSecret(t *testing.T) {
	db, service, admins, admin := setup(t)
	token, original, err := service.Create(admin.ID, "sync", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	store := &rotationBarrierStore{Store: tokenstore.New(db), ready: make(chan struct{}, 2), resume: make(chan struct{})}
	concurrent := tokenapp.NewService(store, admins)
	type result struct {
		secret string
		err    error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, secret, err := concurrent.Rotate(admin.ID, token.ID)
			results <- result{secret, err}
		}()
	}
	<-store.ready
	<-store.ready
	close(store.resume)
	successes := 0
	for i := 0; i < 2; i++ {
		result := <-results
		if result.err == nil {
			successes++
			_, _, err := service.Authenticate(result.secret)
			require.NoError(t, err)
		} else {
			require.ErrorIs(t, result.err, tokenapp.ErrRotation)
			require.Empty(t, result.secret)
		}
	}
	require.Equal(t, 1, successes)
	_, _, err = service.Authenticate(original)
	require.ErrorIs(t, err, tokenapp.ErrInvalid)
}

type failingRotationStore struct{ tokencontract.Store }

func (s failingRotationStore) Rotate(uint, uint, string, string, time.Time) (bool, error) {
	return false, errors.New("write failed")
}

func TestFailedRotationKeepsOldSecretAndCannotUndoRevocation(t *testing.T) {
	db, service, admins, admin := setup(t)
	token, oldSecret, err := service.Create(admin.ID, "sync", time.Now().Add(time.Hour), nil)
	require.NoError(t, err)
	store := tokenstore.New(db)
	failing := tokenapp.NewService(failingRotationStore{store}, admins)
	_, secret, err := failing.Rotate(admin.ID, token.ID)
	require.Error(t, err)
	require.Empty(t, secret)
	_, _, err = service.Authenticate(oldSecret)
	require.NoError(t, err)
	require.NoError(t, service.Revoke(admin.ID, token.ID))
	ok, err := store.Rotate(admin.ID, token.ID, token.Digest, strings.Repeat("a", 64), time.Now())
	require.NoError(t, err)
	require.False(t, ok)
	_, _, err = service.Authenticate(oldSecret)
	require.ErrorIs(t, err, tokenapp.ErrInvalid)
}

func TestCreationValidation(t *testing.T) {
	_, service, admins, admin := setup(t)
	for _, name := range []string{"", " ", strings.Repeat("中", 81)} {
		_, secret, err := service.Create(admin.ID, name, time.Now().Add(time.Hour), nil)
		require.ErrorIs(t, err, tokenapp.ErrInput)
		require.Empty(t, secret)
	}
	for _, expiry := range []time.Time{time.Time{}, time.Now().Add(-time.Second), time.Now().Add(366 * 24 * time.Hour)} {
		_, _, err := service.Create(admin.ID, "sync", expiry, nil)
		require.ErrorIs(t, err, tokenapp.ErrInput)
	}
	_, _, err := service.Create(admin.ID+1, "sync", time.Now().Add(time.Hour), nil)
	require.ErrorIs(t, err, tokenapp.ErrSubAccount)
	admin.IsSuper = true
	require.NoError(t, admins.Update(admin))
	_, _, err = service.Create(admin.ID, "sync", time.Now().Add(time.Hour), nil)
	require.ErrorIs(t, err, tokenapp.ErrSubAccount)
}

func TestIPWhitelistPersistenceAndLegacyTokens(t *testing.T) {
	db, service, _, admin := setup(t)
	token, secret, err := service.Create(admin.ID, "restricted", time.Now().Add(time.Hour),
		[]string{"192.0.2.42/24", "2001:db8::1"})
	require.NoError(t, err)
	persisted, _, err := service.Authenticate(secret)
	require.NoError(t, err)
	require.Equal(t, []string{"192.0.2.0/24", "2001:db8::1"}, persisted.IPWhitelist)
	require.True(t, tokenapp.AllowsIP(persisted.IPWhitelist, "192.0.2.1"))
	require.False(t, tokenapp.AllowsIP(persisted.IPWhitelist, "198.51.100.1"))
	items, err := service.List(admin.ID)
	require.NoError(t, err)
	require.Equal(t, persisted.IPWhitelist, items[0].IPWhitelist)

	_, secretOnError, err := service.Create(admin.ID, "bad", time.Now().Add(time.Hour), []string{"invalid"})
	require.ErrorIs(t, err, tokenapp.ErrIPWhitelist)
	require.Empty(t, secretOnError)
	var count int64
	require.NoError(t, db.Model(&tokendomain.Token{}).Count(&count).Error)
	require.EqualValues(t, 1, count)

	// TEXT values written outside the serializer must still be decoded strictly.
	require.NoError(t, db.Model(token).Update("ip_whitelist", `["192.0.2.1","invalid"]`).Error)
	persisted, _, err = service.Authenticate(secret)
	require.NoError(t, err)
	require.False(t, tokenapp.AllowsIP(persisted.IPWhitelist, "192.0.2.1"))
	require.NoError(t, db.Model(token).Update("ip_whitelist", `not-json`).Error)
	_, _, err = service.Authenticate(secret)
	require.Error(t, err)

	// NULL is the state of the added column for credentials created before upgrade.
	require.NoError(t, db.Model(token).Update("ip_whitelist", nil).Error)
	persisted, _, err = service.Authenticate(secret)
	require.NoError(t, err)
	require.True(t, tokenapp.AllowsIP(persisted.IPWhitelist, "198.51.100.1"))
	items, err = service.List(admin.ID)
	require.NoError(t, err)
	require.Equal(t, []string{}, items[0].IPWhitelist)
}
