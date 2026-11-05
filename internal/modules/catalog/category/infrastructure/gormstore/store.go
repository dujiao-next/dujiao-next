package gormstore

import (
	"errors"
	"strconv"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorycontract "github.com/dujiao-next/internal/modules/catalog/category/contract"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"gorm.io/gorm"
)

// CategoryStore 是 Catalog 分类端口的 GORM 实现。
type CategoryStore struct {
	db *gorm.DB
}

var _ categorycontract.Repository = (*CategoryStore)(nil)

// NewCategoryStore 创建分类存储。
func NewCategoryStore(db *gorm.DB) *CategoryStore {
	return &CategoryStore{db: db}
}

// List 分类列表
func (r *CategoryStore) List() ([]categorydomain.Category, error) {
	var categories []categorydomain.Category
	if err := r.db.Where("deleted_at IS NULL").Order("sort_order DESC, id ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// ListActive 启用的分类列表
func (r *CategoryStore) ListActive() ([]categorydomain.Category, error) {
	var categories []categorydomain.Category
	if err := r.db.Where("deleted_at IS NULL AND is_active = ?", true).Order("sort_order DESC, id ASC").Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// GetByID 根据 ID 获取分类
func (r *CategoryStore) GetByID(id string) (*categorydomain.Category, error) {
	pk, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, nil
	}
	var category categorydomain.Category
	if err := r.db.Where("deleted_at IS NULL").First(&category, pk).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

// Create 创建分类
func (r *CategoryStore) Create(category *categorydomain.Category) error {
	return r.db.Create(category).Error
}

// Update 更新分类
func (r *CategoryStore) Update(category *categorydomain.Category) error {
	return r.db.Save(category).Error
}

// UpdateActive 更新启用状态
func (r *CategoryStore) UpdateActive(id string, active bool) error {
	return r.db.Model(&categorydomain.Category{}).Where("id = ?", id).Update("is_active", active).Error
}

// Delete 删除分类
func (r *CategoryStore) Delete(id string) error {
	return r.db.Model(&categorydomain.Category{}).Where("id = ? AND deleted_at IS NULL", id).Update("deleted_at", time.Now()).Error
}

// CountBySlug 统计 slug 数量
func (r *CategoryStore) CountBySlug(slug string, excludeID *string) (int64, error) {
	var count int64
	query := r.db.Model(&categorydomain.Category{}).Where("deleted_at IS NULL AND slug = ?", slug)
	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountChildren 统计某分类的子分类数量
func (r *CategoryStore) CountChildren(categoryID string) (int64, error) {
	var count int64
	if err := r.db.Model(&categorydomain.Category{}).Where("deleted_at IS NULL AND parent_id = ?", categoryID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// CountProducts 统计某分类下商品数
func (r *CategoryStore) CountProducts(categoryID string) (int64, error) {
	var count int64
	if err := r.db.Model(&productdomain.Product{}).Where("deleted_at IS NULL AND category_id = ?", categoryID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// GetBySlug 根据 slug 获取分类
func (r *CategoryStore) GetBySlug(slug string) (*categorydomain.Category, error) {
	var category categorydomain.Category
	if err := r.db.Where("deleted_at IS NULL AND slug = ?", slug).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

// GetBySlugUnscoped 根据 slug 获取分类，包含软删除记录。
func (r *CategoryStore) GetBySlugUnscoped(slug string) (*categorydomain.Category, error) {
	var category categorydomain.Category
	if err := r.db.Unscoped().Where("slug = ?", slug).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

// Restore 恢复软删除分类并刷新展示信息。
func (r *CategoryStore) Restore(category *categorydomain.Category) error {
	return r.db.Unscoped().Model(&categorydomain.Category{}).Where("id = ?", category.ID).Updates(map[string]interface{}{
		"parent_id":  category.ParentID,
		"name_json":  category.NameJSON,
		"icon":       category.Icon,
		"sort_order": category.SortOrder,
		"is_active":  category.IsActive,
		"deleted_at": nil,
	}).Error
}

// CountActiveProducts 统计某分类下已上架商品数
func (r *CategoryStore) CountActiveProducts(categoryID string) (int64, error) {
	var count int64
	if err := r.db.Model(&productdomain.Product{}).Where("deleted_at IS NULL AND category_id = ? AND is_active = ?", categoryID, true).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
