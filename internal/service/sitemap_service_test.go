package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"

	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

func newSitemapServiceForTest(t *testing.T) (*SitemapService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:sitemap_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Product{},
		&models.ProductSKU{},
		&models.Post{},
	); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	svc := NewSitemapService(
		repository.NewProductRepository(db),
		repository.NewCategoryRepository(db),
		repository.NewPostRepository(db),
	)
	return svc, db
}

func TestSitemapServiceIncludesActiveContent(t *testing.T) {
	svc, db := newSitemapServiceForTest(t)

	activeCategory := models.Category{Slug: "games", NameJSON: models.JSON{"zh-CN": "games"}, IsActive: true}
	inactiveCategory := models.Category{Slug: "hidden", NameJSON: models.JSON{"zh-CN": "hidden"}, IsActive: true}
	if err := db.Create(&activeCategory).Error; err != nil {
		t.Fatalf("create active category: %v", err)
	}
	if err := db.Create(&inactiveCategory).Error; err != nil {
		t.Fatalf("create inactive category: %v", err)
	}
	// GORM 的 default:true tag 会让零值 false 写入时被 DB 默认值覆盖，需显式 Update 才能落到 false
	if err := db.Model(&models.Category{}).Where("id = ?", inactiveCategory.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("update inactive category: %v", err)
	}

	visibleProduct := models.Product{
		CategoryID:      activeCategory.ID,
		Slug:            "visible-product",
		TitleJSON:       models.JSON{"zh-CN": "p"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&visibleProduct).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	hiddenByProductInactive := models.Product{
		CategoryID:      activeCategory.ID,
		Slug:            "draft-product",
		TitleJSON:       models.JSON{"zh-CN": "p"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        false,
	}
	if err := db.Create(&hiddenByProductInactive).Error; err != nil {
		t.Fatalf("create draft product: %v", err)
	}

	hiddenByCategoryInactive := models.Product{
		CategoryID:      inactiveCategory.ID,
		Slug:            "in-hidden-category",
		TitleJSON:       models.JSON{"zh-CN": "p"},
		PriceAmount:     models.NewMoneyFromDecimal(decimal.NewFromInt(10)),
		PurchaseType:    constants.ProductPurchaseMember,
		FulfillmentType: constants.FulfillmentTypeManual,
		IsActive:        true,
	}
	if err := db.Create(&hiddenByCategoryInactive).Error; err != nil {
		t.Fatalf("create hidden-category product: %v", err)
	}

	publishedPost := models.Post{
		Slug:        "hello",
		Type:        constants.PostTypeBlog,
		TitleJSON:   models.JSON{"zh-CN": "hello"},
		IsPublished: true,
	}
	draftPost := models.Post{
		Slug:        "draft",
		Type:        constants.PostTypeBlog,
		TitleJSON:   models.JSON{"zh-CN": "draft"},
		IsPublished: false,
	}
	if err := db.Create(&publishedPost).Error; err != nil {
		t.Fatalf("create published post: %v", err)
	}
	if err := db.Create(&draftPost).Error; err != nil {
		t.Fatalf("create draft post: %v", err)
	}

	xmlStr, err := svc.Generate(context.Background(), "https://example.com", "none")
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
	svc, _ := newSitemapServiceForTest(t)

	body := svc.GenerateRobots("https://example.com")

	mustContain := []string{
		"User-agent: *",
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
	svc, _ := newSitemapServiceForTest(t)
	if _, err := svc.Generate(context.Background(), "", "none"); err == nil {
		t.Fatalf("expected error for empty base url")
	}
}
