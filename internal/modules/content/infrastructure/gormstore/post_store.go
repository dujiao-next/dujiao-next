package gormstore

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/modules/content/contract"
	"github.com/dujiao-next/internal/modules/content/domain"
	"gorm.io/gorm"
)

// PostStore 使用 GORM 持久化文章和文章商品关联。
type PostStore struct {
	db *gorm.DB
}

var (
	_ contract.PostStore                = (*PostStore)(nil)
	_ contract.PostProductRelationStore = (*PostStore)(nil)
)

// NewPostStore 创建文章持久化适配器。
func NewPostStore(db *gorm.DB) *PostStore {
	return &PostStore{db: db}
}

// WithinPostWriteTransaction 在同一个数据库事务中提供文章与商品关联 Store。
func (s *PostStore) WithinPostWriteTransaction(
	ctx context.Context,
	operation func(contract.PostStore, contract.PostProductRelationStore) error,
) error {
	return withContext(s.db, ctx).Transaction(func(tx *gorm.DB) error {
		transactionStore := NewPostStore(tx)
		return operation(transactionStore, transactionStore)
	})
}

func (s *PostStore) List(ctx context.Context, query contract.PostQuery) ([]domain.Post, int64, error) {
	posts := make([]domain.Post, 0)
	db := withContext(s.db, ctx)
	statement := db.Model(&domain.Post{}).Where("posts.deleted_at IS NULL")

	if query.OnlyPublished {
		statement = statement.Where("is_published = ?", true)
	}
	if query.Type != "" {
		statement = statement.Where("type = ?", query.Type)
	}
	if query.CategoryID != "" {
		statement = statement.Where("posts.category_id = ?", query.CategoryID)
	}
	// 三态发布状态筛选：OnlyPublished 已强制 is_published = true，此时不再叠加
	if query.IsPublished != nil && !query.OnlyPublished {
		statement = statement.Where("posts.is_published = ?", *query.IsPublished)
	}
	if search := strings.TrimSpace(query.Search); search != "" {
		like := "%" + search + "%"
		condition, argCount := buildLocalizedLikeCondition(db, []string{"slug"}, []string{"title_json"})
		statement = statement.Where(condition, repeatLikeArgs(like, argCount)...)
	}

	var total int64
	if err := statement.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	statement = applyPagination(statement, query.Page, query.PageSize)
	orderBy := "created_at DESC"
	if query.Order == contract.PostOrderPublishedDesc {
		orderBy = "published_at DESC, created_at DESC"
	}
	if err := statement.Order(orderBy).Find(&posts).Error; err != nil {
		return nil, 0, err
	}
	return posts, total, nil
}

func (s *PostStore) GetBySlug(ctx context.Context, slug string, onlyPublished bool) (*domain.Post, error) {
	statement := withContext(s.db, ctx).Where("slug = ? AND deleted_at IS NULL", slug)
	if onlyPublished {
		statement = statement.Where("is_published = ?", true)
	}

	var post domain.Post
	if err := statement.First(&post).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (s *PostStore) GetByID(ctx context.Context, id string) (*domain.Post, error) {
	pk, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, nil
	}
	var post domain.Post
	if err := withContext(s.db, ctx).Where("deleted_at IS NULL").First(&post, pk).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &post, nil
}

func (s *PostStore) Create(ctx context.Context, post *domain.Post) error {
	return withContext(s.db, ctx).Create(post).Error
}

func (s *PostStore) Update(ctx context.Context, post *domain.Post) error {
	return withContext(s.db, ctx).Save(post).Error
}

