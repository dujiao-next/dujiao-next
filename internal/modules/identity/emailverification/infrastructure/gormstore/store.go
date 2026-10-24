package emailverificationstore

import (
	"errors"
	"time"

	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"

	"gorm.io/gorm"
)

type Store struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{db: db}
}

func (store *Store) Create(code *emailverificationdomain.Code) error {
	return store.db.Create(code).Error
}

func (store *Store) GetLatest(email, purpose string) (*emailverificationdomain.Code, error) {
	var record emailverificationdomain.Code
	if err := store.db.Where("email = ? AND purpose = ? AND deleted_at IS NULL", email, purpose).
		Order("sent_at desc, id desc").
		First(&record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (store *Store) MarkVerified(id uint, verifiedAt time.Time) (bool, error) {
	result := store.db.Model(&emailverificationdomain.Code{}).
		Where("id = ? AND deleted_at IS NULL AND verified_at IS NULL", id).
		Update("verified_at", verifiedAt)
	return result.RowsAffected == 1, result.Error
}

func (store *Store) Delete(id uint) error {
	return store.db.Model(&emailverificationdomain.Code{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", time.Now()).Error
}

func (store *Store) ReserveAttempt(id uint, maxAttempts int) (bool, error) {
	query := store.db.Model(&emailverificationdomain.Code{}).
		Where("id = ? AND deleted_at IS NULL AND verified_at IS NULL", id)
	if maxAttempts > 0 {
		query = query.Where("attempt_count < ?", maxAttempts)
	}
	result := query.UpdateColumn("attempt_count", gorm.Expr("attempt_count + 1"))
	return result.RowsAffected == 1, result.Error
}
