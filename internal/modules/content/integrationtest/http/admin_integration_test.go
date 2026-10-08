package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	localfilestore "github.com/dujiao-next/internal/modules/content/infrastructure/filestore/local"
	"github.com/dujiao-next/internal/modules/content/infrastructure/gormstore"
	contenttransport "github.com/dujiao-next/internal/modules/content/transport/http"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAdminContentHandlerTest(t *testing.T) (*contenttransport.AdminHandler, *contentapp.PostService, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:admin_content_handler_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&contentdomain.PostCategory{},
		&contentdomain.Post{},
		&contentdomain.PostProduct{},
		&contentdomain.Banner{},
		&contentdomain.Media{},
	); err != nil {
		t.Fatalf("auto migrate admin content tables: %v", err)
	}

	postStore := gormstore.NewPostStore(db)
	postService := contentapp.NewPostService(
		postStore,
		postStore,
		gormstore.NewPostCategoryStore(db),
		contentapp.SystemClock{},
	)
	handler := contenttransport.NewAdminHandler(
		postService,
		contentapp.NewPostCategoryService(gormstore.NewPostCategoryStore(db)),
		contentapp.NewBannerService(gormstore.NewBannerStore(db), contentapp.SystemClock{}),
		contentapp.NewMediaService(gormstore.NewMediaStore(db), localfilestore.New(), nil),
	)
	return handler, postService, db
}

func adminContentTestRouter(handler *contenttransport.AdminHandler) *gin.Engine {
	router := gin.New()
	admin := router.Group("/api/v1/admin")
	contenttransport.RegisterAdminRoutes(admin, handler)
	return router
}

func requestAdminContent(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 for %s %s, got %d body=%s", method, path, recorder.Code, recorder.Body.String())
	}
	return recorder
}

func decodeAdminContentResponse(t *testing.T, recorder *httptest.ResponseRecorder) response.Response {
	t.Helper()
	var got response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode admin content response: %v body=%s", err, recorder.Body.String())
	}
	return got
}

