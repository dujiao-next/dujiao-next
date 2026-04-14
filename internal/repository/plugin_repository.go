package repository

import (
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// PluginListFilter 插件列表过滤器。
type PluginListFilter struct {
	Page      int
	PageSize  int
	Keyword   string
	Type      string
	Status    string
	Source    string
	IsEnabled *bool
}

// PluginLogListFilter 插件日志过滤器。
type PluginLogListFilter struct {
	Page      int
	PageSize  int
	PluginID  string
	Level     string
	EventType string
}

// PluginMarketListFilter 在线插件库过滤器。
type PluginMarketListFilter struct {
	Page       int
	PageSize   int
	RegistryID string
	Keyword    string
	Type       string
	PluginID   string
}

// PluginRepository 插件中心仓储。
type PluginRepository interface {
	GetPlugin(id string) (*models.Plugin, error)
	SavePlugin(item *models.Plugin) error
	ListPlugins(filter PluginListFilter) ([]models.Plugin, int64, error)
	DeletePlugin(id string) error
	GetPluginVersion(pluginID, version string) (*models.PluginVersion, error)
	GetActivePluginVersion(pluginID string) (*models.PluginVersion, error)
	GetLatestPluginVersion(pluginID string) (*models.PluginVersion, error)
	ListPluginVersions(pluginID string) ([]models.PluginVersion, error)
	SavePluginVersion(item *models.PluginVersion) error
	DeletePluginVersions(pluginID string) error
	CreateRuntimeLog(item *models.PluginRuntimeLog) error
	ListRuntimeLogs(filter PluginLogListFilter) ([]models.PluginRuntimeLog, int64, error)
	ReplaceRouteRegistries(pluginID string, items []models.PluginRouteRegistry) error
	ReplacePageRegistries(pluginID string, items []models.PluginPageRegistry) error
	ReplaceEventSubscriptions(pluginID string, items []models.PluginEventSubscription) error
	ListRouteRegistries(pluginID string) ([]models.PluginRouteRegistry, error)
	ListPageRegistries(pluginID string) ([]models.PluginPageRegistry, error)
	ListEventSubscriptions(pluginID string) ([]models.PluginEventSubscription, error)
	UpsertMarketRegistry(item *models.PluginMarketRegistry) error
	ListMarketRegistries() ([]models.PluginMarketRegistry, error)
	GetMarketRegistry(id string) (*models.PluginMarketRegistry, error)
	ReplaceMarketCache(registryID string, items []models.PluginMarketCache) error
	ListMarketItems(filter PluginMarketListFilter) ([]models.PluginMarketCache, int64, error)
	GetMarketItem(registryID, pluginID, version string) (*models.PluginMarketCache, error)
	ListMarketVersions(registryID, pluginID string) ([]models.PluginMarketCache, error)
}

// GormPluginRepository GORM 实现。
type GormPluginRepository struct {
	db *gorm.DB
}

// NewPluginRepository 创建插件中心仓储。
func NewPluginRepository(db *gorm.DB) *GormPluginRepository {
	return &GormPluginRepository{db: db}
}

func (r *GormPluginRepository) GetPlugin(id string) (*models.Plugin, error) {
	if r == nil || r.db == nil || strings.TrimSpace(id) == "" {
		return nil, nil
	}
	var item models.Plugin
	if err := r.db.First(&item, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginRepository) SavePlugin(item *models.Plugin) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginRepository) ListPlugins(filter PluginListFilter) ([]models.Plugin, int64, error) {
	if r == nil || r.db == nil {
		return []models.Plugin{}, 0, nil
	}
	query := r.db.Model(&models.Plugin{})
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("id LIKE ? OR name LIKE ? OR author LIKE ?", like, like, like)
	}
	if value := strings.TrimSpace(filter.Type); value != "" {
		query = query.Where("type = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.Source); value != "" {
		query = query.Where("source = ?", value)
	}
	if filter.IsEnabled != nil {
		query = query.Where("is_enabled = ?", *filter.IsEnabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.Plugin, 0)
	if err := applyPagination(query.Order("updated_at DESC"), filter.Page, filter.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormPluginRepository) DeletePlugin(id string) error {
	if r == nil || r.db == nil || strings.TrimSpace(id) == "" {
		return nil
	}
	pluginID := strings.TrimSpace(id)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginVersion{}).Error; err != nil {
			return err
		}
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginRuntimeLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginRouteRegistry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginPageRegistry{}).Error; err != nil {
			return err
		}
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginEventSubscription{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Plugin{}, "id = ?", pluginID).Error
	})
}

func (r *GormPluginRepository) GetPluginVersion(pluginID, version string) (*models.PluginVersion, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" || strings.TrimSpace(version) == "" {
		return nil, nil
	}
	var item models.PluginVersion
	if err := r.db.Where("plugin_id = ? AND version = ?", strings.TrimSpace(pluginID), strings.TrimSpace(version)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginRepository) GetActivePluginVersion(pluginID string) (*models.PluginVersion, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil, nil
	}
	var item models.PluginVersion
	if err := r.db.Where("plugin_id = ? AND is_active = ?", strings.TrimSpace(pluginID), true).Order("id DESC").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginRepository) GetLatestPluginVersion(pluginID string) (*models.PluginVersion, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil, nil
	}
	var item models.PluginVersion
	if err := r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).Order("id DESC").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginRepository) ListPluginVersions(pluginID string) ([]models.PluginVersion, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return []models.PluginVersion{}, nil
	}
	items := make([]models.PluginVersion, 0)
	if err := r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginRepository) SavePluginVersion(item *models.PluginVersion) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginRepository) DeletePluginVersions(pluginID string) error {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	return r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).Delete(&models.PluginVersion{}).Error
}

