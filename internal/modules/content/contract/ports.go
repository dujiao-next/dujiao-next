package contract

import (
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

// RelatedProduct 是 Content 从 Catalog 读取的最小商品投影。
type RelatedProduct struct {
	ID          uint
	Slug        string
	Title       jsonmap.JSON
	PriceAmount money.Amount
	Images      []string
	IsActive    bool
}

// RelatedPost 是 Catalog 从 Content 读取的最小文章投影。
type RelatedPost struct {
	ID          uint
	Slug        string
	Type        string
	Title       jsonmap.JSON
	Summary     jsonmap.JSON
	Thumbnail   string
	PublishedAt *time.Time
}

// PostWriteUnitOfWork 保证文章本体与商品关联在同一个持久化事务内写入。
type PostWriteUnitOfWork interface {
	WithinPostWriteTransaction(
		ctx context.Context,
		operation func(posts PostStore, relations PostProductRelationStore) error,
	) error
}

// PostStore 持久化文章本体；商品关联由独立端口负责。
type PostStore interface {
	PostWriteUnitOfWork
	List(ctx context.Context, query PostQuery) ([]domain.Post, int64, error)
	GetBySlug(ctx context.Context, slug string, onlyPublished bool) (*domain.Post, error)
	GetByID(ctx context.Context, id string) (*domain.Post, error)
	Create(ctx context.Context, post *domain.Post) error
	Update(ctx context.Context, post *domain.Post) error
	UpdatePublished(ctx context.Context, id string, published bool, publishedAt *time.Time) error
	UpdateCategory(ctx context.Context, id string, categoryID *uint) error
	Delete(ctx context.Context, id string) error
	CountBySlug(ctx context.Context, slug string, excludeID *string) (int64, error)
}

// PostProductRelationStore 持久化文章与商品的有序关联。
type PostProductRelationStore interface {
	GetRelatedProductIDs(ctx context.Context, postID uint) ([]uint, error)
	SetRelatedProductIDs(ctx context.Context, postID uint, productIDs []uint) error
	ListRelatedProducts(ctx context.Context, postID uint) ([]RelatedProduct, error)
	ListPostsForProduct(ctx context.Context, productID uint, postType string, onlyPublished bool, limit int) ([]RelatedPost, error)
}

// PostCategoryStore 持久化文章分类和分类占用关系。
type PostCategoryStore interface {
	ListAll(ctx context.Context, parentID *uint) ([]domain.PostCategory, error)
	ListActive(ctx context.Context) ([]domain.PostCategory, error)
	ListTree(ctx context.Context) ([]domain.PostCategory, error)
	GetByID(ctx context.Context, id uint) (*domain.PostCategory, error)
	Create(ctx context.Context, category *domain.PostCategory) error
	Update(ctx context.Context, category *domain.PostCategory) error
	UpdateActive(ctx context.Context, id uint, active bool) error
	Delete(ctx context.Context, id uint) error
	CountBySlug(ctx context.Context, slug string, excludeID *uint) (int64, error)
	CountChildren(ctx context.Context, parentID uint) (int64, error)
	CountPostsByCategory(ctx context.Context, categoryID uint) (int64, error)
}

// BannerStore 持久化后台 Banner，并提供按时间窗口读取公开 Banner 的查询。
type BannerStore interface {
	List(ctx context.Context, query BannerQuery) ([]domain.Banner, int64, error)
	ListValidByPosition(ctx context.Context, position string, limit int, now time.Time) ([]domain.Banner, error)
	GetByID(ctx context.Context, id string) (*domain.Banner, error)
	Create(ctx context.Context, banner *domain.Banner) error
	Update(ctx context.Context, banner *domain.Banner) error
	Delete(ctx context.Context, id string) error
}

// MediaStore 持久化素材元数据，不负责物理文件操作。
type MediaStore interface {
	List(ctx context.Context, query MediaQuery) ([]domain.Media, int64, error)
	GetByID(ctx context.Context, id uint) (*domain.Media, error)
	GetByPath(ctx context.Context, path string) (*domain.Media, error)
	Create(ctx context.Context, media *domain.Media) error
	Update(ctx context.Context, media *domain.Media) error
	Delete(ctx context.Context, id uint) error
}

// FileStore 描述 Media 用例真实需要的本地文件能力。
type FileStore interface {
	Stat(name string) (fs.FileInfo, error)
	Open(name string) (io.ReadCloser, error)
	Remove(name string) error
}

// Clock 让发布时间和 Banner 时间窗口可以在测试中固定。
type Clock interface {
	Now() time.Time
}

// WarningLogger 只暴露 Media 用例记录非致命文件副作用失败所需的能力。
type WarningLogger interface {
	Warnw(message string, keysAndValues ...interface{})
}