// UpdatePublished 只写发布状态，publishedAt 非空时一并回填发布时间。
// 文章的多语言字段按 JSON 存储，读回后可能为空值，因此局部更新不做整行回写。
func (s *PostStore) UpdatePublished(ctx context.Context, id string, published bool, publishedAt *time.Time) error {
	updates := map[string]interface{}{"is_published": published}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	return withContext(s.db, ctx).Model(&domain.Post{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updates).Error
}

// UpdateCategory 只写分类列，categoryID 为 nil 表示移出分类。
func (s *PostStore) UpdateCategory(ctx context.Context, id string, categoryID *uint) error {
	return withContext(s.db, ctx).Model(&domain.Post{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("category_id", categoryID).Error
}

func (s *PostStore) Delete(ctx context.Context, id string) error {
	return withContext(s.db, ctx).Model(&domain.Post{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("deleted_at", s.db.NowFunc()).Error
}

func (s *PostStore) CountBySlug(ctx context.Context, slug string, excludeID *string) (int64, error) {
	var count int64
	statement := withContext(s.db, ctx).Model(&domain.Post{}).Where("slug = ? AND deleted_at IS NULL", slug)
	if excludeID != nil {
		statement = statement.Where("id != ?", *excludeID)
	}
	if err := statement.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *PostStore) GetRelatedProductIDs(ctx context.Context, postID uint) ([]uint, error) {
	var ids []uint
	err := withContext(s.db, ctx).Model(&domain.PostProduct{}).
		Where("post_id = ?", postID).
		Order("sort ASC, id ASC").
		Pluck("product_id", &ids).Error
	return ids, err
}

func (s *PostStore) SetRelatedProductIDs(ctx context.Context, postID uint, productIDs []uint) error {
	return withContext(s.db, ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("post_id = ?", postID).Delete(&domain.PostProduct{}).Error; err != nil {
			return err
		}
		if len(productIDs) == 0 {
			return nil
		}

		seen := make(map[uint]struct{}, len(productIDs))
		records := make([]domain.PostProduct, 0, len(productIDs))
		for index, productID := range productIDs {
			if productID == 0 {
				continue
			}
			if _, exists := seen[productID]; exists {
				continue
			}
			seen[productID] = struct{}{}
			records = append(records, domain.PostProduct{
				PostID:    postID,
				ProductID: productID,
				Sort:      index,
			})
		}
		if len(records) == 0 {
			return nil
		}
		return tx.Create(&records).Error
	})
}

func (s *PostStore) ListRelatedProducts(ctx context.Context, postID uint) ([]contract.RelatedProduct, error) {
	var products []productdomain.Product
	err := withContext(s.db, ctx).
		Joins("INNER JOIN post_products pp ON pp.product_id = products.id").
		Where("pp.post_id = ? AND products.deleted_at IS NULL", postID).
		Order("pp.sort ASC, pp.id ASC").
		Find(&products).Error
	if err != nil {
		return nil, err
	}
	result := make([]contract.RelatedProduct, 0, len(products))
	for index := range products {
		product := &products[index]
		result = append(result, contract.RelatedProduct{
			ID:          product.ID,
			Slug:        product.Slug,
			Title:       product.TitleJSON,
			PriceAmount: product.PriceAmount,
			Images:      product.Images,
			IsActive:    product.IsActive,
		})
	}
	return result, nil
}

func (s *PostStore) ListPostsForProduct(ctx context.Context, productID uint, postType string, onlyPublished bool, limit int) ([]contract.RelatedPost, error) {
	posts := make([]domain.Post, 0)
	statement := withContext(s.db, ctx).
		Joins("INNER JOIN post_products pp ON pp.post_id = posts.id").
		Where("pp.product_id = ? AND posts.deleted_at IS NULL", productID)
	if postType != "" {
		statement = statement.Where("posts.type = ?", postType)
	}
	if onlyPublished {
		statement = statement.Where("posts.is_published = ?", true)
	}
	if limit > 0 {
		statement = statement.Limit(limit)
	}
	if err := statement.Order("pp.sort ASC, pp.id ASC").Find(&posts).Error; err != nil {
		return nil, err
	}
	result := make([]contract.RelatedPost, 0, len(posts))
	for index := range posts {
		post := &posts[index]
		result = append(result, contract.RelatedPost{
			ID:          post.ID,
			Slug:        post.Slug,
			Type:        post.Type,
			Title:       post.TitleJSON,
			Summary:     post.SummaryJSON,
			Thumbnail:   post.Thumbnail,
			PublishedAt: post.PublishedAt,
		})
	}
	return result, nil
}
