package contenthttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/gin-gonic/gin"
)

type requestContextKey struct{}

func TestPublicHandlerPassesRequestContextToUseCase(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	posts := &publicPostQueriesStub{}
	handler := NewPublicHandler(posts, nil, nil)
	router := gin.New()
	router.GET("/posts", handler.GetPosts)

	requestContext, cancel := context.WithCancel(context.WithValue(context.Background(), requestContextKey{}, "request-value"))
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/posts?type=blog&page=2&page_size=5", nil).WithContext(requestContext)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if posts.receivedContext == nil {
		t.Fatal("use case did not receive a context")
	}
	if got := posts.receivedContext.Value(requestContextKey{}); got != "request-value" {
		t.Fatalf("context value = %v, want request-value", got)
	}
	if posts.receivedContext.Err() != context.Canceled {
		t.Fatalf("context error = %v, want context.Canceled", posts.receivedContext.Err())
	}
	if posts.receivedQuery.Type != "blog" || posts.receivedQuery.Page != 2 || posts.receivedQuery.PageSize != 5 {
		t.Fatalf("query mapping mismatch: %#v", posts.receivedQuery)
	}
}

// TestPublicPostListPassesCategoryFilter 确认分类筛选参数（slug）会被透传到用例层。
func TestPublicPostListPassesCategoryFilter(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	posts := &publicPostQueriesStub{}
	handler := NewPublicHandler(posts, &publicPostCategoryQueriesStub{}, nil)
	router := gin.New()
	router.GET("/posts", handler.GetPosts)

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/posts?type=blog&category_slug=changelog", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	if posts.receivedQuery.CategorySlug != "changelog" {
		t.Fatalf("CategorySlug = %q, want changelog", posts.receivedQuery.CategorySlug)
	}
}

// TestPublicPostDetailCategoryRendering 覆盖详情分类段的三种情形：有分类、未挂分类、
// 分类已被删除。后两种都必须保证详情本身照常可读，只是不渲染分类。
// 对外只暴露分类 slug 与名称 —— 详情响应里不该出现分类自增 id。
func TestPublicPostDetailCategoryRendering(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	categoryID := uint(12)

	cases := []struct {
		name         string
		post         *contentdomain.Post
		categories   *publicPostCategoryQueriesStub
		wantCategory bool
	}{
		{
			name: "挂有分类时详情带出分类 slug 与名称",
			post: &contentdomain.Post{ID: 7, Slug: "v1-4", Type: constants.PostTypeBlog, CategoryID: &categoryID},
			categories: &publicPostCategoryQueriesStub{byID: map[uint]*contentdomain.PostCategory{
				categoryID: {ID: categoryID, Slug: "changelog", NameJSON: jsonmap.JSON{"zh-CN": "更新日志"}, IsActive: true},
			}},
			wantCategory: true,
		},
		{
			name:       "未挂分类时不返回分类字段",
			post:       &contentdomain.Post{ID: 8, Slug: "notice", Type: constants.PostTypeNotice},
			categories: &publicPostCategoryQueriesStub{},
		},
		{
			name: "公告即使带有分类ID也不返回分类",
			post: &contentdomain.Post{ID: 10, Slug: "notice-categorized", Type: constants.PostTypeNotice, CategoryID: &categoryID},
			categories: &publicPostCategoryQueriesStub{byID: map[uint]*contentdomain.PostCategory{
				categoryID: {ID: categoryID, Slug: "changelog", NameJSON: jsonmap.JSON{"zh-CN": "更新日志"}, IsActive: true},
			}},
		},
		{
			name:       "分类已被删除时详情仍可读且不带分类",
			post:       &contentdomain.Post{ID: 9, Slug: "orphan", Type: constants.PostTypeBlog, CategoryID: &categoryID},
			categories: &publicPostCategoryQueriesStub{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := NewPublicHandler(&publicPostQueriesStub{post: tc.post}, tc.categories, nil)
			router := gin.New()
			router.GET("/posts/:slug", handler.GetPostBySlug)

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/posts/"+tc.post.Slug, nil))

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body=%s", response.Code, response.Body.String())
			}
			body := response.Body.String()
			if tc.wantCategory {
				if !strings.Contains(body, `"category_slug":"changelog"`) {
					t.Fatalf("detail response must carry category slug, got %s", body)
				}
				// 分类名随详情一起返回，前端不必再拉一次分类列表。
				if !strings.Contains(body, `"category_name":{"zh-CN":"更新日志"}`) {
					t.Fatalf("detail response must carry category name, got %s", body)
				}
				return
			}
			if strings.Contains(body, `"category_slug"`) || strings.Contains(body, `"category_name"`) {
				t.Fatalf("detail response must omit category, got %s", body)
			}
		})
	}
}

