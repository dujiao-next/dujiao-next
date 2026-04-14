package repository

import (
	"errors"
	"strings"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
)

// PluginLicenseListFilter 插件授权中心列表过滤器。
type PluginLicenseListFilter struct {
	Page        int
	PageSize    int
	Keyword     string
	PluginID    string
	Status      string
	LicenseMode string
}

// PluginLicensePlatformRepository 在线授权中心平台仓储。
type PluginLicensePlatformRepository interface {
	ListLicenses(filter PluginLicenseListFilter) ([]models.PluginLicense, int64, error)
	GetLicenseByLicenseID(licenseID string) (*models.PluginLicense, error)
	GetLicenseByLicenseKey(licenseKey string) (*models.PluginLicense, error)
	SaveLicense(item *models.PluginLicense) error

	GetCatalogPluginByPluginID(pluginID string) (*models.PluginMarketCatalogPlugin, error)
	GetPluginPlan(id uint) (*models.PluginMarketPlan, error)
	GetPluginPlanByCode(pluginID, planCode string) (*models.PluginMarketPlan, error)
	ListPluginPlans(pluginID string) ([]models.PluginMarketPlan, error)

	ListBindingConflictLicenses(pluginID, domain, serverIP, excludeLicenseID string) ([]models.PluginLicense, error)

	ListLicenseActivations(licenseID string) ([]models.PluginLicenseActivation, error)
	GetLicenseActivationByToken(token string) (*models.PluginLicenseActivation, error)
	GetLicenseActivationByInstallID(licenseID, installID string) (*models.PluginLicenseActivation, error)
	GetLatestActiveActivation(licenseID string) (*models.PluginLicenseActivation, error)
	SaveLicenseActivation(item *models.PluginLicenseActivation) error

	ListRecentLicenseHeartbeats(licenseID string, limit int) ([]models.PluginLicenseHeartbeat, error)
	CreateLicenseHeartbeat(item *models.PluginLicenseHeartbeat) error
}

// GormPluginLicensePlatformRepository GORM 授权平台仓储实现。
type GormPluginLicensePlatformRepository struct {
	db *gorm.DB
}

// NewPluginLicensePlatformRepository 创建在线授权中心平台仓储。
func NewPluginLicensePlatformRepository(db *gorm.DB) *GormPluginLicensePlatformRepository {
	return &GormPluginLicensePlatformRepository{db: db}
}

