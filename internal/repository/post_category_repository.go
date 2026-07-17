package repository

import (
	"errors"

	"github.com/dujiao-next/internal/models"
	"gorm.io/gorm"
)

// PostCategoryRepository 文章分类数据访问接口
type PostCategoryRepository interface {
	ListAll(parentID *uint) ([]models.PostCategory, error)
	ListActive() ([]models.PostCategory, error)
	ListTree() ([]models.PostCategory, error)
	GetByID(id uint) (*models.PostCategory, error)
	Create(cat *models.PostCategory) error
	Update(cat *models.PostCategory) error
	UpdateActive(id uint, active bool) error
	Delete(id uint) error
	CountBySlug(slug string, excludeID *uint) (int64, error)
	CountChildren(parentID uint) (int64, error)
	CountPostsByCategory(categoryID uint) (int64, error)
}

// GormPostCategoryRepository GORM 实现
type GormPostCategoryRepository struct {
	db *gorm.DB
}

// NewPostCategoryRepository 创建文章分类仓库
func NewPostCategoryRepository(db *gorm.DB) *GormPostCategoryRepository {
	return &GormPostCategoryRepository{db: db}
}

func (r *GormPostCategoryRepository) ListAll(parentID *uint) ([]models.PostCategory, error) {
	var cats []models.PostCategory
	q := r.db.Order("sort_order ASC, id ASC")
	if parentID != nil {
		q = q.Where("parent_id = ?", *parentID)
	}
	if err := q.Find(&cats).Error; err != nil {
		return nil, err
	}
	return cats, nil
}

// ListActive 获取所有激活的分类（平铺，供公开接口用）
func (r *GormPostCategoryRepository) ListActive() ([]models.PostCategory, error) {
	var cats []models.PostCategory
	if err := r.db.Where("is_active = ?", true).Order("sort_order ASC, id ASC").Find(&cats).Error; err != nil {
		return nil, err
	}
	return cats, nil
}

// ListTree 获取全部分类树（含禁用，供后台用）
func (r *GormPostCategoryRepository) ListTree() ([]models.PostCategory, error) {
	var all []models.PostCategory
	if err := r.db.Order("sort_order ASC, id ASC").Find(&all).Error; err != nil {
		return nil, err
	}
	idMap := make(map[uint]*models.PostCategory)
	for i := range all {
		idMap[all[i].ID] = &all[i]
	}
	for i := range all {
		if all[i].ParentID != nil && *all[i].ParentID != 0 {
			if parent, ok := idMap[*all[i].ParentID]; ok {
				parent.Children = append(parent.Children, all[i])
			}
		}
	}
	roots := make([]models.PostCategory, 0, len(all))
	for i := range all {
		if all[i].ParentID == nil || *all[i].ParentID == 0 {
			roots = append(roots, *idMap[all[i].ID])
		}
	}
	return roots, nil
}

func (r *GormPostCategoryRepository) GetByID(id uint) (*models.PostCategory, error) {
	var cat models.PostCategory
	if err := r.db.First(&cat, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &cat, nil
}

func (r *GormPostCategoryRepository) Create(cat *models.PostCategory) error {
	return r.db.Create(cat).Error
}

func (r *GormPostCategoryRepository) Update(cat *models.PostCategory) error {
	return r.db.Save(cat).Error
}

func (r *GormPostCategoryRepository) UpdateActive(id uint, active bool) error {
	return r.db.Model(&models.PostCategory{}).Where("id = ?", id).Update("is_active", active).Error
}

func (r *GormPostCategoryRepository) CountPostsByCategory(categoryID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.Post{}).Where("category_id = ?", categoryID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormPostCategoryRepository) CountBySlug(slug string, excludeID *uint) (int64, error) {
	var count int64
	q := r.db.Model(&models.PostCategory{}).Where("slug = ?", slug)
	if excludeID != nil {
		q = q.Where("id != ?", *excludeID)
	}
	if err := q.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormPostCategoryRepository) CountChildren(parentID uint) (int64, error) {
	var count int64
	if err := r.db.Model(&models.PostCategory{}).Where("parent_id = ?", parentID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (r *GormPostCategoryRepository) Delete(id uint) error {
	return r.db.Delete(&models.PostCategory{}, id).Error
}
