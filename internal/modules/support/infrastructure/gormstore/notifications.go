package gormstore

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/support/domain"
	"gorm.io/gorm"
)

func (s *Store) ClaimReminder(ctx context.Context, id uint, now time.Time) (bool, error) {
	r := s.db.WithContext(ctx).Model(&domain.Reminder{}).Where("id = ? AND due_at <= ? AND (lease_until IS NULL OR lease_until < ?)", id, now, now).Updates(map[string]interface{}{"lease_until": now.Add(2 * time.Minute), "attempts": gorm.Expr("attempts + 1")})
	return r.RowsAffected == 1, r.Error
}
func (s *Store) RetryReminder(ctx context.Context, id uint, due time.Time, reason string) error {
	return s.db.WithContext(ctx).Model(&domain.Reminder{}).Where("id = ?", id).Updates(map[string]interface{}{"due_at": due, "lease_until": time.Time{}, "last_error": reason}).Error
}
func (s *Store) Config(ctx context.Context) (*domain.NotificationConfig, error) {
	v := &domain.NotificationConfig{ID: 1}
	err := s.db.WithContext(ctx).First(v, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, nil
	}
	return v, err
}
func (s *Store) SaveConfig(ctx context.Context, v *domain.NotificationConfig) error {
	v.ID = 1
	return s.db.WithContext(ctx).Save(v).Error
}
func (s *Store) Binding(ctx context.Context, id uint) (*domain.Binding, error) {
	v := &domain.Binding{AdminID: id}
	err := s.db.WithContext(ctx).First(v, "admin_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, nil
	}
	return v, err
}
func (s *Store) SaveBinding(ctx context.Context, v *domain.Binding) error {
	return s.db.WithContext(ctx).Save(v).Error
}
func (s *Store) Bindings(ctx context.Context) ([]domain.Binding, error) {
	v := []domain.Binding{}
	err := s.db.WithContext(ctx).Where("enabled = ? AND (uid <> '' OR (mode = 'simple' AND spt_cipher <> ''))", true).Find(&v).Error
	return v, err
}
func (s *Store) Delivery(ctx context.Context, reminder, admin uint) (*domain.Delivery, error) {
	v := &domain.Delivery{ReminderID: reminder, AdminID: admin}
	err := s.db.WithContext(ctx).Where("reminder_id = ? AND admin_id = ?", reminder, admin).First(v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return v, nil
	}
	return v, err
}
func (s *Store) SaveDelivery(ctx context.Context, v *domain.Delivery) error {
	return s.db.WithContext(ctx).Save(v).Error
}
func (s *Store) Deliveries(ctx context.Context) ([]domain.Delivery, error) {
	v := []domain.Delivery{}
	err := s.db.WithContext(ctx).Order("id DESC").Limit(100).Find(&v).Error
	return v, err
}
