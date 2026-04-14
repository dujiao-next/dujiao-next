package repository

import (
	"errors"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// SettingRepository 设置数据访问接口
type SettingRepository interface {
	GetByKey(key string) (*models.Setting, error)
	Upsert(key string, value models.JSON) (*models.Setting, error)
	DeleteByKey(key string) error
}

// GormSettingRepository GORM 实现
type GormSettingRepository struct {
	db *gorm.DB
}

// NewSettingRepository 创建设置仓库
func NewSettingRepository(db *gorm.DB) *GormSettingRepository {
	return &GormSettingRepository{db: db}
}

// GetByKey 获取设置
func (r *GormSettingRepository) GetByKey(key string) (*models.Setting, error) {
	var setting models.Setting
	if err := r.db.Where("key = ?", key).First(&setting).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &setting, nil
}

// Upsert 更新或创建设置
func (r *GormSettingRepository) Upsert(key string, value models.JSON) (*models.Setting, error) {
	setting, err := r.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if setting == nil {
		setting = &models.Setting{
			Key:       key,
			ValueJSON: value,
		}
		if err := r.db.Create(setting).Error; err != nil {
			return nil, err
		}
		return setting, nil
	}

	setting.ValueJSON = value
	if err := r.db.Save(setting).Error; err != nil {
		return nil, err
	}
	return setting, nil
}

// DeleteByKey 删除指定设置。
func (r *GormSettingRepository) DeleteByKey(key string) error {
	if r == nil || r.db == nil || key == "" {
		return nil
	}
	return r.db.Delete(&models.Setting{}, "key = ?", key).Error
}
