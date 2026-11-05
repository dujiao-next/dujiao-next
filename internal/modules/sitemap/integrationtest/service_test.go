package sitemap_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	productgormstore "github.com/dujiao-next/internal/modules/catalog/product/store/gormstore"

	"github.com/dujiao-next/internal/constants"
	categorygormstore "github.com/dujiao-next/internal/modules/catalog/category/infrastructure/gormstore"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	sitemapapp "github.com/dujiao-next/internal/modules/sitemap/application"
	sitemapcontract "github.com/dujiao-next/internal/modules/sitemap/contract"
	sitemapcatalog "github.com/dujiao-next/internal/modules/sitemap/infrastructure/catalogreader"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type noopCache struct{}

func (noopCache) GetString(context.Context, string) (string, error) { return "", nil }
func (noopCache) SetString(context.Context, string, string, time.Duration) error {
	return nil
}

func newSitemapServiceForTest(t *testing.T, reader sitemapcontract.PublishedPostReader) (*sitemapapp.Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:sitemap_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&productdomain.ProductSKU{},
		&contentdomain.Post{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	postStore := gormstore.NewPostStore(db)
	if reader == nil {
		reader = sitemapcontract.PublishedPostReaderFunc(func(ctx context.Context, limit int) ([]sitemapcontract.PublishedPost, error) {
			posts, _, err := postStore.List(ctx, contentcontract.PostQuery{
				Page:          1,
				PageSize:      limit,
				OnlyPublished: true,
				Order:         contentcontract.PostOrderPublishedDesc,
			})
			if err != nil {
				return nil, err
			}
			result := make([]sitemapcontract.PublishedPost, 0, len(posts))
			for _, post := range posts {
				result = append(result, sitemapcontract.PublishedPost{
					Slug:        post.Slug,
					CreatedAt:   post.CreatedAt,
					PublishedAt: post.PublishedAt,
				})
			}
			return result, nil
		})
	}
	svc, err := sitemapapp.NewService(
		sitemapcatalog.New(productgormstore.NewProductStore(db), categorygormstore.NewCategoryStore(db)),
		reader,
		noopCache{},
	)
	if err != nil {
		t.Fatalf("create sitemap service: %v", err)
	}
	return svc, db
}

func TestNewSitemapServiceRejectsNilPublishedPostReader(t *testing.T) {
	if _, err := sitemapapp.NewService(nil, nil, nil); err == nil {
		t.Fatal("expected nil dependencies to be rejected")
	}
}

type recordingPublishedPostReader struct {
	ctx   context.Context
	limit int
}

func (r *recordingPublishedPostReader) ListPublishedPosts(ctx context.Context, limit int) ([]sitemapcontract.PublishedPost, error) {
	r.ctx = ctx
	r.limit = limit
	return []sitemapcontract.PublishedPost{}, nil
}

func TestSitemapServicePassesCallerContextToPublishedPostReader(t *testing.T) {
	reader := &recordingPublishedPostReader{}
	svc, _ := newSitemapServiceForTest(t, reader)

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "sitemap-request")
	if _, err := svc.Generate(ctx, "https://context.example"); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if reader.ctx.Value(contextKey{}) != "sitemap-request" {
		t.Fatal("caller context was not propagated")
	}
	if reader.limit != 50000 {
		t.Fatalf("limit = %d, want %d", reader.limit, 50000)
	}
}

func TestSitemapServicePropagatesPublishedPostReaderFailure(t *testing.T) {
	wantErr := fmt.Errorf("published posts unavailable")
	svc, _ := newSitemapServiceForTest(t, sitemapcontract.PublishedPostReaderFunc(func(context.Context, int) ([]sitemapcontract.PublishedPost, error) {
		return nil, wantErr
	}))

	if _, err := svc.Generate(context.Background(), "https://failure.example"); err == nil || !strings.Contains(err.Error(), wantErr.Error()) {
		t.Fatalf("error = %v, want wrapped %v", err, wantErr)
	}
}

