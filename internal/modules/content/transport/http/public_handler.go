package contenthttp

import (
	"context"
	"errors"
	"strconv"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	contentpresenter "github.com/dujiao-next/internal/modules/content/transport/presenter"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

// PublicPostQueries 是公开 Content Handler 实际需要的文章读取能力。
type PublicPostQueries interface {
	ListPublic(ctx context.Context, query contentapp.PublicPostQuery) ([]contentdomain.Post, int64, error)
	GetPublicBySlug(ctx context.Context, slug string) (*contentdomain.Post, error)
	ListRelatedProducts(ctx context.Context, postID uint) ([]contentcontract.RelatedProduct, error)
}

// PublicPostCategoryQueries 是公开 Handler 实际需要的分类读取能力。
type PublicPostCategoryQueries interface {
	ListActive(ctx context.Context) ([]contentdomain.PostCategory, error)
	GetByID(ctx context.Context, id uint) (*contentdomain.PostCategory, error)
}

// PublicBannerQueries 是公开 Handler 实际需要的 Banner 读取能力。
type PublicBannerQueries interface {
	ListPublic(ctx context.Context, query contentapp.PublicBannerQuery) ([]contentdomain.Banner, error)
}

// PublicHandler 处理 Content 公开 HTTP 接口，仅持有窄用例依赖。
type PublicHandler struct {
	posts      PublicPostQueries
	categories PublicPostCategoryQueries
	banners    PublicBannerQueries
}

// NewPublicHandler 创建公开 Content Handler。
func NewPublicHandler(posts PublicPostQueries, categories PublicPostCategoryQueries, banners PublicBannerQueries) *PublicHandler {
	return &PublicHandler{posts: posts, categories: categories, banners: banners}
}

// GetPosts 获取公开文章或公告列表。
func (h *PublicHandler) GetPosts(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	posts, total, err := h.posts.ListPublic(c.Request.Context(), contentapp.PublicPostQuery{
		Type:         c.Query("type"),
		Search:       c.Query("search"),
		CategorySlug: c.Query("category_slug"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, contentpresenter.NewPostRespList(posts), pagination)
}

// GetPostBySlug 根据 slug 获取公开文章详情。
func (h *PublicHandler) GetPostBySlug(c *gin.Context) {
	requestContext := c.Request.Context()
	post, err := h.posts.GetPublicBySlug(requestContext, c.Param("slug"))
	if err != nil {
		if errors.Is(err, contentcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.post_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	result := contentpresenter.NewPostResp(post)
	if post.Type == constants.PostTypeBlog {
		// 分类只属于博客文章（公告不支持分类），且分类链接指向博客列表，
		// 因此这里按类型收口，避免历史脏数据让公告面包屑链到空列表。
		// slug 与名称一次带出，前端不必再拉一次分类列表。
		if category := h.loadPostCategory(requestContext, post.CategoryID); category != nil {
			result.CategorySlug = category.Slug
			result.CategoryName = category.NameJSON
		}
		products, relatedErr := h.posts.ListRelatedProducts(requestContext, post.ID)
		if relatedErr == nil {
			result.RelatedProducts = contentpresenter.NewRelatedProductCardList(products)
		}
	}
	response.Success(c, result)
}

// loadPostCategory 读取文章所属分类，供详情页展示分类名称、拼接分类链接。
// 文章未挂分类、分类已被删除或读取失败时返回 nil —— 分类只是详情页的补充信息，
// 不应因此让整篇详情不可读，所以这里静默降级。
func (h *PublicHandler) loadPostCategory(ctx context.Context, categoryID *uint) *contentdomain.PostCategory {
	if categoryID == nil || *categoryID == 0 || h.categories == nil {
		return nil
	}
	category, err := h.categories.GetByID(ctx, *categoryID)
	if err != nil || category == nil {
		return nil
	}
	return category
}

// GetPublicBanners 获取公开 Banner 列表。
func (h *PublicHandler) GetPublicBanners(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	banners, err := h.banners.ListPublic(c.Request.Context(), contentapp.PublicBannerQuery{
		Position: c.DefaultQuery("position", constants.BannerPositionHomeHero),
		Limit:    limit,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.banner_fetch_failed", err)
		return
	}
	response.Success(c, contentpresenter.NewBannerRespList(banners))
}

// GetPostCategories 获取公开文章分类平铺列表。
func (h *PublicHandler) GetPostCategories(c *gin.Context) {
	categories, err := h.categories.ListActive(c.Request.Context())
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.post_category_fetch_failed", err)
		return
	}
	response.Success(c, contentpresenter.NewPostCategoryRespList(categories))
}
