package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/repository"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPostServiceForTest(t *testing.T) (*PostService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:post_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.PostCategory{}, &models.Post{}, &models.PostProduct{}); err != nil {
		t.Fatalf("auto migrate post tables failed: %v", err)
	}

	postRepo := repository.NewPostRepository(db)
	postCategoryRepo := repository.NewPostCategoryRepository(db)
	return NewPostService(postRepo, postCategoryRepo), db
}

func createPostCategoryFixture(t *testing.T, db *gorm.DB, slug string, parentID *uint) models.PostCategory {
	t.Helper()

	category := models.PostCategory{
		ParentID: parentID,
		Slug:     slug,
		NameJSON: models.JSON{
			"zh-CN": slug,
		},
		IsActive: true,
	}
	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("create post category fixture failed: %v", err)
	}
	return category
}

func createPostFixture(t *testing.T, db *gorm.DB, slug string, postType string, categoryID *uint) models.Post {
	t.Helper()

	post := models.Post{
		Slug:       slug,
		Type:       postType,
		TitleJSON:  models.JSON{"zh-CN": slug},
		CategoryID: categoryID,
	}
	if err := db.Create(&post).Error; err != nil {
		t.Fatalf("create post fixture failed: %v", err)
	}
	return post
}

func TestPostServiceCreateRejectsNoticeCategory(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	leaf := createPostCategoryFixture(t, db, "announcements", nil)

	_, err := svc.Create(CreatePostInput{
		Slug:       "notice-with-category",
		Type:       constants.PostTypeNotice,
		TitleJSON:  map[string]interface{}{"zh-CN": "notice-with-category"},
		CategoryID: &leaf.ID,
	})
	if err != ErrPostNoticeCategoryUnsupported {
		t.Fatalf("expected ErrPostNoticeCategoryUnsupported, got %v", err)
	}
}

func TestPostServiceCreateRejectsMissingOrParentCategory(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	parent := createPostCategoryFixture(t, db, "blog", nil)
	_ = createPostCategoryFixture(t, db, "backend", &parent.ID)

	missingID := uint(9999)
	_, err := svc.Create(CreatePostInput{
		Slug:       "blog-missing-category",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "blog-missing-category"},
		CategoryID: &missingID,
	})
	if err != ErrPostCategoryInvalid {
		t.Fatalf("expected ErrPostCategoryInvalid for missing category, got %v", err)
	}

	_, err = svc.Create(CreatePostInput{
		Slug:       "blog-parent-category",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "blog-parent-category"},
		CategoryID: &parent.ID,
	})
	if err != ErrPostCategoryInvalid {
		t.Fatalf("expected ErrPostCategoryInvalid for parent category, got %v", err)
	}
}

func TestPostServiceUpdateRejectsUnsupportedOrInvalidCategoryAssignment(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	parent := createPostCategoryFixture(t, db, "blog", nil)
	leaf := createPostCategoryFixture(t, db, "backend", &parent.ID)
	post := createPostFixture(t, db, "service-post", constants.PostTypeBlog, &leaf.ID)

	_, err := svc.Update(fmt.Sprintf("%d", post.ID), CreatePostInput{
		Slug:       post.Slug,
		Type:       constants.PostTypeNotice,
		TitleJSON:  map[string]interface{}{"zh-CN": post.Slug},
		CategoryID: &leaf.ID,
	})
	if err != ErrPostNoticeCategoryUnsupported {
		t.Fatalf("expected ErrPostNoticeCategoryUnsupported on notice update, got %v", err)
	}

	_, err = svc.Update(fmt.Sprintf("%d", post.ID), CreatePostInput{
		Slug:       post.Slug,
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": post.Slug},
		CategoryID: &parent.ID,
	})
	if err != ErrPostCategoryInvalid {
		t.Fatalf("expected ErrPostCategoryInvalid on parent category update, got %v", err)
	}
}

func TestPostServiceCategoryAssignmentRespectsInactive(t *testing.T) {
	svc, db := newPostServiceForTest(t)
	inactive := createPostCategoryFixture(t, db, "archived", nil)
	if err := db.Model(&models.PostCategory{}).Where("id = ?", inactive.ID).Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate category fixture failed: %v", err)
	}

	_, err := svc.Create(CreatePostInput{
		Slug:       "blog-inactive-category",
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": "blog-inactive-category"},
		CategoryID: &inactive.ID,
	})
	if err != ErrPostCategoryInvalid {
		t.Fatalf("expected ErrPostCategoryInvalid for inactive category, got %v", err)
	}

	// 文章已有分类后来被禁用：保存时保留原分类应放行
	post := createPostFixture(t, db, "blog-keeps-category", constants.PostTypeBlog, &inactive.ID)
	updated, err := svc.Update(fmt.Sprintf("%d", post.ID), CreatePostInput{
		Slug:       post.Slug,
		Type:       constants.PostTypeBlog,
		TitleJSON:  map[string]interface{}{"zh-CN": post.Slug},
		CategoryID: &inactive.ID,
	})
	if err != nil {
		t.Fatalf("expected keeping now-inactive category to succeed, got %v", err)
	}
	if updated.CategoryID == nil || *updated.CategoryID != inactive.ID {
		t.Fatalf("expected category to be kept, got %v", updated.CategoryID)
	}
}
