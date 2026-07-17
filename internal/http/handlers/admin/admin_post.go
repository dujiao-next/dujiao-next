package admin

import (
	"errors"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/models"
	"github.com/dujiao-next/internal/service"

	"github.com/gin-gonic/gin"
)

// GetAdminPosts 获取文章列表 (Admin)
func (h *Handler) GetAdminPosts(c *gin.Context) {
	page, pageSize := shared.ParsePagination(c)
	postType := c.Query("type")
	search := c.Query("search")

	posts, total, err := h.PostService.ListAdmin(postType, search, page, pageSize)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, posts, pagination)
}

// ====================  文章管理  ====================

// CreatePostRequest 创建文章请求
type CreatePostRequest struct {
	Slug        string                 `json:"slug" binding:"required"`
	Type        string                 `json:"type" binding:"required"` // blog 或 notice
	TitleJSON   map[string]interface{} `json:"title" binding:"required"`
	SummaryJSON map[string]interface{} `json:"summary"`
	ContentJSON map[string]interface{} `json:"content"`
	Thumbnail   string                 `json:"thumbnail"`
	IsPublished *bool                  `json:"is_published"`
	ProductIDs  *[]uint                `json:"product_ids"` // nil 不改；非 nil 替换关联
	CategoryID  *uint                  `json:"category_id"`
}

// CreatePost 创建文章
func (h *Handler) CreatePost(c *gin.Context) {
	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	post, err := h.PostService.Create(service.CreatePostInput{
		Slug:        req.Slug,
		Type:        req.Type,
		TitleJSON:   req.TitleJSON,
		SummaryJSON: req.SummaryJSON,
		ContentJSON: req.ContentJSON,
		Thumbnail:   req.Thumbnail,
		IsPublished: req.IsPublished,
		ProductIDs:  req.ProductIDs,
		CategoryID:  req.CategoryID,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidPostType) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_type_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrPostCategoryInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_category_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrPostNoticeCategoryUnsupported) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_notice_category_unsupported", nil)
			return
		}
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_exists", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_create_failed", err)
		return
	}

	response.Success(c, post)
}

// UpdatePost 更新文章
func (h *Handler) UpdatePost(c *gin.Context) {
	id := c.Param("id")

	var req CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		shared.RespondBindError(c, err)
		return
	}

	post, err := h.PostService.Update(id, service.CreatePostInput{
		Slug:        req.Slug,
		Type:        req.Type,
		TitleJSON:   req.TitleJSON,
		SummaryJSON: req.SummaryJSON,
		ContentJSON: req.ContentJSON,
		Thumbnail:   req.Thumbnail,
		IsPublished: req.IsPublished,
		ProductIDs:  req.ProductIDs,
		CategoryID:  req.CategoryID,
	})
	if err != nil {
		if errors.Is(err, service.ErrInvalidPostType) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_type_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrPostCategoryInvalid) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_category_invalid", nil)
			return
		}
		if errors.Is(err, service.ErrPostNoticeCategoryUnsupported) {
			shared.RespondError(c, response.CodeBadRequest, "error.post_notice_category_unsupported", nil)
			return
		}
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.post_not_found", nil)
			return
		}
		if errors.Is(err, service.ErrSlugExists) {
			shared.RespondError(c, response.CodeBadRequest, "error.slug_used", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_update_failed", err)
		return
	}

	response.Success(c, post)
}

// AdminPostProductRef 后台编辑回填的关联商品精简结构
type AdminPostProductRef struct {
	ID    uint        `json:"id"`
	Slug  string      `json:"slug"`
	Title models.JSON `json:"title"`
	Image string      `json:"image,omitempty"`
}

// GetAdminPostProductIDs 获取文章关联商品列表（后台编辑回填）
func (h *Handler) GetAdminPostProductIDs(c *gin.Context) {
	id, err := shared.ParseParamUint(c, "id")
	if err != nil {
		shared.RespondError(c, response.CodeBadRequest, "error.invalid_id", nil)
		return
	}
	products, err := h.PostService.ListRelatedProducts(id)
	if err != nil {
		shared.RespondError(c, response.CodeInternal, "error.post_fetch_failed", err)
		return
	}
	refs := make([]AdminPostProductRef, 0, len(products))
	for i := range products {
		p := &products[i]
		ref := AdminPostProductRef{
			ID:    p.ID,
			Slug:  p.Slug,
			Title: p.TitleJSON,
		}
		if len(p.Images) > 0 {
			ref.Image = p.Images[0]
		}
		refs = append(refs, ref)
	}
	response.Success(c, refs)
}

// DeletePost 删除文章（软删除）
func (h *Handler) DeletePost(c *gin.Context) {
	id := c.Param("id")

	if err := h.PostService.Delete(id); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			shared.RespondError(c, response.CodeNotFound, "error.post_not_found", nil)
			return
		}
		shared.RespondError(c, response.CodeInternal, "error.post_delete_failed", err)
		return
	}

	response.Success(c, nil)
}
