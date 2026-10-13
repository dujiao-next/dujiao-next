package support

import (
	"crypto/sha256"

	"github.com/dujiao-next/internal/crypto"
)

type SecretBox struct{ Secret string }

func (s SecretBox) Encrypt(v string) (string, error) {
	key := sha256.Sum256([]byte(s.Secret))
	return crypto.Encrypt(key[:], v)
}
func (s SecretBox) Decrypt(v string) (string, error) {
	key := sha256.Sum256([]byte(s.Secret))
	return crypto.Decrypt(key[:], v)
}