func TestAdminContentHandlersSuccessContracts(t *testing.T) {
	handler, _, db := setupAdminContentHandlerTest(t)
	router := adminContentTestRouter(handler)

	postRecorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/posts", `{
		"slug":"admin-post",
		"type":"blog",
		"title":{"zh-CN":"Admin post"},
		"is_published":true
	}`)
	if got := decodeAdminContentResponse(t, postRecorder); got.StatusCode != response.CodeOK {
		t.Fatalf("create post should succeed: %#v", got)
	}
	listRecorder := requestAdminContent(t, router, http.MethodGet, "/api/v1/admin/posts?type=blog", "")
	var postList struct {
		StatusCode int `json:"status_code"`
		Data       []struct {
			Slug string `json:"slug"`
		} `json:"data"`
		Pagination response.Pagination `json:"pagination"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &postList); err != nil {
		t.Fatalf("decode admin post list: %v", err)
	}
	if postList.StatusCode != response.CodeOK || len(postList.Data) != 1 || postList.Data[0].Slug != "admin-post" || postList.Pagination.Total != 1 {
		t.Fatalf("admin post list mismatch: %#v", postList)
	}

	categoryRecorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/post-categories", `{
		"name":{"zh-CN":"Docs"},
		"slug":"docs"
	}`)
	var categoryCreated struct {
		StatusCode int `json:"status_code"`
		Data       struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(categoryRecorder.Body.Bytes(), &categoryCreated); err != nil {
		t.Fatalf("decode created category: %v", err)
	}
	if categoryCreated.StatusCode != response.CodeOK || categoryCreated.Data.ID == 0 {
		t.Fatalf("create category should return ID: %#v", categoryCreated)
	}
	statusPath := fmt.Sprintf("/api/v1/admin/post-categories/%d/status", categoryCreated.Data.ID)
	if got := decodeAdminContentResponse(t, requestAdminContent(t, router, http.MethodPatch, statusPath, `{"is_active":false}`)); got.StatusCode != response.CodeOK {
		t.Fatalf("disable category should succeed: %#v", got)
	}
	var category contentdomain.PostCategory
	if err := db.First(&category, categoryCreated.Data.ID).Error; err != nil {
		t.Fatalf("reload disabled category: %v", err)
	}
	if category.IsActive {
		t.Fatalf("category status patch should persist false: %#v", category)
	}

	bannerRecorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/banners", `{
		"name":"Home",
		"image":"/uploads/home.png",
		"link_type":"none",
		"is_active":false
	}`)
	if got := decodeAdminContentResponse(t, bannerRecorder); got.StatusCode != response.CodeOK {
		t.Fatalf("create banner should succeed: %#v", got)
	}
	var inactiveBanner contentdomain.Banner
	if err := db.Where("name = ?", "Home").First(&inactiveBanner).Error; err != nil {
		t.Fatalf("reload inactive banner: %v", err)
	}
	if inactiveBanner.IsActive {
		t.Fatalf("admin banner create should persist explicit is_active=false: %#v", inactiveBanner)
	}

	media := contentdomain.Media{
		Name:     "before",
		Filename: "asset.png",
		Path:     "/uploads/common/asset.png",
		MimeType: "image/png",
		Size:     5,
		Scene:    "common",
	}
	if err := db.Create(&media).Error; err != nil {
		t.Fatalf("create media: %v", err)
	}
	mediaPath := fmt.Sprintf("/api/v1/admin/media/%d", media.ID)
	if got := decodeAdminContentResponse(t, requestAdminContent(t, router, http.MethodPut, mediaPath, `{"name":"after"}`)); got.StatusCode != response.CodeOK {
		t.Fatalf("rename media should succeed: %#v", got)
	}
	if err := db.First(&media, media.ID).Error; err != nil {
		t.Fatalf("reload media: %v", err)
	}
	if media.Name != "after" {
		t.Fatalf("media rename mismatch: %#v", media)
	}
}

func TestAdminContentHandlersValidationAndDomainErrorContracts(t *testing.T) {
	handler, posts, _ := setupAdminContentHandlerTest(t)
	router := adminContentTestRouter(handler)

	if _, err := posts.Create(context.Background(), contentapp.CreatePostInput{
		Slug:      "duplicate",
		Type:      "blog",
		TitleJSON: map[string]interface{}{"zh-CN": "duplicate"},
	}); err != nil {
		t.Fatalf("seed duplicate post: %v", err)
	}

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		wantCode int
	}{
		{
			name:     "post bind validation",
			method:   http.MethodPost,
			path:     "/api/v1/admin/posts",
			body:     `{"slug":"missing-title","type":"blog"}`,
			wantCode: response.CodeBadRequest,
		},
		{
			name:     "duplicate post slug",
			method:   http.MethodPost,
			path:     "/api/v1/admin/posts",
			body:     `{"slug":"duplicate","type":"blog","title":{"zh-CN":"duplicate"}}`,
			wantCode: response.CodeBadRequest,
		},
		{
			name:     "missing post update",
			method:   http.MethodPut,
			path:     "/api/v1/admin/posts/9999",
			body:     `{"slug":"missing","type":"blog","title":{"zh-CN":"missing"}}`,
			wantCode: response.CodeNotFound,
		},
		{
			name:     "invalid category parent",
			method:   http.MethodPost,
			path:     "/api/v1/admin/post-categories",
			body:     `{"name":{"zh-CN":"invalid"},"slug":"invalid","parent_id":9999}`,
			wantCode: response.CodeBadRequest,
		},
		{
			name:     "banner invalid time window",
			method:   http.MethodPost,
			path:     "/api/v1/admin/banners",
			body:     `{"name":"invalid","image":"/invalid.png","start_at":"2026-07-21T00:00:00Z","end_at":"2026-07-20T00:00:00Z"}`,
			wantCode: response.CodeBadRequest,
		},
		{
			name:     "missing banner",
			method:   http.MethodGet,
			path:     "/api/v1/admin/banners/9999",
			wantCode: response.CodeNotFound,
		},
		{
			name:     "invalid media ID",
			method:   http.MethodDelete,
			path:     "/api/v1/admin/media/not-a-number",
			wantCode: response.CodeBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := requestAdminContent(t, router, test.method, test.path, test.body)
			got := decodeAdminContentResponse(t, recorder)
			if got.StatusCode != test.wantCode {
				t.Fatalf("business status want %d got %d body=%s", test.wantCode, got.StatusCode, recorder.Body.String())
			}
		})
	}
}

// 后台文章列表的筛选参数：category_id 等值筛选、is_published 三态筛选，
// 并与 type 按 AND 组合；非法状态值返回参数错误而不是静默忽略。
func TestAdminContentPostsListFilters(t *testing.T) {
	handler, _, _ := setupAdminContentHandlerTest(t)
	router := adminContentTestRouter(handler)

	categoryRecorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/post-categories", `{
		"name":{"zh-CN":"筛选分类"},
		"slug":"filter-category"
	}`)
	var created struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(categoryRecorder.Body.Bytes(), &created); err != nil || created.Data.ID == 0 {
		t.Fatalf("create filter category failed: err=%v body=%s", err, categoryRecorder.Body.String())
	}
	categoryID := created.Data.ID

	createPost := func(slug, postType string, postCategoryID uint, published bool) {
		t.Helper()
		body := fmt.Sprintf(
			`{"slug":%q,"type":%q,"title":{"zh-CN":%q},"category_id":%d,"is_published":%t}`,
			slug, postType, slug, postCategoryID, published)
		if got := decodeAdminContentResponse(t, requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/posts", body)); got.StatusCode != response.CodeOK {
			t.Fatalf("create post %q failed: %#v", slug, got)
		}
	}
	// 公告不支持分类，保持无分类，与 validateCategoryAssignment 的约束一致
	createPost("filter-published", "blog", categoryID, true)
	createPost("filter-draft", "blog", categoryID, false)
	createPost("filter-uncategorized", "blog", 0, true)
	createPost("filter-notice", "notice", 0, true)

	listSlugs := func(query string) map[string]bool {
		t.Helper()
		recorder := requestAdminContent(t, router, http.MethodGet, "/api/v1/admin/posts?page_size=50"+query, "")
		var payload struct {
			Data []struct {
				Slug string `json:"slug"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("decode post list %q failed: %v body=%s", query, err, recorder.Body.String())
		}
		got := make(map[string]bool, len(payload.Data))
		for _, row := range payload.Data {
			got[row.Slug] = true
		}
		return got
	}

	got := listSlugs(fmt.Sprintf("&category_id=%d", categoryID))
	if !got["filter-published"] || !got["filter-draft"] || got["filter-uncategorized"] || got["filter-notice"] || len(got) != 2 {
		t.Fatalf("category_id filter mismatch: %+v", got)
	}

	got = listSlugs("&is_published=1")
	if !got["filter-published"] || !got["filter-uncategorized"] || !got["filter-notice"] || got["filter-draft"] || len(got) != 3 {
		t.Fatalf("is_published=1 mismatch: %+v", got)
	}

	got = listSlugs("&is_published=0")
	if !got["filter-draft"] || len(got) != 1 {
		t.Fatalf("is_published=0 mismatch: %+v", got)
	}

	got = listSlugs("&is_published=all")
	if len(got) != 4 {
		t.Fatalf("is_published=all should not filter: %+v", got)
	}

	got = listSlugs(fmt.Sprintf("&category_id=%d&type=blog", categoryID))
	if !got["filter-published"] || !got["filter-draft"] || len(got) != 2 {
		t.Fatalf("category_id + type mismatch: %+v", got)
	}

	got = listSlugs(fmt.Sprintf("&category_id=%d&is_published=0", categoryID))
	if !got["filter-draft"] || len(got) != 1 {
		t.Fatalf("category_id + is_published mismatch: %+v", got)
	}

	got = listSlugs("&category_id=999999")
	if len(got) != 0 {
		t.Fatalf("unknown category should be empty: %+v", got)
	}

	invalid := decodeAdminContentResponse(t, requestAdminContent(t, router, http.MethodGet, "/api/v1/admin/posts?is_published=abc", ""))
	if invalid.StatusCode != response.CodeBadRequest {
		t.Fatalf("is_published=abc should be rejected: %#v", invalid)
	}
}

