package service

import (
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
)

// PostService 文章业务服务
type PostService struct {
	repo         repository.PostRepository
	categoryRepo repository.PostCategoryRepository
}

// NewPostService 创建文章服务
func NewPostService(repo repository.PostRepository, categoryRepo repository.PostCategoryRepository) *PostService {
	return &PostService{repo: repo, categoryRepo: categoryRepo}
}

// CreatePostInput 创建/更新文章输入
type CreatePostInput struct {
	Slug        string
	Type        string
	TitleJSON   map[string]interface{}
	SummaryJSON map[string]interface{}
	ContentJSON map[string]interface{}
	Thumbnail   string
	IsPublished *bool
	ProductIDs  *[]uint // nil 表示不修改关联，非 nil（含空数组）表示替换关联
	CategoryID  *uint   // CategoryID（Update 时 nil=清除 Create 时 nil=无分类）
}

var allowedPostTypes = map[string]struct{}{
	constants.PostTypeBlog:   {},
	constants.PostTypeNotice: {},
}

// ListPublic 获取公开文章列表
func (s *PostService) ListPublic(postType, search string, page, pageSize int) ([]models.Post, int64, error) {
	filter := repository.PostListFilter{
		Page:          page,
		PageSize:      pageSize,
		Type:          postType,
		Search:        search,
		OnlyPublished: true,
		OrderBy:       "published_at DESC, created_at DESC",
	}
	return s.repo.List(filter)
}

// GetPublicBySlug 获取公开文章详情
func (s *PostService) GetPublicBySlug(slug string) (*models.Post, error) {
	post, err := s.repo.GetBySlug(slug, true)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrNotFound
	}
	return post, nil
}

// ListAdmin 获取后台文章列表
func (s *PostService) ListAdmin(postType, search string, page, pageSize int) ([]models.Post, int64, error) {
	filter := repository.PostListFilter{
		Page:     page,
		PageSize: pageSize,
		Type:     postType,
		Search:   search,
		OrderBy:  "created_at DESC",
	}
	return s.repo.List(filter)
}

// Create 创建文章
func (s *PostService) Create(input CreatePostInput) (*models.Post, error) {
	if !isAllowedPostType(input.Type) {
		return nil, ErrInvalidPostType
	}
	normalizedCategoryID := normalizePostCategoryID(input.CategoryID)
	if err := s.validateCategoryAssignment(input.Type, normalizedCategoryID, nil); err != nil {
		return nil, err
	}

	count, err := s.repo.CountBySlug(input.Slug, nil)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}

	isPublished := false
	if input.IsPublished != nil {
		isPublished = *input.IsPublished
	}

	post := models.Post{
		Slug:        input.Slug,
		Type:        input.Type,
		TitleJSON:   models.JSON(input.TitleJSON),
		SummaryJSON: models.JSON(input.SummaryJSON),
		ContentJSON: models.JSON(input.ContentJSON),
		Thumbnail:   input.Thumbnail,
		IsPublished: isPublished,
		CategoryID:  normalizedCategoryID,
	}
	if isPublished {
		now := time.Now()
		post.PublishedAt = &now
	}

	if err := s.repo.Create(&post); err != nil {
		return nil, err
	}
	if input.ProductIDs != nil {
		if err := s.repo.SetRelatedProductIDs(post.ID, *input.ProductIDs); err != nil {
			return nil, err
		}
	}
	return &post, nil
}

// Update 更新文章
func (s *PostService) Update(id string, input CreatePostInput) (*models.Post, error) {
	if !isAllowedPostType(input.Type) {
		return nil, ErrInvalidPostType
	}

	post, err := s.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if post == nil {
		return nil, ErrNotFound
	}
	normalizedCategoryID := normalizePostCategoryID(input.CategoryID)
	if err := s.validateCategoryAssignment(input.Type, normalizedCategoryID, post.CategoryID); err != nil {
		return nil, err
	}

	count, err := s.repo.CountBySlug(input.Slug, &id)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrSlugExists
	}

	post.Slug = input.Slug
	post.Type = input.Type
	post.TitleJSON = models.JSON(input.TitleJSON)
	post.SummaryJSON = models.JSON(input.SummaryJSON)
	post.ContentJSON = models.JSON(input.ContentJSON)
	post.Thumbnail = input.Thumbnail
	post.CategoryID = normalizedCategoryID
	if input.IsPublished != nil {
		wasPublished := post.IsPublished
		post.IsPublished = *input.IsPublished
		if *input.IsPublished && !wasPublished && post.PublishedAt == nil {
			now := time.Now()
			post.PublishedAt = &now
		}
	}

	if err := s.repo.Update(post); err != nil {
		return nil, err
	}
	if input.ProductIDs != nil {
		if err := s.repo.SetRelatedProductIDs(post.ID, *input.ProductIDs); err != nil {
			return nil, err
		}
	}
	return post, nil
}

// GetRelatedProductIDs 获取文章关联商品 ID 列表（后台编辑回填）
func (s *PostService) GetRelatedProductIDs(postID uint) ([]uint, error) {
	return s.repo.GetRelatedProductIDs(postID)
}

// ListRelatedProducts 获取文章关联的商品列表（公开博客详情用）
func (s *PostService) ListRelatedProducts(postID uint) ([]models.Product, error) {
	return s.repo.ListRelatedProducts(postID)
}

// ListPostsForProduct 获取与某商品关联的已发布博客列表（公开商品详情用，仅 blog 类型）
func (s *PostService) ListPostsForProduct(productID uint, limit int) ([]models.Post, error) {
	return s.repo.ListPostsForProduct(productID, constants.PostTypeBlog, true, limit)
}

// Delete 删除文章
func (s *PostService) Delete(id string) error {
	post, err := s.repo.GetByID(id)
	if err != nil {
		return err
	}
	if post == nil {
		return ErrNotFound
	}
	return s.repo.Delete(id)
}

func isAllowedPostType(postType string) bool {
	_, ok := allowedPostTypes[postType]
	return ok
}

func (s *PostService) validateCategoryAssignment(postType string, categoryID *uint, currentCategoryID *uint) error {
	if postType == constants.PostTypeNotice {
		if categoryID != nil && *categoryID > 0 {
			return ErrPostNoticeCategoryUnsupported
		}
		return nil
	}
	if categoryID == nil || *categoryID == 0 {
		return nil
	}
	if s.categoryRepo == nil {
		return ErrPostCategoryInvalid
	}

	category, err := s.categoryRepo.GetByID(*categoryID)
	if err != nil {
		return err
	}
	if category == nil {
		return ErrPostCategoryInvalid
	}
	// 禁用分类不可新挂载；保留文章已有分类（后来才被禁用）不受影响
	if !category.IsActive && !sameOptionalUint(currentCategoryID, categoryID) {
		return ErrPostCategoryInvalid
	}

	childCount, err := s.categoryRepo.CountChildren(*categoryID)
	if err != nil {
		return err
	}
	if childCount > 0 && !sameOptionalUint(currentCategoryID, categoryID) {
		return ErrPostCategoryInvalid
	}

	return nil
}

func normalizePostCategoryID(categoryID *uint) *uint {
	if categoryID != nil && *categoryID == 0 {
		return nil
	}
	return categoryID
}

func sameOptionalUint(left *uint, right *uint) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
