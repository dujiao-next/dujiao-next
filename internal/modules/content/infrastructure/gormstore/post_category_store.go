package gormstore

import (
	"context"
	"errors"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
	"gorm.io/gorm"
)

// PostCategoryStore 使用 GORM 持久化文章分类。
type PostCategoryStore struct {
	db *gorm.DB
}

var _ contract.PostCategoryStore = (*PostCategoryStore)(nil)

// NewPostCategoryStore 创建文章分类持久化适配器。
func NewPostCategoryStore(db *gorm.DB) *PostCategoryStore {
	return &PostCategoryStore{db: db}
}

func (s *PostCategoryStore) ListAll(ctx context.Context, parentID *uint) ([]domain.PostCategory, error) {
	categories := make([]domain.PostCategory, 0)
	statement := withContext(s.db, ctx).Where("deleted_at IS NULL").Order("sort_order ASC, id ASC")
	if parentID != nil {
		statement = statement.Where("parent_id = ?", *parentID)
	}
	if err := statement.Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (s *PostCategoryStore) ListActive(ctx context.Context) ([]domain.PostCategory, error) {
	categories := make([]domain.PostCategory, 0)
	err := withContext(s.db, ctx).
		Where("is_active = ? AND deleted_at IS NULL", true).
		Order("sort_order ASC, id ASC").
		Find(&categories).Error
	return categories, err
}

func (s *PostCategoryStore) ListTree(ctx context.Context) ([]domain.PostCategory, error) {
	all := make([]domain.PostCategory, 0)
	if err := withContext(s.db, ctx).Where("deleted_at IS NULL").Order("sort_order ASC, id ASC").Find(&all).Error; err != nil {
		return nil, err
	}

	byID := make(map[uint]*domain.PostCategory, len(all))
	for index := range all {
		byID[all[index].ID] = &all[index]
	}
	for index := range all {
		if all[index].ParentID != nil && *all[index].ParentID != 0 {
			if parent, exists := byID[*all[index].ParentID]; exists {
				parent.Children = append(parent.Children, all[index])
			}
		}
	}

	roots := make([]domain.PostCategory, 0, len(all))
	for index := range all {
		if all[index].ParentID == nil || *all[index].ParentID == 0 {
			roots = append(roots, *byID[all[index].ID])
		}
	}
	return roots, nil
}

func (s *PostCategoryStore) GetByID(ctx context.Context, id uint) (*domain.PostCategory, error) {
	var category domain.PostCategory
	if err := withContext(s.db, ctx).Where("deleted_at IS NULL").First(&category, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

func (s *PostCategoryStore) Create(ctx context.Context, category *domain.PostCategory) error {
	return withContext(s.db, ctx).Create(category).Error
}

func (s *PostCategoryStore) Update(ctx context.Context, category *domain.PostCategory) error {
	return withContext(s.db, ctx).Save(category).Error
}

func (s *PostCategoryStore) UpdateActive(ctx context.Context, id uint, active bool) error {
	return withContext(s.db, ctx).Model(&domain.PostCategory{}).Where("id = ? AND deleted_at IS NULL", id).Update("is_active", active).Error
}

func (s *PostCategoryStore) Delete(ctx context.Context, id uint) error {
	return withContext(s.db, ctx).Model(&domain.PostCategory{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", s.db.NowFunc()).Error
}

func (s *PostCategoryStore) CountBySlug(ctx context.Context, slug string, excludeID *uint) (int64, error) {
	var count int64
	statement := withContext(s.db, ctx).Model(&domain.PostCategory{}).Where("slug = ? AND deleted_at IS NULL", slug)
	if excludeID != nil {
		statement = statement.Where("id != ?", *excludeID)
	}
	if err := statement.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PostCategoryStore) GetBySlug(ctx context.Context, slug string) (*domain.PostCategory, error) {
	var category domain.PostCategory
	if err := withContext(s.db, ctx).Where("slug = ? AND deleted_at IS NULL", slug).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &category, nil
}

func (s *PostCategoryStore) CountChildren(ctx context.Context, parentID uint) (int64, error) {
	var count int64
	err := withContext(s.db, ctx).Model(&domain.PostCategory{}).Where("parent_id = ? AND deleted_at IS NULL", parentID).Count(&count).Error
	return count, err
}

// CountPostsByCategory 统计该分类下仍然有效的博客文章数。
//
// 只统计 type=blog：公告（notice）没有分类功能，但历史数据/导入数据里可能残留
// 非空的 category_id。若把它们计入，后台会出现「分类下明明看不到文章，却提示
// 下面有文章不能删除」的僵局，因此这里按类型收口。
func (s *PostCategoryStore) CountPostsByCategory(ctx context.Context, categoryID uint) (int64, error) {
	var count int64
	err := withContext(s.db, ctx).Model(&domain.Post{}).
		Where("category_id = ? AND deleted_at IS NULL AND type = ?", categoryID, constants.PostTypeBlog).
		Count(&count).Error
	return count, err
}