// 后台文章的批量操作：批量发布/转草稿、批量移动分类、批量删除，
// 以及单项失败不影响其余项的部分成功语义（公告不支持分类、ID 不存在）。
func TestAdminContentPostsBatchActions(t *testing.T) {
	handler, _, db := setupAdminContentHandlerTest(t)
	router := adminContentTestRouter(handler)

	categoryRecorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/post-categories", `{
		"name":{"zh-CN":"批量分类"},
		"slug":"batch-category"
	}`)
	var createdCategory struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(categoryRecorder.Body.Bytes(), &createdCategory); err != nil || createdCategory.Data.ID == 0 {
		t.Fatalf("create batch category failed: err=%v body=%s", err, categoryRecorder.Body.String())
	}
	categoryID := createdCategory.Data.ID

	createPost := func(slug, postType string, published bool, postCategoryID uint) uint {
		t.Helper()
		body := fmt.Sprintf(`{"slug":%q,"type":%q,"title":{"zh-CN":%q},"category_id":%d,"is_published":%t}`,
			slug, postType, slug, postCategoryID, published)
		recorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/posts", body)
		var created struct {
			Data struct {
				ID uint `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil || created.Data.ID == 0 {
			t.Fatalf("create post %q failed: err=%v body=%s", slug, err, recorder.Body.String())
		}
		return created.Data.ID
	}

	batch := func(path, body string) (int, []uint) {
		t.Helper()
		recorder := requestAdminContent(t, router, http.MethodPost, path, body)
		var result struct {
			Data struct {
				Total        int    `json:"total"`
				SuccessCount int    `json:"success_count"`
				FailedIDs    []uint `json:"failed_ids"`
			} `json:"data"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode batch result for %s failed: %v body=%s", path, err, recorder.Body.String())
		}
		if result.Data.Total != result.Data.SuccessCount+len(result.Data.FailedIDs) {
			t.Fatalf("batch result for %s should account for every id: %+v", path, result.Data)
		}
		return result.Data.SuccessCount, result.Data.FailedIDs
	}

	loadPost := func(id uint) contentdomain.Post {
		t.Helper()
		var post contentdomain.Post
		if err := db.First(&post, id).Error; err != nil {
			t.Fatalf("load post %d failed: %v", id, err)
		}
		return post
	}

	draftA := createPost("batch-draft-a", "blog", false, 0)
	draftB := createPost("batch-draft-b", "blog", false, 0)
	publishedOne := createPost("batch-published", "blog", true, 0)
	notice := createPost("batch-notice", "notice", true, 0)

	// 批量发布：草稿转为已发布，并回填发布时间
	successCount, failedIDs := batch("/api/v1/admin/posts/batch-status", fmt.Sprintf(`{"ids":[%d,%d],"is_published":true}`, draftA, draftB))
	if successCount != 2 || len(failedIDs) != 0 {
		t.Fatalf("batch publish should succeed for both drafts: success=%d failed=%v", successCount, failedIDs)
	}
	for _, id := range []uint{draftA, draftB} {
		post := loadPost(id)
		if !post.IsPublished {
			t.Fatalf("post %d should be published: %#v", id, post)
		}
		if post.PublishedAt == nil {
			t.Fatalf("post %d should have published_at backfilled", id)
		}
	}

	// 批量转草稿：只影响选中的文章
	successCount, failedIDs = batch("/api/v1/admin/posts/batch-status", fmt.Sprintf(`{"ids":[%d],"is_published":false}`, draftA))
	if successCount != 1 || len(failedIDs) != 0 {
		t.Fatalf("batch draft should succeed: success=%d failed=%v", successCount, failedIDs)
	}
	if loadPost(draftA).IsPublished {
		t.Fatalf("post %d should be draft after batch draft", draftA)
	}
	if !loadPost(draftB).IsPublished {
		t.Fatalf("post %d should stay published", draftB)
	}

	// 批量移动分类：公告不支持分类，只应失败该项
	successCount, failedIDs = batch("/api/v1/admin/posts/batch-category", fmt.Sprintf(`{"ids":[%d,%d],"category_id":%d}`, draftB, notice, categoryID))
	if successCount != 1 || len(failedIDs) != 1 || failedIDs[0] != notice {
		t.Fatalf("batch category should fail only on notice: success=%d failed=%v", successCount, failedIDs)
	}
	moved := loadPost(draftB)
	if moved.CategoryID == nil || *moved.CategoryID != categoryID {
		t.Fatalf("post %d should move to category %d, got %#v", draftB, categoryID, moved.CategoryID)
	}
	if loadPost(notice).CategoryID != nil {
		t.Fatalf("notice %d should keep no category", notice)
	}

	// category_id 传 0 表示移出分类
	successCount, failedIDs = batch("/api/v1/admin/posts/batch-category", fmt.Sprintf(`{"ids":[%d],"category_id":0}`, draftB))
	if successCount != 1 || len(failedIDs) != 0 {
		t.Fatalf("batch clear category should succeed: success=%d failed=%v", successCount, failedIDs)
	}
	if loadPost(draftB).CategoryID != nil {
		t.Fatalf("post %d should be uncategorized again", draftB)
	}

	// 批量删除：软删除，不存在的 ID 计入 failed_ids
	successCount, failedIDs = batch("/api/v1/admin/posts/batch-delete", fmt.Sprintf(`{"ids":[%d,%d,999999]}`, publishedOne, notice))
	if successCount != 2 || len(failedIDs) != 1 || failedIDs[0] != 999999 {
		t.Fatalf("batch delete should report the missing id: success=%d failed=%v", successCount, failedIDs)
	}
	for _, id := range []uint{publishedOne, notice} {
		// Post.DeletedAt 是普通 *time.Time，GORM 不会自动追加软删除条件，
		// 这里与仓储层一致地显式排除已删除行。
		var live int64
		if err := db.Model(&contentdomain.Post{}).Where("id = ? AND deleted_at IS NULL", id).Count(&live).Error; err != nil {
			t.Fatalf("count live post %d failed: %v", id, err)
		}
		if live != 0 {
			t.Fatalf("post %d should be soft deleted", id)
		}
		var total int64
		if err := db.Model(&contentdomain.Post{}).Where("id = ?", id).Count(&total).Error; err != nil {
			t.Fatalf("count post %d failed: %v", id, err)
		}
		if total != 1 {
			t.Fatalf("post %d should be soft deleted rather than physically removed", id)
		}
	}

	// 空 ids 应被绑定校验拒绝；业务错误码在响应体中，HTTP 仍为 200
	emptyRecorder := requestAdminContent(t, router, http.MethodPost, "/api/v1/admin/posts/batch-status", `{"ids":[]}`)
	if got := decodeAdminContentResponse(t, emptyRecorder); got.StatusCode != response.CodeBadRequest {
		t.Fatalf("empty ids should be rejected: %#v", got)
	}

	// 由 SQL 直接写入的文章，其多语言字段读回后可能为空值（jsonmap.JSON 只解析 []byte）。
	// 批量更新必须只写目标列，否则会撞上 title_json 的非空约束、或清空可空的 summary/content。
	if err := db.Exec(`insert into posts (slug, type, title_json, summary_json, content_json, is_published, created_at)
		values (?, ?, ?, ?, ?, ?, ?)`,
		"batch-raw-sql", constants.PostTypeBlog,
		`{"zh-CN":"原始标题"}`, `{"zh-CN":"原始摘要"}`, `{"zh-CN":"原始内容"}`, false, time.Now()).Error; err != nil {
		t.Fatalf("insert sql-written post failed: %v", err)
	}
	var rawID uint
	if err := db.Raw("select id from posts where slug = ?", "batch-raw-sql").Scan(&rawID).Error; err != nil || rawID == 0 {
		t.Fatalf("load sql-written post failed: err=%v id=%d", err, rawID)
	}

	successCount, failedIDs = batch("/api/v1/admin/posts/batch-status", fmt.Sprintf(`{"ids":[%d],"is_published":true}`, rawID))
	if successCount != 1 || len(failedIDs) != 0 {
		t.Fatalf("batch publish on sql-written post should succeed: success=%d failed=%v", successCount, failedIDs)
	}

	var after struct {
		TitleJSON   string
		SummaryJSON string
		ContentJSON string
		IsPublished bool
	}
	if err := db.Raw(`select cast(title_json as text) as title_json, cast(summary_json as text) as summary_json,
		cast(content_json as text) as content_json, is_published from posts where id = ?`, rawID).Scan(&after).Error; err != nil {
		t.Fatalf("reload sql-written post failed: %v", err)
	}
	if after.TitleJSON != `{"zh-CN":"原始标题"}` || after.SummaryJSON != `{"zh-CN":"原始摘要"}` || after.ContentJSON != `{"zh-CN":"原始内容"}` {
		t.Fatalf("batch update must not rewrite the localized columns: %+v", after)
	}
	if !after.IsPublished {
		t.Fatalf("sql-written post should be published after batch update")
	}
}