func TestSitemapServiceIncludesActiveContent(t *testing.T) {
	svc, db := newSitemapServiceForTest(t, nil)

	activeCategory := categorydomain.Category{Slug: "games", NameJSON: jsonmap.JSON{"zh-CN": "games"}, IsActive: true}
	inactiveCategory := categorydomain.Category{Slug: "hidden", NameJSON: jsonmap.JSON{"zh-CN": "hidden"}, IsActive: true}
	if err := db.Create(&activeCategory).Error; err != nil {
		t.Fatalf("create active category: %v", err)
	}
	if err := db.Create(&inactiveCategory).Error; err != nil {
		t.Fatalf("create inactive category: %v", err)
	}
	// GORM 的 default:true tag 会让零值 false 写入时被 DB 默认值覆盖，需显式 Update 才能落到 false
	if err := db.Model(&categorydomain.Category{}).Where("id = ?", inactiveCategory.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("update inactive category: %v", err)
	}

	visibleProduct := productdomain.Product{
		CategoryID:      activeCategory.ID,
		Slug:            "visible-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "p"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&visibleProduct).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	hiddenByProductInactive := productdomain.Product{
		CategoryID:      activeCategory.ID,
		Slug:            "draft-product",
		TitleJSON:       jsonmap.JSON{"zh-CN": "p"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        false,
	}
	if err := db.Create(&hiddenByProductInactive).Error; err != nil {
		t.Fatalf("create draft product: %v", err)
	}

	hiddenByCategoryInactive := productdomain.Product{
		CategoryID:      inactiveCategory.ID,
		Slug:            "in-hidden-category",
		TitleJSON:       jsonmap.JSON{"zh-CN": "p"},
		PriceAmount:     money.FromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&hiddenByCategoryInactive).Error; err != nil {
		t.Fatalf("create hidden-category product: %v", err)
	}

	publishedPost := contentdomain.Post{
		Slug:        "hello",
		Type:        constants.PostTypeBlog,
		TitleJSON:   jsonmap.JSON{"zh-CN": "hello"},
		IsPublished: true,
	}
	draftPost := contentdomain.Post{
		Slug:        "draft",
		Type:        constants.PostTypeBlog,
		TitleJSON:   jsonmap.JSON{"zh-CN": "draft"},
		IsPublished: false,
	}
	if err := db.Create(&publishedPost).Error; err != nil {
		t.Fatalf("create published post: %v", err)
	}
	if err := db.Create(&draftPost).Error; err != nil {
		t.Fatalf("create draft post: %v", err)
	}

	xmlStr, err := svc.Generate(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}

	mustContain := []string{
		"<urlset",
		"https://example.com/",
		"https://example.com/products",
		"https://example.com/blog",
		"https://example.com/categories/games",
		"https://example.com/products/visible-product",
		"https://example.com/blog/hello",
	}
	for _, s := range mustContain {
		if !strings.Contains(xmlStr, s) {
			t.Errorf("expected sitemap to contain %q\noutput:\n%s", s, xmlStr)
		}
	}

	mustNotContain := []string{
		"hidden",             // 停用分类
		"draft-product",      // 下架商品
		"in-hidden-category", // 分类停用下的商品
		"/blog/draft",        // 未发布文章
	}
	for _, s := range mustNotContain {
		if strings.Contains(xmlStr, s) {
			t.Errorf("sitemap should not contain %q\noutput:\n%s", s, xmlStr)
		}
	}
}

func TestSitemapServiceGenerateRobotsIncludesSitemapURL(t *testing.T) {
	svc, _ := newSitemapServiceForTest(t, nil)

	body := svc.GenerateRobots("https://example.com")

	mustContain := []string{
		"User-agent: *",
		"Allow: /api/v1/public/\nDisallow: /api/\n",
		"Disallow: /admin/",
		"Disallow: /me/",
		"Sitemap: https://example.com/sitemap.xml",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("robots.txt should contain %q\noutput:\n%s", s, body)
		}
	}
}

func TestSitemapServiceGenerateRejectsEmptyBaseURL(t *testing.T) {
	svc, _ := newSitemapServiceForTest(t, nil)
	if _, err := svc.Generate(context.Background(), ""); err == nil {
		t.Fatalf("expected error for empty base url")
	}
}
