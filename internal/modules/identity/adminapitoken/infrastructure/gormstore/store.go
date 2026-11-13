package adminapitokenstore

import (
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/identity/adminapitoken/domain"
	"gorm.io/gorm"
)

type Store struct{ db *gorm.DB }

func New(db *gorm.DB) *Store { return &Store{db: db} }

func (s *Store) Create(token *adminapitokendomain.Token) error {
	return s.db.Create(token).Error
}

func (s *Store) List(adminID uint) ([]adminapitokendomain.Token, error) {
	items := make([]adminapitokendomain.Token, 0)
	err := s.db.Omit("digest").Where("admin_id = ?", adminID).Order("id DESC").Find(&items).Error
	return items, err
}

func (s *Store) GetByDigest(digest string) (*adminapitokendomain.Token, error) {
	var token adminapitokendomain.Token
	err := s.db.Where("digest = ?", digest).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &token, err
}

func (s *Store) Revoke(adminID, tokenID uint, now time.Time) (bool, error) {
	result := s.db.Model(&adminapitokendomain.Token{}).
		Where("id = ? AND admin_id = ? AND revoked_at IS NULL", tokenID, adminID).
		Update("revoked_at", now)
	return result.RowsAffected != 0, result.Error
}

func (s *Store) GetByID(adminID, tokenID uint) (*adminapitokendomain.Token, error) {
	var token adminapitokendomain.Token
	err := s.db.Where("id = ? AND admin_id = ?", tokenID, adminID).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &token, err
}

// Compare-and-swap prevents concurrent rotations from returning superseded
// credentials and cannot revive a revoked or expired token.
func (s *Store) Rotate(adminID, tokenID uint, oldDigest, newDigest string, now time.Time) (bool, error) {
	result := s.db.Model(&adminapitokendomain.Token{}).
		Where("id = ? AND admin_id = ? AND digest = ? AND revoked_at IS NULL AND expires_at > ?", tokenID, adminID, oldDigest, now.UTC()).
		Updates(map[string]interface{}{"digest": newDigest, "last_used_at": nil})
	return result.RowsAffected != 0, result.Error
}

// Touch never saves a stale token snapshot, so concurrent revocation cannot be undone.
// At most one timestamp write per minute avoids a SQLite write on every request.
func (s *Store) Touch(tokenID uint, now time.Time) error {
	return s.db.Model(&adminapitokendomain.Token{}).
		Where("id = ? AND revoked_at IS NULL AND (last_used_at IS NULL OR last_used_at < ?)", tokenID, now.Add(-time.Minute)).
		Update("last_used_at", now).Error
}
