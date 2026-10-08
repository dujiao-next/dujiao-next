package gormstore

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"

	"github.com/dujiao-next/internal/constants"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupContentStoreTest(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:content_store_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&categorydomain.Category{},
		&productdomain.Product{},
		&contentdomain.PostCategory{},
		&contentdomain.Post{},
		&contentdomain.PostProduct{},
		&contentdomain.Banner{},
		&contentdomain.Media{},
	); err != nil {
		t.Fatalf("auto migrate content store tables: %v", err)
	}
	return db
}

func TestPostStoreQueriesAndOrderedRelations(t *testing.T) {
	db := setupContentStoreTest(t)
	store := NewPostStore(db)
	ctx := context.Background()
	now := time.Now()
	older := now.Add(-2 * time.Hour)
	newer := now.Add(-time.Hour)

	category := categorydomain.Category{Slug: "products", NameJSON: jsonmap.JSON{"zh-CN": "products"}, IsActive: true}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create product category: %v", err)
	}
	firstProduct := productdomain.Product{CategoryID: category.ID, Slug: "first-product", TitleJSON: jsonmap.JSON{"zh-CN": "first"}, IsActive: true}
	secondProduct := productdomain.Product{CategoryID: category.ID, Slug: "second-product", TitleJSON: jsonmap.JSON{"zh-CN": "second"}, IsActive: true}
	if err := db.Create(&firstProduct).Error; err != nil {
		t.Fatalf("create first product: %v", err)
	}
	if err := db.Create(&secondProduct).Error; err != nil {
		t.Fatalf("create second product: %v", err)
	}

	posts := []contentdomain.Post{
		{Slug: "older", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "Older guide"}, IsPublished: true, PublishedAt: &older},
		{Slug: "newer", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "Newer guide"}, IsPublished: true, PublishedAt: &newer},
		{Slug: "draft", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "Draft guide"}, IsPublished: false},
		{Slug: "notice", Type: constants.PostTypeNotice, TitleJSON: jsonmap.JSON{"zh-CN": "Notice"}, IsPublished: true, PublishedAt: &newer},
	}
	for index := range posts {
		if err := store.Create(ctx, &posts[index]); err != nil {
			t.Fatalf("create post %q: %v", posts[index].Slug, err)
		}
	}

	listed, total, err := store.List(ctx, contentcontract.PostQuery{
		Page:          1,
		PageSize:      20,
		Type:          constants.PostTypeBlog,
		Search:        "guide",
		OnlyPublished: true,
		Order:         contentcontract.PostOrderPublishedDesc,
	})
	if err != nil {
		t.Fatalf("list public blog posts: %v", err)
	}
	if total != 2 || len(listed) != 2 || listed[0].Slug != "newer" || listed[1].Slug != "older" {
		t.Fatalf("post query mismatch total=%d posts=%#v", total, listed)
	}

	count, err := store.CountBySlug(ctx, "newer", nil)
	if err != nil || count != 1 {
		t.Fatalf("count slug want 1, count=%d err=%v", count, err)
	}
	excludeID := fmt.Sprintf("%d", posts[1].ID)
	count, err = store.CountBySlug(ctx, "newer", &excludeID)
	if err != nil || count != 0 {
		t.Fatalf("excluded slug count want 0, count=%d err=%v", count, err)
	}

	if err := store.SetRelatedProductIDs(ctx, posts[0].ID, []uint{secondProduct.ID, 0, firstProduct.ID, secondProduct.ID}); err != nil {
		t.Fatalf("set related products: %v", err)
	}
	ids, err := store.GetRelatedProductIDs(ctx, posts[0].ID)
	if err != nil {
		t.Fatalf("get related product IDs: %v", err)
	}
	if len(ids) != 2 || ids[0] != secondProduct.ID || ids[1] != firstProduct.ID {
		t.Fatalf("related product order/dedup mismatch: %v", ids)
	}
	relatedProducts, err := store.ListRelatedProducts(ctx, posts[0].ID)
	if err != nil {
		t.Fatalf("list related products: %v", err)
	}
	if len(relatedProducts) != 2 || relatedProducts[0].ID != secondProduct.ID || relatedProducts[1].ID != firstProduct.ID {
		t.Fatalf("related product query mismatch: %#v", relatedProducts)
	}

	if err := store.SetRelatedProductIDs(ctx, posts[1].ID, []uint{secondProduct.ID}); err != nil {
		t.Fatalf("relate second post: %v", err)
	}
	if err := store.SetRelatedProductIDs(ctx, posts[2].ID, []uint{secondProduct.ID}); err != nil {
		t.Fatalf("relate draft post: %v", err)
	}
	if err := store.SetRelatedProductIDs(ctx, posts[3].ID, []uint{secondProduct.ID}); err != nil {
		t.Fatalf("relate notice: %v", err)
	}
	forProduct, err := store.ListPostsForProduct(ctx, secondProduct.ID, constants.PostTypeBlog, true, 10)
	if err != nil {
		t.Fatalf("list posts for product: %v", err)
	}
	if len(forProduct) != 2 || forProduct[0].ID != posts[0].ID || forProduct[1].ID != posts[1].ID {
		t.Fatalf("posts for product must filter draft/notice and preserve relation order: %#v", forProduct)
	}

	if err := store.Delete(ctx, fmt.Sprintf("%d", posts[0].ID)); err != nil {
		t.Fatalf("delete post: %v", err)
	}
	deleted, err := store.GetByID(ctx, fmt.Sprintf("%d", posts[0].ID))
	if err != nil || deleted != nil {
		t.Fatalf("soft-deleted post should be hidden, post=%#v err=%v", deleted, err)
	}
}

