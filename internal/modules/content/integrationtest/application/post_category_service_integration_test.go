package application_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPostCategoryServiceForTest(t *testing.T) (*contentapp.PostCategoryService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:post_category_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&contentdomain.PostCategory{}, &contentdomain.Post{}); err != nil {
		t.Fatalf("auto migrate post category tables: %v", err)
	}

	return contentapp.NewPostCategoryService(gormstore.NewPostCategoryStore(db)), db
}

func createCategoryThroughService(t *testing.T, svc *contentapp.PostCategoryService, slug string, parentID *uint, sortOrder int) *contentdomain.PostCategory {
	t.Helper()

	category, err := svc.Create(context.Background(), contentapp.CreatePostCategoryInput{
		NameJSON:  jsonmap.JSON{"zh-CN": slug},
		Slug:      slug,
		ParentID:  parentID,
		SortOrder: sortOrder,
	})
	if err != nil {
		t.Fatalf("create category %q: %v", slug, err)
	}
	return category
}

func TestPostCategoryServiceCreateRejectsInvalidParentAndDuplicateSlug(t *testing.T) {
	svc, _ := newPostCategoryServiceForTest(t)
	root := createCategoryThroughService(t, svc, "root", nil, 0)
	child := createCategoryThroughService(t, svc, "child", &root.ID, 0)

	missingID := uint(9999)
	for name, parentID := range map[string]*uint{
		"missing parent": &missingID,
		"third level":    &child.ID,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := svc.Create(context.Background(), contentapp.CreatePostCategoryInput{
				NameJSON: jsonmap.JSON{"zh-CN": name},
				Slug:     "invalid-" + name,
				ParentID: parentID,
			})
			if err != contentcontract.ErrCategoryParentInvalid {
				t.Fatalf("expected domaincontent.ErrCategoryParentInvalid, got %v", err)
			}
		})
	}

	_, err := svc.Create(context.Background(), contentapp.CreatePostCategoryInput{
		NameJSON: jsonmap.JSON{"zh-CN": "duplicate"},
		Slug:     root.Slug,
	})
	if err != contentcontract.ErrSlugExists {
		t.Fatalf("expected domaincontent.ErrSlugExists, got %v", err)
	}
}

func TestPostCategoryServiceUpdateRejectsCyclesAndMovingRootWithChildren(t *testing.T) {
	svc, _ := newPostCategoryServiceForTest(t)
	root := createCategoryThroughService(t, svc, "root", nil, 0)
	_ = createCategoryThroughService(t, svc, "child", &root.ID, 0)
	otherRoot := createCategoryThroughService(t, svc, "other-root", nil, 0)

	_, err := svc.Update(context.Background(), root.ID, contentapp.CreatePostCategoryInput{
		NameJSON: jsonmap.JSON{"zh-CN": "root"},
		Slug:     root.Slug,
		ParentID: &root.ID,
	})
	if err != contentcontract.ErrCategoryParentInvalid {
		t.Fatalf("expected self-parent update to fail, got %v", err)
	}

	_, err = svc.Update(context.Background(), root.ID, contentapp.CreatePostCategoryInput{
		NameJSON: jsonmap.JSON{"zh-CN": "root"},
		Slug:     root.Slug,
		ParentID: &otherRoot.ID,
	})
	if err != contentcontract.ErrCategoryParentInvalid {
		t.Fatalf("expected root with children move to fail, got %v", err)
	}
}

func TestPostCategoryServiceDeleteRejectsCategoriesInUse(t *testing.T) {
	svc, db := newPostCategoryServiceForTest(t)
	root := createCategoryThroughService(t, svc, "root", nil, 0)
	_ = createCategoryThroughService(t, svc, "child", &root.ID, 0)

	if err := svc.Delete(context.Background(), root.ID); err != contentcontract.ErrCategoryInUse {
		t.Fatalf("expected category with child to be in use, got %v", err)
	}

	withPost := createCategoryThroughService(t, svc, "with-post", nil, 0)
	post := contentdomain.Post{
		Slug:       "category-post",
		Type:       constants.PostTypeBlog,
		TitleJSON:  jsonmap.JSON{"zh-CN": "category-post"},
		CategoryID: &withPost.ID,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("create post using category: %v", err)
	}
	if err := svc.Delete(context.Background(), withPost.ID); err != contentcontract.ErrCategoryInUse {
		t.Fatalf("expected category with post to be in use, got %v", err)
	}

	empty := createCategoryThroughService(t, svc, "empty", nil, 0)
	if err := svc.Delete(context.Background(), empty.ID); err != nil {
		t.Fatalf("delete empty category: %v", err)
	}
	if _, err := svc.GetByID(context.Background(), empty.ID); err != contentcontract.ErrNotFound {
		t.Fatalf("expected soft-deleted category to be unavailable, got %v", err)
	}
}

// 公告没有分类功能，但历史数据里可能残留非空 category_id。
// 这种脏数据不能让分类删除被拦下 —— 否则后台会提示「下面有文章」，
// 而运营在文章列表里根本看不到任何文章。
func TestPostCategoryServiceDeleteIgnoresCategorizedNotices(t *testing.T) {
	svc, db := newPostCategoryServiceForTest(t)
	category := createCategoryThroughService(t, svc, "notice-dirty", nil, 0)

	notice := contentdomain.Post{
		Slug:       "dirty-notice",
		Type:       constants.PostTypeNotice,
		TitleJSON:  jsonmap.JSON{"zh-CN": "公告"},
		CategoryID: &category.ID,
	}
	if err := db.Create(&notice).Error; err != nil {
		t.Fatalf("create categorized notice: %v", err)
	}

	if err := svc.Delete(context.Background(), category.ID); err != nil {
		t.Fatalf("category with only a categorized notice must be deletable, got %v", err)
	}
}

func TestPostCategoryServiceListActiveFiltersAndSorts(t *testing.T) {
	svc, _ := newPostCategoryServiceForTest(t)
	higherSort := createCategoryThroughService(t, svc, "higher-sort", nil, 2)
	lowerSort := createCategoryThroughService(t, svc, "lower-sort", nil, 1)
	disabled := createCategoryThroughService(t, svc, "disabled", nil, 0)

	if _, err := svc.SetActive(context.Background(), disabled.ID, false); err != nil {
		t.Fatalf("disable category: %v", err)
	}

	categories, err := svc.ListActive(context.Background())
	if err != nil {
		t.Fatalf("list active categories: %v", err)
	}
	if len(categories) != 2 {
		t.Fatalf("expected two active categories, got %#v", categories)
	}
	if categories[0].ID != lowerSort.ID || categories[1].ID != higherSort.ID {
		t.Fatalf("expected sort_order then id ordering, got %#v", categories)
	}
}

func TestPostCategoryServiceListTreeIncludesDisabledChildren(t *testing.T) {
	svc, _ := newPostCategoryServiceForTest(t)
	root := createCategoryThroughService(t, svc, "root", nil, 0)
	child := createCategoryThroughService(t, svc, "child", &root.ID, 0)
	if _, err := svc.SetActive(context.Background(), child.ID, false); err != nil {
		t.Fatalf("disable child category: %v", err)
	}

	tree, err := svc.ListTree(context.Background())
	if err != nil {
		t.Fatalf("list category tree: %v", err)
	}
	if len(tree) != 1 || tree[0].ID != root.ID {
		t.Fatalf("unexpected category roots: %#v", tree)
	}
	if len(tree[0].Children) != 1 || tree[0].Children[0].ID != child.ID || tree[0].Children[0].IsActive {
		t.Fatalf("disabled child should remain visible in admin tree: %#v", tree[0].Children)
	}
}
