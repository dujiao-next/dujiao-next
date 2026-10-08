package contenthttp

import (
	"context"
	"errors"
	"strconv"
	"strings"

	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

// AdminPostUseCases 是后台 Content Handler 实际需要的文章能力。
type AdminPostUseCases interface {
	ListAdmin(ctx context.Context, query contentapp.AdminPostQuery) ([]contentdomain.Post, int64, error)
	Create(ctx context.Context, input contentapp.CreatePostInput) (*contentdomain.Post, error)
	Update(ctx context.Context, id string, input contentapp.CreatePostInput) (*contentdomain.Post, error)
	Delete(ctx context.Context, id string) error
	SetPublished(ctx context.Context, id string, published bool) error
	MoveCategory(ctx context.Context, id string, categoryID uint) error
	ListRelatedProducts(ctx context.Context, postID uint) ([]contentcontract.RelatedProduct, error)
}

// AdminPostCategoryUseCases 是后台 Handler 实际需要的文章分类能力。
type AdminPostCategoryUseCases interface {
	ListAll(ctx context.Context, parentID *uint) ([]contentdomain.PostCategory, error)
	ListTree(ctx context.Context) ([]contentdomain.PostCategory, error)
	Create(ctx context.Context, input contentapp.CreatePostCategoryInput) (*contentdomain.PostCategory, error)
	Update(ctx context.Context, id uint, input contentapp.CreatePostCategoryInput) (*contentdomain.PostCategory, error)
	Delete(ctx context.Context, id uint) error
	SetActive(ctx context.Context, id uint, active bool) (*contentdomain.PostCategory, error)
}

// AdminBannerUseCases 是后台 Handler 实际需要的 Banner 能力。
type AdminBannerUseCases interface {
	ListAdmin(ctx context.Context, query contentapp.AdminBannerQuery) ([]contentdomain.Banner, int64, error)
	GetByID(ctx context.Context, id string) (*contentdomain.Banner, error)
	Create(ctx context.Context, input contentapp.BannerInput) (*contentdomain.Banner, error)
	Update(ctx context.Context, id string, input contentapp.BannerInput) (*contentdomain.Banner, error)
	Delete(ctx context.Context, id string) error
}

// AdminMediaUseCases 是后台 Handler 实际需要的素材能力。
type AdminMediaUseCases interface {
	List(ctx context.Context, query contentapp.MediaListQuery) ([]contentdomain.Media, int64, error)
	Rename(ctx context.Context, id uint, name string) error
	Delete(ctx context.Context, id uint) error
	BatchDelete(ctx context.Context, ids []uint) (int, []uint)
}

// AdminHandler 处理 Content 后台 HTTP 接口，仅持有窄用例依赖。
type AdminHandler struct {
	posts      AdminPostUseCases
	categories AdminPostCategoryUseCases
	banners    AdminBannerUseCases
	media      AdminMediaUseCases
}

// NewAdminHandler 创建后台 Content Handler。
func NewAdminHandler(posts AdminPostUseCases, categories AdminPostCategoryUseCases, banners AdminBannerUseCases, media AdminMediaUseCases) *AdminHandler {
	return &AdminHandler{posts: posts, categories: categories, banners: banners, media: media}
}

// GetAdminPosts 获取后台文章列表。
func (h *AdminHandler) GetAdminPosts(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	// 三态发布状态筛选：?is_published=1 仅已发布，?is_published=0 仅草稿，缺省/ all 不限。
	isPublished, err := parseTriStateBoolFilter(c.Query("is_published"))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	posts, total, err := h.posts.ListAdmin(c.Request.Context(), contentapp.AdminPostQuery{
		Type:        c.Query("type"),
		Search:      c.Query("search"),
		CategoryID:  c.Query("category_id"),
		IsPublished: isPublished,
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, posts, response.BuildPagination(page, pageSize, total))
}

// parseTriStateBoolFilter 把三态布尔查询参数解析为 *bool：""/"all" 返回 nil（不筛选），
// 其余真值与假值词表对齐商品侧的同名函数。
func parseTriStateBoolFilter(raw string) (*bool, error) {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return nil, nil
	case "1", "true", "yes", "on", "enabled", "has":
		parsed := true
		return &parsed, nil
	case "0", "false", "no", "off", "disabled", "none":
		parsed := false
		return &parsed, nil
	default:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, err
		}
		return &parsed, nil
	}
}