func (r *GormPluginLicensePlatformRepository) ListLicenses(filter PluginLicenseListFilter) ([]models.PluginLicense, int64, error) {
	if r == nil || r.db == nil {
		return []models.PluginLicense{}, 0, nil
	}
	query := r.db.Model(&models.PluginLicense{})
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("license_id LIKE ? OR license_key LIKE ? OR plugin_id LIKE ? OR bound_domain LIKE ?", like, like, like, like)
	}
	if value := strings.TrimSpace(filter.PluginID); value != "" {
		query = query.Where("plugin_id = ?", value)
	}
	if value := strings.TrimSpace(filter.Status); value != "" {
		query = query.Where("status = ?", value)
	}
	if value := strings.TrimSpace(filter.LicenseMode); value != "" {
		query = query.Where("license_mode = ?", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := make([]models.PluginLicense, 0)
	if err := applyPagination(query.Order("updated_at DESC, id DESC"), filter.Page, filter.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormPluginLicensePlatformRepository) GetLicenseByLicenseID(licenseID string) (*models.PluginLicense, error) {
	if r == nil || r.db == nil || strings.TrimSpace(licenseID) == "" {
		return nil, nil
	}
	var item models.PluginLicense
	if err := r.db.Where("license_id = ?", strings.TrimSpace(licenseID)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginLicensePlatformRepository) GetLicenseByLicenseKey(licenseKey string) (*models.PluginLicense, error) {
	if r == nil || r.db == nil || strings.TrimSpace(licenseKey) == "" {
		return nil, nil
	}
	var item models.PluginLicense
	if err := r.db.Where("license_key = ?", strings.TrimSpace(licenseKey)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginLicensePlatformRepository) SaveLicense(item *models.PluginLicense) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginLicensePlatformRepository) GetCatalogPluginByPluginID(pluginID string) (*models.PluginMarketCatalogPlugin, error) {
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

func (r *GormPluginLicensePlatformRepository) GetPluginPlan(id uint) (*models.PluginMarketPlan, error) {
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

func (r *GormPluginLicensePlatformRepository) GetPluginPlanByCode(pluginID, planCode string) (*models.PluginMarketPlan, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" || strings.TrimSpace(planCode) == "" {
		return nil, nil
	}
	var item models.PluginMarketPlan
	if err := r.db.Where("plugin_id = ? AND plan_code = ?", strings.TrimSpace(pluginID), strings.TrimSpace(planCode)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginLicensePlatformRepository) ListPluginPlans(pluginID string) ([]models.PluginMarketPlan, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return []models.PluginMarketPlan{}, nil
	}
	items := make([]models.PluginMarketPlan, 0)
	if err := r.db.Where("plugin_id = ?", strings.TrimSpace(pluginID)).Order("sort_order DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginLicensePlatformRepository) ListBindingConflictLicenses(pluginID, domain, serverIP, excludeLicenseID string) ([]models.PluginLicense, error) {
	if r == nil || r.db == nil || strings.TrimSpace(pluginID) == "" {
		return []models.PluginLicense{}, nil
	}
	query := r.db.Model(&models.PluginLicense{}).Where("plugin_id = ?", strings.TrimSpace(pluginID))
	domain = strings.TrimSpace(domain)
	serverIP = strings.TrimSpace(serverIP)
	switch {
	case domain != "" && serverIP != "":
		query = query.Where("(bound_domain = ? OR bound_server_ip = ?)", domain, serverIP)
	case domain != "":
		query = query.Where("bound_domain = ?", domain)
	case serverIP != "":
		query = query.Where("bound_server_ip = ?", serverIP)
	default:
		return []models.PluginLicense{}, nil
	}
	if excluded := strings.TrimSpace(excludeLicenseID); excluded != "" {
		query = query.Where("license_id <> ?", excluded)
	}
	items := make([]models.PluginLicense, 0)
	if err := query.Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginLicensePlatformRepository) ListLicenseActivations(licenseID string) ([]models.PluginLicenseActivation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(licenseID) == "" {
		return []models.PluginLicenseActivation{}, nil
	}
	items := make([]models.PluginLicenseActivation, 0)
	if err := r.db.Where("license_id = ?", strings.TrimSpace(licenseID)).Order("updated_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginLicensePlatformRepository) GetLicenseActivationByToken(token string) (*models.PluginLicenseActivation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(token) == "" {
		return nil, nil
	}
	var item models.PluginLicenseActivation
	if err := r.db.Where("activation_token = ?", strings.TrimSpace(token)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginLicensePlatformRepository) GetLicenseActivationByInstallID(licenseID, installID string) (*models.PluginLicenseActivation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(licenseID) == "" || strings.TrimSpace(installID) == "" {
		return nil, nil
	}
	var item models.PluginLicenseActivation
	if err := r.db.Where("license_id = ? AND install_id = ?", strings.TrimSpace(licenseID), strings.TrimSpace(installID)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginLicensePlatformRepository) GetLatestActiveActivation(licenseID string) (*models.PluginLicenseActivation, error) {
	if r == nil || r.db == nil || strings.TrimSpace(licenseID) == "" {
		return nil, nil
	}
	var item models.PluginLicenseActivation
	if err := r.db.Where("license_id = ? AND status = ?", strings.TrimSpace(licenseID), "active").Order("updated_at DESC, id DESC").First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *GormPluginLicensePlatformRepository) SaveLicenseActivation(item *models.PluginLicenseActivation) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Save(item).Error
}

func (r *GormPluginLicensePlatformRepository) ListRecentLicenseHeartbeats(licenseID string, limit int) ([]models.PluginLicenseHeartbeat, error) {
	if r == nil || r.db == nil || strings.TrimSpace(licenseID) == "" {
		return []models.PluginLicenseHeartbeat{}, nil
	}
	if limit <= 0 {
		limit = 20
	}
	items := make([]models.PluginLicenseHeartbeat, 0, limit)
	if err := r.db.Where("license_id = ?", strings.TrimSpace(licenseID)).Order("created_at DESC, id DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *GormPluginLicensePlatformRepository) CreateLicenseHeartbeat(item *models.PluginLicenseHeartbeat) error {
	if r == nil || r.db == nil || item == nil {
		return nil
	}
	return r.db.Create(item).Error
}
