package gormstore

import (
	"errors"
	"strings"

	resellercontract "github.com/dujiao-next/internal/modules/reseller/contract"

	resellerdomain "github.com/dujiao-next/internal/modules/reseller/domain"
	"github.com/dujiao-next/internal/shared/money"

	"gorm.io/gorm"
)

// CreateProfile 创建分销商资料。
func (r *Store) CreateProfile(profile *resellerdomain.Profile) error {
	if profile == nil {
		return errors.New("reseller profile is nil")
	}
	return r.db.Create(profile).Error
}

// GetProfileByID 按 ID 获取分销商资料。
func (r *Store) GetProfileByID(id uint) (*resellerdomain.Profile, error) {
	if id == 0 {
		return nil, nil
	}
	var profile resellerdomain.Profile
	if err := r.db.Preload("User", "deleted_at IS NULL").
		Where("reseller_profiles.id = ? AND reseller_profiles.deleted_at IS NULL", id).
		First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// GetProfileByUserID 按用户 ID 获取分销商资料。
func (r *Store) GetProfileByUserID(userID uint) (*resellerdomain.Profile, error) {
	if userID == 0 {
		return nil, nil
	}
	var profile resellerdomain.Profile
	if err := r.db.Preload("User", "deleted_at IS NULL").
		Where("user_id = ? AND reseller_profiles.deleted_at IS NULL", userID).
		First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &profile, nil
}

// UpdateProfile 更新分销商资料。
func (r *Store) UpdateProfile(profile *resellerdomain.Profile) error {
	if profile == nil || profile.ID == 0 {
		return errors.New("invalid reseller profile")
	}
	return r.db.Model(&resellerdomain.Profile{}).
		Where("id = ? AND deleted_at IS NULL", profile.ID).
		Select("*").
		Updates(profile).Error
}

// UpdateActiveProfileDefaultMarkup 在同一次写入中校验归属、状态和当前封顶。
func (r *Store) UpdateActiveProfileDefaultMarkup(userID uint, markup money.Amount) (bool, error) {
	result := r.db.Model(&resellerdomain.Profile{}).
		Where("user_id = ? AND status = ? AND deleted_at IS NULL", userID, resellerdomain.ProfileStatusActive).
		Where("(max_markup_percent = 0 OR max_markup_percent >= ?)", markup).
		Update("default_markup_percent", markup)
	return result.RowsAffected != 0, result.Error
}

// ListProfiles 分页列出分销商资料。
func (r *Store) ListProfiles(filter resellercontract.ProfileListFilter) ([]resellerdomain.Profile, int64, error) {
	rows := make([]resellerdomain.Profile, 0)
	query := r.db.Model(&resellerdomain.Profile{}).
		Preload("User", "deleted_at IS NULL").
		Where("reseller_profiles.deleted_at IS NULL")
	if filter.UserID > 0 {
		query = query.Where("reseller_profiles.user_id = ?", filter.UserID)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("reseller_profiles.status = ?", status)
	}
	if settlement := strings.TrimSpace(filter.SettlementStatus); settlement != "" {
		query = query.Where("reseller_profiles.settlement_status = ?", settlement)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + strings.ToLower(keyword) + "%"
		query = query.Joins("LEFT JOIN users ON users.id = reseller_profiles.user_id").
			Where("LOWER(users.email) LIKE ? OR LOWER(users.display_name) LIKE ? OR CAST(reseller_profiles.id AS TEXT) = ?", like, like, keyword)
	}
	if filter.CreatedFrom != nil {
		query = query.Where("reseller_profiles.created_at >= ?", *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		query = query.Where("reseller_profiles.created_at <= ?", *filter.CreatedTo)
	}
	var total int64
	if err := query.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := applyPagination(query.Session(&gorm.Session{}), filter.Page, filter.PageSize).
		Order("reseller_profiles.id DESC").
		Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// IsActiveRelatedAccount 判断用户是否为分销商已启用的关联账号。
func (r *Store) IsActiveRelatedAccount(resellerID uint, userID uint) (bool, error) {
	if resellerID == 0 || userID == 0 {
		return false, nil
	}
	var count int64
	if err := r.db.Model(&resellerdomain.RelatedAccount{}).
		Where("reseller_id = ? AND user_id = ? AND status = ? AND deleted_at IS NULL", resellerID, userID, resellerdomain.RelatedAccountStatusActive).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
