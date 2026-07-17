package service

import (
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

// PostCategoryService 文章分类业务服务
type PostCategoryService struct {
	repo repository.PostCategoryRepository
}

// NewPostCategoryService 创建文章分类服务
func NewPostCategoryService(repo repository.PostCategoryRepository) *PostCategoryService {
	return &PostCategoryService{repo: repo}
}

// CreatePostCategoryInput 创建/更新文章分类输入
type CreatePostCategoryInput struct {
	NameJSON  models.JSON
	Slug      string
	ParentID  *uint
	SortOrder int
	Icon      string
}

// ListAll 获取所有文章分类
func (s *PostCategoryService) ListAll(parentID *uint) ([]models.PostCategory, error) {
	return s.repo.ListAll(parentID)
}

// ListTree 获取全部分类树（含禁用，供后台用）
func (s *PostCategoryService) ListTree() ([]models.PostCategory, error) {
	return s.repo.ListTree()
}

// GetByID 获取单个分类
func (s *PostCategoryService) GetByID(id uint) (*models.PostCategory, error) {
	cat, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cat == nil {
		return nil, ErrNotFound
	}
	return cat, nil
}

// Create 创建文章分类
func (s *PostCategoryService) Create(input CreatePostCategoryInput) (*models.PostCategory, error) {
	pid := normalizeParentID(input.ParentID)
	if err := s.validateParent(nil, pid); err != nil {
		return nil, err
	}

	count, err := s.repo.CountBySlug(input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}

	cat := &models.PostCategory{
		NameJSON:  input.NameJSON,
		Slug:      input.Slug,
		ParentID:  pid,
		SortOrder: input.SortOrder,
		Icon:      input.Icon,
		IsActive:  true,
	}
	if err := s.repo.Create(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

// Update 更新文章分类
func (s *PostCategoryService) Update(id uint, input CreatePostCategoryInput) (*models.PostCategory, error) {
	cat, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	pid := normalizeParentID(input.ParentID)
	if err := s.validateParent(cat, pid); err != nil {
		return nil, err
	}

	count, err := s.repo.CountBySlug(input.Slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}

	if input.NameJSON != nil {
		cat.NameJSON = input.NameJSON
	}
	if input.Slug != "" {
		cat.Slug = input.Slug
	}
	cat.ParentID = pid
	cat.SortOrder = input.SortOrder
	cat.Icon = input.Icon

	if err := s.repo.Update(cat); err != nil {
		return nil, err
	}
	return cat, nil
}

// Delete 删除文章分类（软删除）。先查存在性，再校验子分类和文章数。
func (s *PostCategoryService) Delete(id uint) error {
	if _, err := s.GetByID(id); err != nil {
		return err
	}

	childCount, err := s.repo.CountChildren(id)
	if err != nil {
		return err
	}
	if childCount > 0 {
		return ErrCategoryInUse
	}

	postCount, err := s.repo.CountPostsByCategory(id)
	if err != nil {
		return err
	}
	if postCount > 0 {
		return ErrCategoryInUse
	}

	return s.repo.Delete(id)
}

// ListActive 获取所有激活的分类（公开接口用）
func (s *PostCategoryService) ListActive() ([]models.PostCategory, error) {
	return s.repo.ListActive()
}

// SetActive 切换分类启用状态
func (s *PostCategoryService) SetActive(id uint, active bool) (*models.PostCategory, error) {
	cat, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if cat.IsActive == active {
		return cat, nil
	}
	if err := s.repo.UpdateActive(id, active); err != nil {
		return nil, err
	}
	cat.IsActive = active
	return cat, nil
}

// validateParent 校验父分类合法性（与商品分类 validateParent 逻辑一致）
func (s *PostCategoryService) validateParent(cat *models.PostCategory, parentID *uint) error {
	if parentID == nil || *parentID == 0 {
		return nil
	}

	if cat != nil && cat.ID == *parentID {
		return ErrCategoryParentInvalid
	}

	parent, err := s.repo.GetByID(*parentID)
	if err != nil {
		return err
	}
	if parent == nil || (parent.ParentID != nil && *parent.ParentID != 0) {
		return ErrCategoryParentInvalid
	}

	if cat != nil && (cat.ParentID == nil || *cat.ParentID == 0) {
		childCount, err := s.repo.CountChildren(cat.ID)
		if err != nil {
			return err
		}
		if childCount > 0 {
			return ErrCategoryParentInvalid
		}
	}

	return nil
}

func normalizeParentID(p *uint) *uint {
	if p != nil && *p == 0 {
		return nil
	}
	return p
}