// CreatePost 创建文章。
func (h *AdminHandler) CreatePost(c *gin.Context) {
	var request CreatePostRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	post, err := h.posts.Create(c.Request.Context(), request.toInput())
	if err != nil {
		switch {
		case errors.Is(err, contentcontract.ErrInvalidPostType):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_type_invalid", nil)
		case errors.Is(err, contentcontract.ErrPostCategoryInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_category_invalid", nil)
		case errors.Is(err, contentcontract.ErrPostNoticeCategoryUnsupported):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_notice_category_unsupported", nil)
		case errors.Is(err, contentcontract.ErrSlugExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.post_create_failed", err)
		}
		return
	}
	response.Success(c, post)
}

// UpdatePost 更新文章。
func (h *AdminHandler) UpdatePost(c *gin.Context) {
	var request CreatePostRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	post, err := h.posts.Update(c.Request.Context(), c.Param("id"), request.toInput())
	if err != nil {
		switch {
		case errors.Is(err, contentcontract.ErrInvalidPostType):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_type_invalid", nil)
		case errors.Is(err, contentcontract.ErrPostCategoryInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_category_invalid", nil)
		case errors.Is(err, contentcontract.ErrPostNoticeCategoryUnsupported):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_notice_category_unsupported", nil)
		case errors.Is(err, contentcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.post_not_found", nil)
		case errors.Is(err, contentcontract.ErrSlugExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.post_update_failed", err)
		}
		return
	}
	response.Success(c, post)
}

// GetAdminPostProductIDs 获取文章关联商品列表。
func (h *AdminHandler) GetAdminPostProductIDs(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_id", nil)
		return
	}
	products, err := h.posts.ListRelatedProducts(c.Request.Context(), id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}
	response.Success(c, newAdminPostProductRefs(products))
}

// DeletePost 软删除文章。
func (h *AdminHandler) DeletePost(c *gin.Context) {
	if err := h.posts.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, contentcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.post_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.post_delete_failed", err)
		return
	}
	response.Success(c, nil)
}

// BatchUpdatePostStatus 批量发布文章或转为草稿。
func (h *AdminHandler) BatchUpdatePostStatus(c *gin.Context) {
	var request BatchPostStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	failedIDs := make([]uint, 0)
	for _, id := range request.IDs {
		if err := h.posts.SetPublished(c.Request.Context(), strconv.FormatUint(uint64(id), 10), request.IsPublished); err != nil {
			failedIDs = append(failedIDs, id)
		}
	}
	response.Success(c, gin.H{
		"total":         len(request.IDs),
		"success_count": len(request.IDs) - len(failedIDs),
		"failed_ids":    failedIDs,
	})
}

// BatchUpdatePostCategory 批量调整文章分类。
func (h *AdminHandler) BatchUpdatePostCategory(c *gin.Context) {
	var request BatchPostCategoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	failedIDs := make([]uint, 0)
	for _, id := range request.IDs {
		if err := h.posts.MoveCategory(c.Request.Context(), strconv.FormatUint(uint64(id), 10), request.CategoryID); err != nil {
			failedIDs = append(failedIDs, id)
		}
	}
	response.Success(c, gin.H{
		"total":         len(request.IDs),
		"success_count": len(request.IDs) - len(failedIDs),
		"failed_ids":    failedIDs,
	})
}

// BatchDeletePosts 批量软删除文章。
func (h *AdminHandler) BatchDeletePosts(c *gin.Context) {
	var request BatchDeletePostsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	failedIDs := make([]uint, 0)
	for _, id := range request.IDs {
		if err := h.posts.Delete(c.Request.Context(), strconv.FormatUint(uint64(id), 10)); err != nil {
			failedIDs = append(failedIDs, id)
		}
	}
	response.Success(c, gin.H{
		"total":         len(request.IDs),
		"success_count": len(request.IDs) - len(failedIDs),
		"failed_ids":    failedIDs,
	})
}

// GetPostCategories 获取后台文章分类列表或树。
func (h *AdminHandler) GetPostCategories(c *gin.Context) {
	requestContext := c.Request.Context()
	if c.Query("tree") == "1" {
		categories, err := h.categories.ListTree(requestContext)
		if err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.post_category_fetch_failed", err)
			return
		}
		response.Success(c, categories)
		return
	}

	var parentID *uint
	if rawParentID := c.Query("parent_id"); rawParentID != "" {
		parsed, err := strconv.ParseUint(rawParentID, 10, 64)
		if err == nil {
			value := uint(parsed)
			parentID = &value
		}
	}
	categories, err := h.categories.ListAll(requestContext, parentID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.post_category_fetch_failed", err)
		return
	}
	response.Success(c, categories)
}

// CreatePostCategory 创建文章分类。
func (h *AdminHandler) CreatePostCategory(c *gin.Context) {
	var request CreatePostCategoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	category, err := h.categories.Create(c.Request.Context(), postCategoryInput(request.NameJSON, request.Slug, request.ParentID, request.SortOrder, request.Icon))
	if err != nil {
		switch {
		case errors.Is(err, contentcontract.ErrSlugExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
		case errors.Is(err, contentcontract.ErrCategoryParentInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.category_parent_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.post_category_create_failed", err)
		}
		return
	}
	response.Success(c, category)
}

// UpdatePostCategory 更新文章分类。
func (h *AdminHandler) UpdatePostCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	var request UpdatePostCategoryRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	category, err := h.categories.Update(c.Request.Context(), uint(id), postCategoryInput(request.NameJSON, request.Slug, request.ParentID, request.SortOrder, request.Icon))
	if err != nil {
		switch {
		case errors.Is(err, contentcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.post_category_not_found", nil)
		case errors.Is(err, contentcontract.ErrSlugExists):
			ginutil.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
		case errors.Is(err, contentcontract.ErrCategoryParentInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.category_parent_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.post_category_update_failed", err)
		}
		return
	}
	response.Success(c, category)
}

// DeletePostCategory 删除文章分类。
func (h *AdminHandler) DeletePostCategory(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if err := h.categories.Delete(c.Request.Context(), uint(id)); err != nil {
		switch {
		case errors.Is(err, contentcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.post_category_not_found", nil)
		case errors.Is(err, contentcontract.ErrCategoryInUse):
			ginutil.RespondError(c, response.CodeBadRequest, "error.post_category_in_use", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.post_category_delete_failed", err)
		}
		return
	}
	response.Success(c, nil)
}

// PatchPostCategoryStatus 切换文章分类启用状态。
func (h *AdminHandler) PatchPostCategoryStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	var request PatchPostCategoryStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	category, err := h.categories.SetActive(c.Request.Context(), uint(id), *request.IsActive)
	if err != nil {
		if errors.Is(err, contentcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.post_category_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.post_category_update_failed", err)
		return
	}
	response.Success(c, category)
}

// GetAdminBanners 获取后台 Banner 列表。
func (h *AdminHandler) GetAdminBanners(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	isActive, err := ginutil.ParseQueryBoolPtr(c, "is_active")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	banners, total, err := h.banners.ListAdmin(c.Request.Context(), contentapp.AdminBannerQuery{
		Position: c.Query("position"),
		Search:   c.Query("search"),
		IsActive: isActive,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.banner_fetch_failed", err)
		return
	}
	response.SuccessWithPage(c, banners, response.BuildPagination(page, pageSize, total))
}

// GetAdminBanner 获取后台 Banner 详情。
func (h *AdminHandler) GetAdminBanner(c *gin.Context) {
	banner, err := h.banners.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, contentcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.banner_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.banner_fetch_failed", err)
		return
	}
	response.Success(c, banner)
}

// CreateBanner 创建 Banner。
func (h *AdminHandler) CreateBanner(c *gin.Context) {
	var request BannerUpsertRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	input, err := buildBannerInputFromRequest(request)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	banner, err := h.banners.Create(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, contentcontract.ErrInvalidBanner) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.banner_invalid", nil)
		} else {
			ginutil.RespondError(c, response.CodeInternal, "error.banner_create_failed", err)
		}
		return
	}
	response.Success(c, banner)
}

// UpdateBanner 更新 Banner。
func (h *AdminHandler) UpdateBanner(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var request BannerUpsertRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	input, err := buildBannerInputFromRequest(request)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	banner, err := h.banners.Update(c.Request.Context(), id, input)
	if err != nil {
		switch {
		case errors.Is(err, contentcontract.ErrInvalidBanner):
			ginutil.RespondError(c, response.CodeBadRequest, "error.banner_invalid", nil)
		case errors.Is(err, contentcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.banner_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.banner_update_failed", err)
		}
		return
	}
	response.Success(c, banner)
}

// DeleteBanner 删除 Banner。
func (h *AdminHandler) DeleteBanner(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.banners.Delete(c.Request.Context(), id); err != nil {
		if errors.Is(err, contentcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.banner_not_found", nil)
		} else {
			ginutil.RespondError(c, response.CodeInternal, "error.banner_delete_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// GetAdminMedia 获取素材列表。
func (h *AdminHandler) GetAdminMedia(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.media.List(c.Request.Context(), contentapp.MediaListQuery{
		Scene:    c.Query("scene"),
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, gin.H{"items": items, "total": total})
}

// UpdateMedia 更新素材名称。
func (h *AdminHandler) UpdateMedia(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_id", nil)
		return
	}
	var request UpdateMediaRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_params", nil)
		return
	}
	if err := h.media.Rename(c.Request.Context(), id, request.Name); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, nil)
}

// BatchDeleteMedia 批量删除素材。
func (h *AdminHandler) BatchDeleteMedia(c *gin.Context) {
	var request BatchDeleteMediaRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	successCount, failedIDs := h.media.BatchDelete(c.Request.Context(), request.IDs)
	response.Success(c, gin.H{
		"total":         len(request.IDs),
		"success_count": successCount,
		"failed_ids":    failedIDs,
	})
}

// DeleteMedia 删除素材。
func (h *AdminHandler) DeleteMedia(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.invalid_id", nil)
		return
	}
	if err := h.media.Delete(c.Request.Context(), id); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, nil)
}
