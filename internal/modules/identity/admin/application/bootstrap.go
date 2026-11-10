package adminapplication

import (
	"crypto/rand"
	"strings"

	"github.com/dujiao-next/internal/logger"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"

	"golang.org/x/crypto/bcrypt"
)

const defaultBootstrapUsername = "admin"

// InitDefaultAdmin creates the first administrator or repairs the configured
// bootstrap administrator's super-admin flag when administrators already exist.
// When no password is configured a random one is generated and returned so the
// caller can show it once; it is never written to the log.
func InitDefaultAdmin(store admincontract.Store, username, password string) (generatedPassword string, err error) {
	bootstrapUsername := normalizeBootstrapUsername(username)
	count, err := store.Count()
	if err != nil {
		return "", err
	}
	if count > 0 {
		admin, err := store.GetByUsername(bootstrapUsername)
		if err != nil || admin == nil || admin.IsSuper {
			return "", err
		}
		admin.IsSuper = true
		if err := store.Update(admin); err != nil {
			logger.Warnw("ensure_default_admin_super_failed", "error", err)
			return "", err
		}
		return "", nil
	}

	if password == "" {
		generatedPassword = rand.Text()
		password = generatedPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	admin := &admindomain.Admin{Username: bootstrapUsername, PasswordHash: string(hash), IsSuper: true}
	if err := store.Create(admin); err != nil {
		return "", err
	}
	logger.Warnw("default_admin_created", "username", bootstrapUsername, "password_generated", generatedPassword != "")
	return generatedPassword, nil
}

func normalizeBootstrapUsername(username string) string {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return defaultBootstrapUsername
	}
	return trimmed
}