func TestAdminHandlerPassesRequestContextToUseCase(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	media := &adminMediaUseCasesStub{}
	handler := NewAdminHandler(nil, nil, nil, media)
	router := gin.New()
	router.GET("/media", handler.GetAdminMedia)

	requestContext := context.WithValue(context.Background(), requestContextKey{}, "admin-request")
	request := httptest.NewRequest(http.MethodGet, "/media?scene=post&page=3&page_size=15", nil).WithContext(requestContext)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if media.receivedContext == nil || media.receivedContext.Value(requestContextKey{}) != "admin-request" {
		t.Fatalf("admin use case received wrong context: %v", media.receivedContext)
	}
	if media.receivedQuery.Scene != "post" || media.receivedQuery.Page != 3 || media.receivedQuery.PageSize != 15 {
		t.Fatalf("media query mapping mismatch: %#v", media.receivedQuery)
	}
}

type publicPostQueriesStub struct {
	receivedContext context.Context
	receivedQuery   contentapp.PublicPostQuery
	post            *contentdomain.Post
}

var _ PublicPostQueries = (*publicPostQueriesStub)(nil)

func (s *publicPostQueriesStub) ListPublic(ctx context.Context, query contentapp.PublicPostQuery) ([]contentdomain.Post, int64, error) {
	s.receivedContext = ctx
	s.receivedQuery = query
	return []contentdomain.Post{}, 0, nil
}

func (s *publicPostQueriesStub) GetPublicBySlug(context.Context, string) (*contentdomain.Post, error) {
	if s.post == nil {
		return nil, contentcontract.ErrNotFound
	}
	return s.post, nil
}

func (s *publicPostQueriesStub) ListRelatedProducts(context.Context, uint) ([]contentcontract.RelatedProduct, error) {
	return nil, nil
}

type publicPostCategoryQueriesStub struct {
	byID map[uint]*contentdomain.PostCategory
}

var _ PublicPostCategoryQueries = (*publicPostCategoryQueriesStub)(nil)

func (s *publicPostCategoryQueriesStub) ListActive(context.Context) ([]contentdomain.PostCategory, error) {
	return nil, nil
}

func (s *publicPostCategoryQueriesStub) GetByID(_ context.Context, id uint) (*contentdomain.PostCategory, error) {
	return s.byID[id], nil
}

type adminMediaUseCasesStub struct {
	receivedContext context.Context
	receivedQuery   contentapp.MediaListQuery
}

var _ AdminMediaUseCases = (*adminMediaUseCasesStub)(nil)

func (s *adminMediaUseCasesStub) List(ctx context.Context, query contentapp.MediaListQuery) ([]contentdomain.Media, int64, error) {
	s.receivedContext = ctx
	s.receivedQuery = query
	return []contentdomain.Media{}, 0, nil
}

func (s *adminMediaUseCasesStub) Rename(context.Context, uint, string) error { return nil }
func (s *adminMediaUseCasesStub) Delete(context.Context, uint) error         { return nil }
func (s *adminMediaUseCasesStub) BatchDelete(context.Context, []uint) (int, []uint) {
	return 0, []uint{}
}