func TestPostCategoryStoreTreeFiltersAndUsageCounts(t *testing.T) {
	db := setupContentStoreTest(t)
	store := NewPostCategoryStore(db)
	ctx := context.Background()

	root := contentdomain.PostCategory{Slug: "root", NameJSON: jsonmap.JSON{"zh-CN": "root"}, IsActive: true, SortOrder: 2}
	if err := store.Create(ctx, &root); err != nil {
		t.Fatalf("create root category: %v", err)
	}
	child := contentdomain.PostCategory{Slug: "child", NameJSON: jsonmap.JSON{"zh-CN": "child"}, ParentID: &root.ID, IsActive: true, SortOrder: 1}
	if err := store.Create(ctx, &child); err != nil {
		t.Fatalf("create child category: %v", err)
	}
	disabled := contentdomain.PostCategory{Slug: "disabled", NameJSON: jsonmap.JSON{"zh-CN": "disabled"}, IsActive: true, SortOrder: 0}
	if err := store.Create(ctx, &disabled); err != nil {
		t.Fatalf("create disabled category fixture: %v", err)
	}
	if err := store.UpdateActive(ctx, disabled.ID, false); err != nil {
		t.Fatalf("disable category: %v", err)
	}

	active, err := store.ListActive(ctx)
	if err != nil {
		t.Fatalf("list active categories: %v", err)
	}
	if len(active) != 2 || active[0].ID != child.ID || active[1].ID != root.ID {
		t.Fatalf("active category filtering/order mismatch: %#v", active)
	}
	tree, err := store.ListTree(ctx)
	if err != nil {
		t.Fatalf("list category tree: %v", err)
	}
	if len(tree) != 2 || tree[1].ID != root.ID || len(tree[1].Children) != 1 || tree[1].Children[0].ID != child.ID {
		t.Fatalf("category tree mismatch: %#v", tree)
	}

	childCount, err := store.CountChildren(ctx, root.ID)
	if err != nil || childCount != 1 {
		t.Fatalf("child count want 1, count=%d err=%v", childCount, err)
	}
	post := contentdomain.Post{Slug: "category-post", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "post"}, CategoryID: &child.ID}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("create categorized post: %v", err)
	}
	postCount, err := store.CountPostsByCategory(ctx, child.ID)
	if err != nil || postCount != 1 {
		t.Fatalf("category post count want 1, count=%d err=%v", postCount, err)
	}

	if err := store.Delete(ctx, disabled.ID); err != nil {
		t.Fatalf("delete category: %v", err)
	}
	deleted, err := store.GetByID(ctx, disabled.ID)
	if err != nil || deleted != nil {
		t.Fatalf("soft-deleted category should be hidden, category=%#v err=%v", deleted, err)
	}
}

