package repository

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// PluginMarketCenterPluginListFilter 在线插件中心插件过滤器。
type PluginMarketCenterPluginListFilter struct {
	Page        int
	PageSize    int
	Keyword     string
	Status      string
	PluginType  string
	BillingMode string
	PublisherID uint
	IsOfficial  *bool
	IsPublic    *bool
}

// PluginMarketPlatformRepository 在线插件中心平台仓储。
type PluginMarketPlatformRepository interface {
	ListMarketPublishers() ([]models.PluginMarketPublisher, error)
	ListMarketPublishersByIDs(ids []uint) ([]models.PluginMarketPublisher, error)
	GetMarketPublisher(id uint) (*models.PluginMarketPublisher, error)
	SaveMarketPublisher(item *models.PluginMarketPublisher) error
	DeleteMarketPublisher(id uint) error
	CountPluginsByPublisher(publisherID uint) (int64, error)

	ListCatalogPlugins(filter PluginMarketCenterPluginListFilter) ([]models.PluginMarketCatalogPlugin, int64, error)
	GetCatalogPluginByPluginID(pluginID string) (*models.PluginMarketCatalogPlugin, error)
	SaveCatalogPlugin(item *models.PluginMarketCatalogPlugin) error
	DeleteCatalogPlugin(pluginID string) error

	ListPluginVersions(pluginID string) ([]models.PluginMarketVersion, error)
	GetPluginVersion(id uint) (*models.PluginMarketVersion, error)
	SavePluginVersion(item *models.PluginMarketVersion) error
	DeletePluginVersion(id uint) error

	ListPluginPlans(pluginID string) ([]models.PluginMarketPlan, error)
	GetPluginPlan(id uint) (*models.PluginMarketPlan, error)
	SavePluginPlan(item *models.PluginMarketPlan) error
	DeletePluginPlan(id uint) error
}

// GormPluginMarketPlatformRepository GORM 实现。
type GormPluginMarketPlatformRepository struct {
	db *gorm.DB
}

// NewPluginMarketPlatformRepository 创建在线插件中心平台仓储。
func NewPluginMarketPlatformRepository(db *gorm.DB) *GormPluginMarketPlatformRepository {
	return &GormPluginMarketPlatformRepository{db: db}
}

func (r *GormPluginMarketPlatformRepository) ListMarketPublishers() ([]models.PluginMarketPublisher, error) {
	if r == nil || r.db == nil {
		return []models.PluginMarketPublisher{}, nil
	}
	items := make([]models.PluginMarketPublisher, 0)
	if err := r.db.Order("is_official DESC, name ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginMarketPlatformRepository) ListMarketPublishersByIDs(ids []uint) ([]models.PluginMarketPublisher, error) {
	if r == nil || r.db == nil || len(ids) == 0 {
		return []models.PluginMarketPublisher{}, nil
	}
	items := make([]models.PluginMarketPublisher, 0, len(ids))
	if err := r.db.Where("id IN ?", ids).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginMarketPlatformRepository) GetMarketPublisher(id uint) (*models.PluginMarketPublisher, error) {
	if r == nil || r.db == nil || id == 0 {
		return nil, nil
	}
	var item models.PluginMarketPublisher
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginMarketPlatformRepository) SaveMarketPublisher(item *models.PluginMarketPublisher) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginMarketPlatformRepository) DeleteMarketPublisher(id uint) error {
	if r == nil || r.db == nil || id == 0 {
		return nil
	}
	return r.db.Delete(&models.PluginMarketPublisher{}, id).Error
}

func (r *GormPluginMarketPlatformRepository) CountPluginsByPublisher(publisherID uint) (int64, error) {
	if r == nil || r.db == nil || publisherID == 0 {
		return 0, nil
	}
	var total int64
	if err := r.db.Model(&models.PluginMarketCatalogPlugin{}).Where("publisher_id = ?", publisherID).Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

func (r *GormPluginMarketPlatformRepository) ListCatalogPlugins(filter PluginMarketCenterPluginListFilter) ([]models.PluginMarketCatalogPlugin, int64, error) {
	if r == nil || r.db == nil {
		return []models.PluginMarketCatalogPlugin{}, 0, nil
	}
	query := r.db.Model(&models.PluginMarketCatalogPlugin{})
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("plugin_id LIKE ? OR slug LIKE ? OR name LIKE ?", like, like, like)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.PluginType); value != "" {
		query = query.Where("plugin_type = ?", value)
	}
	if value := strings.TrimSpace(filter.BillingMode); value != "" {
		query = query.Where("billing_mode = ?", value)
	}
	if filter.PublisherID > 0 {
		query = query.Where("publisher_id = ?", filter.PublisherID)
	}
	if filter.IsOfficial != nil {
		query = query.Where("is_official = ?", *filter.IsOfficial)
	}
	if filter.IsPublic != nil {
		query = query.Where("is_public = ?", *filter.IsPublic)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.PluginMarketCatalogPlugin, 0)
	if err := applyPagination(query.Order("updated_at DESC, id DESC"), filter.Page, filter.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormPluginMarketPlatformRepository) GetCatalogPluginByPluginID(pluginID string) (*models.PluginMarketCatalogPlugin, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil, nil
	}
	var item models.PluginMarketCatalogPlugin
	if err := r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginMarketPlatformRepository) SaveCatalogPlugin(item *models.PluginMarketCatalogPlugin) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginMarketPlatformRepository) DeleteCatalogPlugin(pluginID string) error {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	normalized := strings.TrimSpace(pluginID)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", normalized).Delete(&models.PluginMarketVersion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("plugin_id = ?", normalized).Delete(&models.PluginMarketPlan{}).Error; err != nil {
			return err
		}
		return tx.Where("plugin_id = ?", normalized).Delete(&models.PluginMarketCatalogPlugin{}).Error
	})
}

func (r *GormPluginMarketPlatformRepository) ListPluginVersions(pluginID string) ([]models.PluginMarketVersion, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return []models.PluginMarketVersion{}, nil
	}
	items := make([]models.PluginMarketVersion, 0)
	if err := r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).Order("published_at DESC, updated_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginMarketPlatformRepository) GetPluginVersion(id uint) (*models.PluginMarketVersion, error) {
	if r == nil || r.db == nil || id == 0 {
		return nil, nil
	}
	var item models.PluginMarketVersion
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginMarketPlatformRepository) SavePluginVersion(item *models.PluginMarketVersion) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginMarketPlatformRepository) DeletePluginVersion(id uint) error {
	if r == nil || r.db == nil || id == 0 {
		return nil
	}
	return r.db.Delete(&models.PluginMarketVersion{}, id).Error
}

func (r *GormPluginMarketPlatformRepository) ListPluginPlans(pluginID string) ([]models.PluginMarketPlan, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return []models.PluginMarketPlan{}, nil
	}
	items := make([]models.PluginMarketPlan, 0)
	if err := r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).Order("sort_order DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginMarketPlatformRepository) GetPluginPlan(id uint) (*models.PluginMarketPlan, error) {
	if r == nil || r.db == nil || id == 0 {
		return nil, nil
	}
	var item models.PluginMarketPlan
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginMarketPlatformRepository) SavePluginPlan(item *models.PluginMarketPlan) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginMarketPlatformRepository) DeletePluginPlan(id uint) error {
	if r == nil || r.db == nil || id == 0 {
		return nil
	}
	return r.db.Delete(&models.PluginMarketPlan{}, id).Error
}
