package gormstore

import (
	"context"
	"errors"
	"time"

	"github.com/dujiao-next/internal/modules/support/contract"
	"github.com/dujiao-next/internal/modules/support/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) CreateScan(ctx context.Context, v *domain.ScanSession) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.Binding{AdminID: v.AdminID}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.Binding{}).Where("admin_id = ?", v.AdminID).Updates(map[string]interface{}{"pending_uid": "", "pending_hash": "", "pending_mode": "", "pending_spt_cipher": ""}).Error; err != nil {
			return err
		}
		if err := tx.Model(&domain.ScanSession{}).Where("admin_id = ? AND status IN ?", v.AdminID, []string{"waiting", "scanned"}).Update("status", "cancelled").Error; err != nil {
			return err
		}
		return tx.Create(v).Error
	})
}

func (s *Store) CancelAdminScans(ctx context.Context, admin uint) error {
	return s.db.WithContext(ctx).Model(&domain.ScanSession{}).Where("admin_id = ? AND status IN ?", admin, []string{"waiting", "scanned"}).Updates(map[string]interface{}{"status": "cancelled", "provider_code": "", "verification_hash": ""}).Error
}
func (s *Store) Scan(ctx context.Context, admin uint, id string) (*domain.ScanSession, error) {
	var v domain.ScanSession
	err := s.db.WithContext(ctx).Where("id = ? AND admin_id = ?", id, admin).First(&v).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, contract.ErrNotFound
	}
	return &v, err
}
func (s *Store) ClaimScanPoll(ctx context.Context, admin uint, id string, now time.Time) (bool, error) {
	r := s.db.WithContext(ctx).Model(&domain.ScanSession{}).Where("id = ? AND admin_id = ? AND status = 'waiting' AND expires_at > ? AND next_poll_at <= ?", id, admin, now, now).Update("next_poll_at", now.Add(11*time.Second))
	return r.RowsAffected == 1, r.Error
}
func (s *Store) SetScanCandidate(ctx context.Context, admin uint, id, uid, hash string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r := tx.Model(&domain.ScanSession{}).Where("id = ? AND admin_id = ? AND status = 'waiting' AND expires_at > ?", id, admin, time.Now()).Updates(map[string]interface{}{"status": "scanned", "uid": uid, "verification_hash": hash})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return contract.ErrConflict
		}
		var v domain.ScanSession
		if err := tx.First(&v, "id = ?", id).Error; err != nil {
			return err
		}
		return tx.Model(&domain.Binding{}).Where("admin_id = ?", admin).Updates(map[string]interface{}{"pending_mode": "standard", "pending_spt_cipher": "", "pending_uid": uid, "pending_hash": hash, "pending_expires": v.ExpiresAt}).Error
	})
}
func (s *Store) ConfirmScan(ctx context.Context, admin uint, id, hash string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var v domain.ScanSession
		if err := tx.Where("id = ? AND admin_id = ? AND status = 'scanned' AND expires_at > ? AND verification_hash = ?", id, admin, time.Now(), hash).First(&v).Error; err != nil {
			return contract.ErrForbidden
		}
		r := tx.Model(&domain.Binding{}).Where("admin_id = ? AND pending_uid = ? AND pending_hash = ? AND pending_expires > ?", admin, v.UID, hash, time.Now()).Updates(map[string]interface{}{"mode": "standard", "spt_cipher": "", "uid": v.UID, "enabled": true, "pending_uid": "", "pending_hash": "", "pending_mode": "", "pending_spt_cipher": ""})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return contract.ErrConflict
		}
		r = tx.Model(&domain.ScanSession{}).Where("id = ? AND status = 'scanned'", id).Updates(map[string]interface{}{"status": "confirmed", "provider_code": "", "verification_hash": ""})
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected != 1 {
			return contract.ErrConflict
		}
		return nil
	})
}
func (s *Store) CancelScan(ctx context.Context, admin uint, id string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var v domain.ScanSession
		if err := tx.Where("id = ? AND admin_id = ?", id, admin).First(&v).Error; err != nil {
			return contract.ErrNotFound
		}
		if v.Status == "confirmed" || v.Status == "cancelled" {
			return nil
		}
		if err := tx.Model(&domain.Binding{}).Where("admin_id = ? AND pending_hash = ? AND pending_hash <> ''", admin, v.VerificationHash).Updates(map[string]interface{}{"pending_uid": "", "pending_hash": "", "pending_mode": "", "pending_spt_cipher": ""}).Error; err != nil {
			return err
		}
		return tx.Model(&domain.ScanSession{}).Where("id = ? AND status IN ?", id, []string{"waiting", "scanned"}).Updates(map[string]interface{}{"status": "cancelled", "provider_code": "", "verification_hash": ""}).Error
	})
}