func (r *GormPluginRepository) CreateRuntimeLog(item *models.PluginRuntimeLog) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Create(item).Error
}

func (r *GormPluginRepository) ListRuntimeLogs(filter PluginLogListFilter) ([]models.PluginRuntimeLog, int64, error) {
	if r == nil || r.db == nil {
		return []models.PluginRuntimeLog{}, 0, nil
	}
	query := r.db.Model(&models.PluginRuntimeLog{})
	if value := strings.TrimSpace(filter.PluginID); value != "" {
		query = query.Where("plugin_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Level); value != "" {
		query = query.Where("level = ?", value)
	}
	if value := strings.TrimSpace(filter.EventType); value != "" {
		query = query.Where("event_type = ?", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.PluginRuntimeLog, 0)
	if err := applyPagination(query.Order("id DESC"), filter.Page, filter.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormPluginRepository) ReplaceRouteRegistries(pluginID string, items []models.PluginRouteRegistry) error {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginRouteRegistry{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for index := range items {
			items[index].PluginID = pluginID
		}
		return tx.Create(&items).Error
	})
}

func (r *GormPluginRepository) ReplacePageRegistries(pluginID string, items []models.PluginPageRegistry) error {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginPageRegistry{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for index := range items {
			items[index].PluginID = pluginID
		}
		return tx.Create(&items).Error
	})
}

func (r *GormPluginRepository) ReplaceEventSubscriptions(pluginID string, items []models.PluginEventSubscription) error {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("plugin_id = ?", pluginID).Delete(&models.PluginEventSubscription{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for index := range items {
			items[index].PluginID = pluginID
		}
		return tx.Create(&items).Error
	})
}

func (r *GormPluginRepository) ListRouteRegistries(pluginID string) ([]models.PluginRouteRegistry, error) {
	items := make([]models.PluginRouteRegistry, 0)
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return items, nil
	}
	if err := r.db.Where("plugin_id = ?", pluginID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginRepository) ListPageRegistries(pluginID string) ([]models.PluginPageRegistry, error) {
	items := make([]models.PluginPageRegistry, 0)
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return items, nil
	}
	if err := r.db.Where("plugin_id = ?", pluginID).Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginRepository) ListEventSubscriptions(pluginID string) ([]models.PluginEventSubscription, error) {
	items := make([]models.PluginEventSubscription, 0)
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return items, nil
	}
	if err := r.db.Where("plugin_id = ?", pluginID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginRepository) UpsertMarketRegistry(item *models.PluginMarketRegistry) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginRepository) ListMarketRegistries() ([]models.PluginMarketRegistry, error) {
	items := make([]models.PluginMarketRegistry, 0)
	if r == nil || r.db == nil {
		return items, nil
	}
	if err := r.db.Order("sort_order ASC, id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginRepository) GetMarketRegistry(id string) (*models.PluginMarketRegistry, error) {
	if r == nil || r.db == nil || strings.TrimSpace(id) == "" {
		return nil, nil
	}
	var item models.PluginMarketRegistry
	if err := r.db.First(&item, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginRepository) ReplaceMarketCache(registryID string, items []models.PluginMarketCache) error {
	if r == nil || r.db == nil || strings.TrimSpace(registryID) == "" {
		return nil
	}
	now := time.Now()
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("registry_id = ?", registryID).Delete(&models.PluginMarketCache{}).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		for index := range items {
			items[index].RegistryID = registryID
			if items[index].SyncedAt.IsZero() {
				items[index].SyncedAt = now
			}
		}
		return tx.Create(&items).Error
	})
}

func (r *GormPluginRepository) ListMarketItems(filter PluginMarketListFilter) ([]models.PluginMarketCache, int64, error) {
	if r == nil || r.db == nil {
		return []models.PluginMarketCache{}, 0, nil
	}
	query := r.db.Model(&models.PluginMarketCache{})
	if value := strings.TrimSpace(filter.RegistryID); value != "" {
		query = query.Where("registry_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Type); value != "" {
		query = query.Where("type = ?", value)
	}
	if value := strings.TrimSpace(filter.PluginID); value != "" {
		query = query.Where("plugin_id = ?", value)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("plugin_id LIKE ? OR name LIKE ? OR author LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.PluginMarketCache, 0)
	if err := applyPagination(query.Order("registry_id ASC, plugin_id ASC, created_at DESC"), filter.Page, filter.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormPluginRepository) GetMarketItem(registryID, pluginID, version string) (*models.PluginMarketCache, error) {
	if r == nil || r.db == nil || strings.TrimSpace(registryID) == "" || strings.TrimSpace(pluginID) == "" {
		return nil, nil
	}
	query := r.db.Where("registry_id = ? AND plugin_id = ?", strings.TrimSpace(registryID), strings.TrimSpace(pluginID))
	if strings.TrimSpace(version) != "" {
		query = query.Where("version = ?", strings.TrimSpace(version))
	}
	var item models.PluginMarketCache
	if err := query.Order("id DESC").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginRepository) ListMarketVersions(registryID, pluginID string) ([]models.PluginMarketCache, error) {
	items := make([]models.PluginMarketCache, 0)
	if r == nil || r.db == nil || strings.TrimSpace(registryID) == "" || strings.TrimSpace(pluginID) == "" {
		return items, nil
	}
	if err := r.db.Where("registry_id = ? AND plugin_id = ?", strings.TrimSpace(registryID), strings.TrimSpace(pluginID)).Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