func TestBannerStoreSearchAndValidityWindow(t *testing.T) {
	db := setupContentStoreTest(t)
	store := NewBannerStore(db)
	ctx := context.Background()
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	banners := []contentdomain.Banner{
		{Name: "primary", Position: constants.BannerPositionHomeHero, TitleJSON: jsonmap.JSON{"en-US": "Hero launch"}, Image: "/primary.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, StartAt: &past, EndAt: &future, SortOrder: 20},
		{Name: "secondary", Position: constants.BannerPositionHomeHero, TitleJSON: jsonmap.JSON{"en-US": "Secondary"}, Image: "/secondary.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, SortOrder: 10},
		{Name: "future", Position: constants.BannerPositionHomeHero, TitleJSON: jsonmap.JSON{"en-US": "Future"}, Image: "/future.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, StartAt: &future, SortOrder: 30},
		{Name: "disabled", Position: constants.BannerPositionHomeHero, TitleJSON: jsonmap.JSON{"en-US": "Hero disabled"}, Image: "/disabled.png", LinkType: constants.BannerLinkTypeNone, IsActive: true, SortOrder: 40},
	}
	for index := range banners {
		if err := store.Create(ctx, &banners[index]); err != nil {
			t.Fatalf("create banner %q: %v", banners[index].Name, err)
		}
	}
	if err := db.Model(&contentdomain.Banner{}).Where("id = ?", banners[3].ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("disable banner fixture: %v", err)
	}

	listed, total, err := store.List(ctx, contentcontract.BannerQuery{Page: 1, PageSize: 20, Search: "hero"})
	if err != nil {
		t.Fatalf("search banners: %v", err)
	}
	if total != 2 || len(listed) != 2 || listed[0].Name != "disabled" || listed[1].Name != "primary" {
		t.Fatalf("banner localized search/order mismatch total=%d banners=%#v", total, listed)
	}

	valid, err := store.ListValidByPosition(ctx, constants.BannerPositionHomeHero, 10, now)
	if err != nil {
		t.Fatalf("list valid banners: %v", err)
	}
	if len(valid) != 2 || valid[0].Name != "primary" || valid[1].Name != "secondary" {
		t.Fatalf("banner validity filtering mismatch: %#v", valid)
	}

	if err := store.Delete(ctx, fmt.Sprintf("%d", banners[0].ID)); err != nil {
		t.Fatalf("delete banner: %v", err)
	}
	deleted, err := store.GetByID(ctx, fmt.Sprintf("%d", banners[0].ID))
	if err != nil || deleted != nil {
		t.Fatalf("soft-deleted banner should be hidden, banner=%#v err=%v", deleted, err)
	}
	valid, err = store.ListValidByPosition(ctx, constants.BannerPositionHomeHero, 10, now)
	if err != nil || len(valid) != 1 || valid[0].ID != banners[1].ID {
		t.Fatalf("public banner query must hide soft-deleted rows, banners=%#v err=%v", valid, err)
	}
}

func TestMediaStoreFiltersUpdatesAndSoftDeletes(t *testing.T) {
	db := setupContentStoreTest(t)
	store := NewMediaStore(db)
	ctx := context.Background()

	items := []contentdomain.Media{
		{Name: "Alpha asset", Filename: "alpha.png", Path: "/uploads/common/alpha.png", MimeType: "image/png", Size: 10, Scene: "common"},
		{Name: "Beta", Filename: "special-beta.png", Path: "/uploads/common/beta.png", MimeType: "image/png", Size: 20, Scene: "common"},
		{Name: "Alpha product", Filename: "product.png", Path: "/uploads/product/product.png", MimeType: "image/png", Size: 30, Scene: "product"},
	}
	for index := range items {
		if err := store.Create(ctx, &items[index]); err != nil {
			t.Fatalf("create media %q: %v", items[index].Name, err)
		}
	}

	listed, total, err := store.List(ctx, contentcontract.MediaQuery{Page: 1, PageSize: 20, Scene: "common", Search: "special"})
	if err != nil {
		t.Fatalf("list media: %v", err)
	}
	if total != 1 || len(listed) != 1 || listed[0].ID != items[1].ID {
		t.Fatalf("media scene/search mismatch total=%d items=%#v", total, listed)
	}

	byPath, err := store.GetByPath(ctx, items[0].Path)
	if err != nil || byPath == nil || byPath.ID != items[0].ID {
		t.Fatalf("get media by path mismatch media=%#v err=%v", byPath, err)
	}
	byPath.Name = "Renamed"
	if err := store.Update(ctx, byPath); err != nil {
		t.Fatalf("update media: %v", err)
	}
	updated, err := store.GetByID(ctx, byPath.ID)
	if err != nil || updated == nil || updated.Name != "Renamed" {
		t.Fatalf("updated media mismatch media=%#v err=%v", updated, err)
	}

	if err := store.Delete(ctx, byPath.ID); err != nil {
		t.Fatalf("delete media: %v", err)
	}
	deleted, err := store.GetByID(ctx, byPath.ID)
	if err != nil || deleted != nil {
		t.Fatalf("soft-deleted media should be hidden, media=%#v err=%v", deleted, err)
	}
}

func TestStoresPropagateCancelledContext(t *testing.T) {
	db := setupContentStoreTest(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := NewPostStore(db).List(ctx, contentcontract.PostQuery{Page: 1, PageSize: 20})
	if err == nil {
		t.Fatal("cancelled context should stop the database query")
	}
}

func TestLocalizedSearchDialectExpressions(t *testing.T) {
	postgres, postgresArgs := buildLocalizedLikeConditionByDialect("postgres", []string{"slug"}, []string{"title_json"})
	if postgresArgs != 4 || !strings.Contains(postgres, "ILIKE") || !strings.Contains(postgres, "title_json::jsonb ->> 'zh-CN'") {
		t.Fatalf("postgres localized search expression mismatch args=%d sql=%s", postgresArgs, postgres)
	}
	sqliteCondition, sqliteArgs := buildLocalizedLikeConditionByDialect("sqlite", []string{"slug"}, []string{"title_json"})
	if sqliteArgs != 4 || !strings.Contains(sqliteCondition, " LIKE ") || !strings.Contains(sqliteCondition, `json_extract(title_json, '$."zh-CN"')`) {
		t.Fatalf("sqlite localized search expression mismatch args=%d sql=%s", sqliteArgs, sqliteCondition)
	}
}

// 后台文章列表的分类与发布状态筛选：category_id 等值匹配，is_published 为三态
// （nil 不限 / true 仅已发布 / false 仅草稿），二者与 type、search 按 AND 组合。
func TestPostStoreListAppliesAdminListingFilters(t *testing.T) {
	db := setupContentStoreTest(t)
	store := NewPostStore(db)
	ctx := context.Background()

	guides := contentdomain.PostCategory{Slug: "guides", NameJSON: jsonmap.JSON{"zh-CN": "指南"}, IsActive: true}
	news := contentdomain.PostCategory{Slug: "news", NameJSON: jsonmap.JSON{"zh-CN": "资讯"}, IsActive: true}
	if err := db.Create(&guides).Error; err != nil {
		t.Fatalf("create guides category: %v", err)
	}
	if err := db.Create(&news).Error; err != nil {
		t.Fatalf("create news category: %v", err)
	}

	posts := []contentdomain.Post{
		{Slug: "guides-published", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "指南一"}, CategoryID: &guides.ID, IsPublished: true},
		{Slug: "guides-draft", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "指南二"}, CategoryID: &guides.ID, IsPublished: false},
		{Slug: "news-published", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "资讯一"}, CategoryID: &news.ID, IsPublished: true},
		{Slug: "categoryless", Type: constants.PostTypeBlog, TitleJSON: jsonmap.JSON{"zh-CN": "未分类"}, IsPublished: true},
		{Slug: "guides-notice", Type: constants.PostTypeNotice, TitleJSON: jsonmap.JSON{"zh-CN": "公告"}, IsPublished: true},
	}
	for index := range posts {
		if err := store.Create(ctx, &posts[index]); err != nil {
			t.Fatalf("create post %q: %v", posts[index].Slug, err)
		}
	}

	slugs := func(query contentcontract.PostQuery) map[string]bool {
		t.Helper()
		query.Page = 1
		query.PageSize = 20
		rows, _, err := store.List(ctx, query)
		if err != nil {
			t.Fatalf("list posts: %v", err)
		}
		got := make(map[string]bool, len(rows))
		for _, row := range rows {
			got[row.Slug] = true
		}
		return got
	}

	got := slugs(contentcontract.PostQuery{CategoryID: fmt.Sprintf("%d", guides.ID)})
	if !got["guides-published"] || !got["guides-draft"] || got["news-published"] || got["categoryless"] || got["guides-notice"] || len(got) != 2 {
		t.Fatalf("category filter mismatch: %+v", got)
	}

	published := true
	got = slugs(contentcontract.PostQuery{IsPublished: &published})
	if !got["guides-published"] || !got["news-published"] || !got["categoryless"] || !got["guides-notice"] || got["guides-draft"] || len(got) != 4 {
		t.Fatalf("IsPublished=true mismatch: %+v", got)
	}

	draft := false
	got = slugs(contentcontract.PostQuery{IsPublished: &draft})
	if !got["guides-draft"] || len(got) != 1 {
		t.Fatalf("IsPublished=false mismatch: %+v", got)
	}

	got = slugs(contentcontract.PostQuery{})
	if len(got) != 5 {
		t.Fatalf("nil IsPublished should not filter: %+v", got)
	}

	got = slugs(contentcontract.PostQuery{CategoryID: fmt.Sprintf("%d", guides.ID), IsPublished: &draft})
	if !got["guides-draft"] || len(got) != 1 {
		t.Fatalf("category + IsPublished mismatch: %+v", got)
	}

	got = slugs(contentcontract.PostQuery{CategoryID: fmt.Sprintf("%d", guides.ID), Type: constants.PostTypeBlog})
	if !got["guides-published"] || !got["guides-draft"] || len(got) != 2 {
		t.Fatalf("category + type mismatch: %+v", got)
	}

	got = slugs(contentcontract.PostQuery{CategoryID: fmt.Sprintf("%d", guides.ID), Search: "资讯"})
	if len(got) != 0 {
		t.Fatalf("category + non-matching search should be empty: %+v", got)
	}

	got = slugs(contentcontract.PostQuery{CategoryID: "999999"})
	if len(got) != 0 {
		t.Fatalf("unknown category should be empty: %+v", got)
	}

	// OnlyPublished 已强制 is_published = true，此时不再叠加 IsPublished
	got = slugs(contentcontract.PostQuery{OnlyPublished: true, IsPublished: &draft})
	if !got["guides-published"] || got["guides-draft"] || len(got) != 4 {
		t.Fatalf("OnlyPublished should win over IsPublished: %+v", got)
	}
}
